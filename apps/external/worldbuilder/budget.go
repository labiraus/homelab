package worldbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type InferenceBudget struct {
	Mode            string
	Profile         string
	ProfileRevision string
	// A separate small pool prevents waiting for a connection held by another
	// world transaction. The ledger never locks or foreign-keys the world row.
	Pool          *pgxpool.Pool
	DailyRequests int64
	Concurrent    int64
	RequestMicros int64
	DailyMicros   int64
	Currency      string
	GatewayRoute  string
}
type Provenance struct {
	Profile          string `json:"profile,omitempty"`
	ProfileRevision  string `json:"profileRevision,omitempty"`
	ProxyRequestID   string `json:"proxyRequestId,omitempty"`
	AccountingStatus string `json:"accountingStatus,omitempty"`
	EstimatedMicros  int64  `json:"estimatedMicros,omitempty"`
	ReservedMicros   int64  `json:"reservedMicros,omitempty"`
	GenerationID     string `json:"generationId,omitempty"`
	GenerationPart   int    `json:"generationPart,omitempty"`
	RequestID        string `json:"requestId"`
	Workflow         string `json:"workflow"`
	GatewayRoute     string `json:"gatewayRoute"`
	Provider         string `json:"provider,omitempty"`
	LatencyMS        int64  `json:"latencyMs"`
	AllocatedMicros  int64  `json:"allocatedMicros"`
	Currency         string `json:"currency"`
}

func (b *InferenceBudget) Validate() error {
	if b.Mode == "proxy" {
		if b.Concurrent < 1 || b.Concurrent > 2 || b.Currency != "USD" || b.RequestMicros != 0 || b.DailyMicros != 0 || b.DailyRequests != 0 || b.GatewayRoute == "" || b.ProfileRevision == "" || (b.Profile != "local-default" && b.Profile != "bedrock-nova" && b.Profile != "bedrock-sonnet") {
			return fmt.Errorf("proxy accounting requires a pinned profile, USD, concurrency 1–2 and zero app quotas")
		}
		return nil
	}
	if b.Mode != "" && b.Mode != "legacy" {
		return fmt.Errorf("invalid inference accounting mode")
	}
	if b.DailyRequests < 1 || b.DailyRequests > 100000 || b.Concurrent < 1 || b.Concurrent > 100 || b.RequestMicros < 1 || b.DailyMicros < b.RequestMicros || b.DailyMicros > 1_000_000_000_000 || len(b.Currency) != 3 || len(b.GatewayRoute) == 0 || len(b.GatewayRoute) > 100 {
		return fmt.Errorf("inference requires bounded request/concurrency limits and an explicit per-request/daily currency allowance")
	}
	for _, r := range b.Currency {
		if r < 'A' || r > 'Z' {
			return fmt.Errorf("inference currency must be an uppercase three-letter code")
		}
	}
	return nil
}

func (b *InferenceBudget) reserve(ctx context.Context, actor, session string, request ReasonRequest) (Provenance, error) {
	p := Provenance{RequestID: uuid.NewString(), Workflow: request.Workflow, GatewayRoute: b.GatewayRoute, AllocatedMicros: b.RequestMicros, Currency: b.Currency}
	if err := b.Validate(); err != nil {
		return p, err
	}
	mode := "legacy"
	profile := b.pin()
	if request.InferenceProfile != nil {
		profile = *request.InferenceProfile
	}
	if profile.Mode != b.pin().Mode {
		return p, &Problem{409, "inference_profile_changed", "queued inference belongs to a different accounting mode; create a new request"}
	}
	if b.Mode == "proxy" {
		mode = "proxy"
		p.Profile, p.ProfileRevision = profile.Alias, profile.Revision
		p.AccountingStatus = "proxy_pending"
	}
	tx, err := b.Pool.Begin(ctx)
	if err != nil {
		return p, err
	}
	defer tx.Rollback(ctx)
	// Global per-actor serialisation spans replicas and worlds; hash collisions
	// can only over-serialise, never allow overspending.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('worldbuilder-ai:' || $1,0))`, actor); err != nil {
		return p, err
	}
	var daily, allocated, active int64
	var currencies bool
	err = tx.QueryRow(ctx, `SELECT count(*) FILTER(WHERE accounting_mode='legacy' AND created_at >= date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),
 COALESCE(sum(allocated_micros) FILTER(WHERE accounting_mode='legacy' AND created_at >= date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),0),
 count(*) FILTER(WHERE status='reserved' AND lease_until>now()),
 COALESCE(bool_and(currency=$2) FILTER(WHERE accounting_mode='legacy' AND created_at >= date_trunc('day',now() AT TIME ZONE 'UTC') AT TIME ZONE 'UTC'),true)
 FROM worldbuilder.inference_attempt WHERE actor=$1`, actor, b.Currency).Scan(&daily, &allocated, &active, &currencies)
	if err != nil {
		return p, err
	}
	if active >= b.Concurrent || (mode == "legacy" && (!currencies || daily >= b.DailyRequests || allocated > b.DailyMicros-b.RequestMicros)) {
		return p, &Problem{429, "inference_budget", "daily allowance or concurrent inference limit reached"}
	}
	raw, _ := json.Marshal(request)
	digest := sha256.Sum256(raw)
	var generationID *string
	var part *int
	if request.Generation != nil {
		generationID = &request.Generation.ID
		part = &request.Generation.Part
		p.GenerationID = *generationID
		p.GenerationPart = *part
	}
	_, err = tx.Exec(ctx, `INSERT INTO worldbuilder.inference_attempt(id,world_id,actor,workflow,revision,timeline,session_id,request_hash,lease_until,allocated_micros,currency,gateway_route,generation_job_id,generation_part,accounting_mode,inference_profile) VALUES($1,$2,$3,$4,$5,$6,$7,$8,now()+interval '90 seconds',$9,$10,$11,$12,$13,$14,$15)`, p.RequestID, request.WorldID, actor, request.Workflow, request.Revision, request.Timeline, session, hex.EncodeToString(digest[:]), b.RequestMicros, b.Currency, b.GatewayRoute, generationID, part, mode, profile)
	if err != nil {
		return p, err
	}
	return p, tx.Commit(ctx)
}

type InferenceProfile struct {
	Mode     string `json:"mode"`
	Alias    string `json:"alias,omitempty"`
	Revision string `json:"revision,omitempty"`
}

func (b *InferenceBudget) pin() InferenceProfile {
	if b == nil || b.Mode != "proxy" {
		return InferenceProfile{Mode: "legacy"}
	}
	return InferenceProfile{Mode: "proxy", Alias: b.Profile, Revision: b.ProfileRevision}
}

func (b *InferenceBudget) finish(p Provenance, result ReasonResult, succeeded bool) error {
	// Completion must survive client cancellation and canon transaction rollback.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	status := "failed"
	if succeeded {
		status = "succeeded"
	}
	usageMap := result.Usage
	if usageMap == nil {
		usageMap = map[string]any{}
	}
	if result.Inference != nil {
		usageMap["inference"] = result.Inference
	}
	usage, _ := json.Marshal(usageMap)
	_, err := b.Pool.Exec(ctx, `UPDATE worldbuilder.inference_attempt SET status=$2,model=$3,usage=$4,latency_ms=$5,finished_at=now() WHERE id=$1 AND status='reserved'`, p.RequestID, status, result.Model, usage, p.LatencyMS)
	return err
}
