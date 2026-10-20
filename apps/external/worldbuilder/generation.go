package worldbuilder

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"pkg/api"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type GenerationRequest struct {
	ReasonInput
	ID    string `json:"id"`
	Parts int    `json:"parts"`
}
type Generation struct {
	InferenceProfile InferenceProfile  `json:"inferenceProfile"`
	ID               string            `json:"id"`
	Request          GenerationRequest `json:"request"`
	Status           string            `json:"status"`
	Step             int               `json:"step"`
	Parts            int               `json:"parts"`
	Results          []ReasonResult    `json:"results"`
	ErrorCode        string            `json:"errorCode"`
	CreatedAt        time.Time         `json:"createdAt"`
	LeaseUntil       *time.Time        `json:"leaseUntil,omitempty"`
	LeaseToken       string            `json:"-"`
	ActiveLease      bool              `json:"-"`
}
type GenerationRef struct {
	WorldID string `json:"worldId"`
	JobID   string `json:"jobId"`
}
type GenerationRunState struct {
	Done   bool   `json:"done"`
	Status string `json:"status"`
	Step   int    `json:"step"`
	Parts  int    `json:"parts"`
}

func generationRunState(g Generation) GenerationRunState {
	return GenerationRunState{Done: g.Status != "queued" && g.Status != "running", Status: g.Status, Step: g.Step, Parts: g.Parts}
}

func (sc *scope) generation(ctx context.Context, id string) (Generation, error) {
	var g Generation
	var request, results []byte
	if _, err := uuid.Parse(id); err != nil {
		return g, missing
	}
	err := sc.tx.QueryRow(ctx, `SELECT id::text,request,status,step,parts,results,error_code,created_at,COALESCE(lease_token::text,''),lease_until,COALESCE(lease_until>now(),false),inference_profile FROM worldbuilder.generation_job WHERE world_id=$1 AND id=$2 AND owner=$3`, sc.world.ID, id, sc.actor).Scan(&g.ID, &request, &g.Status, &g.Step, &g.Parts, &results, &g.ErrorCode, &g.CreatedAt, &g.LeaseToken, &g.LeaseUntil, &g.ActiveLease, &g.InferenceProfile)
	if errors.Is(err, pgx.ErrNoRows) {
		return g, missing
	}
	if err != nil {
		return g, err
	}
	if err = json.Unmarshal(request, &g.Request); err != nil {
		return g, err
	}
	err = json.Unmarshal(results, &g.Results)
	return g, err
}
func (sc *scope) createGeneration(ctx context.Context, b GenerationRequest) (Generation, error) {
	if !sc.jobsEnabled {
		return Generation{}, &Problem{503, "unavailable", "durable generation is not enabled"}
	}
	if b.Parts == 0 {
		b.Parts = 1
	}
	if _, err := uuid.Parse(b.ID); err != nil || b.ChangeID != "" || strings.TrimSpace(b.Question) == "" || len(b.Question) > 4000 || b.Parts < 1 || b.Parts > 4 || (b.Workflow != "story_turn" && b.Workflow != "narrative_generate") || (b.Workflow == "story_turn" && b.Parts != 1) {
		return Generation{}, bad("generation requires UUID id, creative workflow, question and 1–4 narrative parts (one story turn)")
	}
	raw, _ := json.Marshal(b)
	if old, err := sc.generation(ctx, b.ID); err == nil {
		previous, _ := json.Marshal(old.Request)
		if string(previous) != string(raw) {
			return old, conflict("generation id reused with different request")
		}
		return old, nil
	} else if err != missing {
		return Generation{}, err
	}
	if b.Workflow == "narrative_generate" && (b.SessionID != "" || b.ExpectedVersion != nil) {
		return Generation{}, bad("standalone narratives do not bind a private session")
	}
	if _, err := sc.tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('worldbuilder-generation:' || $1,0))`, sc.actor); err != nil {
		return Generation{}, err
	}
	var pending int
	if err := sc.tx.QueryRow(ctx, `SELECT count(*) FROM worldbuilder.generation_job WHERE owner=$1 AND status IN ('queued','running')`, sc.actor).Scan(&pending); err != nil {
		return Generation{}, err
	}
	if pending >= 20 {
		return Generation{}, &Problem{429, "generation_queue", "at most 20 outstanding generations per actor"}
	}
	snapshot, err := sc.snapshot(ctx, b.Revision)
	if err != nil {
		return Generation{}, err
	}
	if snapshot[b.Timeline].Kind != "timeline" {
		return Generation{}, bad("unknown generation timeline")
	}
	if b.Workflow == "story_turn" {
		session, e := sc.session(ctx, b.SessionID)
		if e != nil {
			return Generation{}, e
		}
		if b.ExpectedVersion == nil || *b.ExpectedVersion != session.Version || b.Revision != session.PinnedRevision || b.Timeline != session.Timeline {
			return Generation{}, conflict("generation must match the inspected session version and pin")
		}
	}
	tag, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.generation_job(world_id,id,owner,request,parts,inference_profile) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, sc.world.ID, b.ID, sc.actor, raw, b.Parts, sc.budget.pin())
	if err != nil {
		return Generation{}, err
	}
	if tag.RowsAffected() == 0 {
		return Generation{}, conflict("generation id already used")
	}
	eventID := uuid.NewString()
	event, _ := json.Marshal(map[string]any{"eventId": eventID, "worldId": sc.world.ID, "jobId": b.ID, "schemaVersion": 1})
	if _, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.generation_outbox(id,world_id,job_id,payload) VALUES($1,$2,$3,$4)`, eventID, sc.world.ID, b.ID, event); err != nil {
		return Generation{}, err
	}
	if err = sc.audit(ctx, "generation.queue", b.ID, map[string]any{"workflow": b.Workflow, "parts": b.Parts, "revision": b.Revision, "sessionId": b.SessionID}); err != nil {
		return Generation{}, err
	}
	return sc.generation(ctx, b.ID)
}
func (sc *scope) generations(ctx context.Context, limit, offset int64) (any, error) {
	rows, err := sc.tx.Query(ctx, `SELECT id::text,status,step,parts,error_code,created_at,request->>'workflow',left(request->>'question',200) FROM worldbuilder.generation_job WHERE world_id=$1 AND owner=$2 ORDER BY created_at DESC,id LIMIT $3 OFFSET $4`, sc.world.ID, sc.actor, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []any{}
	for rows.Next() {
		var id, status, code, workflow, question string
		var step, parts int
		var created time.Time
		if err = rows.Scan(&id, &status, &step, &parts, &code, &created, &workflow, &question); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "status": status, "step": step, "parts": parts, "errorCode": code, "createdAt": created, "workflow": workflow, "question": question})
	}
	return map[string]any{"enabled": sc.jobsEnabled, "items": items}, rows.Err()
}
func (sc *scope) cancelGeneration(ctx context.Context, id string) (Generation, error) {
	g, err := sc.generation(ctx, id)
	if err != nil {
		return g, err
	}
	if g.Status == "queued" || g.Status == "running" {
		if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='cancelled',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE world_id=$1 AND id=$2`, sc.world.ID, id); err != nil {
			return g, err
		}
		if err = sc.audit(ctx, "generation.cancel", id, map[string]any{"completedParts": g.Step}); err != nil {
			return g, err
		}
	}
	return sc.generation(ctx, id)
}
func (s *Store) failGeneration(ref GenerationRef, token, status, code string) (GenerationRunState, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.Pool.Exec(ctx, `UPDATE worldbuilder.generation_job SET status=$3,error_code=$4,lease_until=NULL,lease_token=NULL,updated_at=now() WHERE world_id=$1 AND id=$2 AND status IN ('queued','running') AND ($5='' OR lease_token::text=$5)`, ref.WorldID, ref.JobID, status, code, token)
	if err != nil {
		return GenerationRunState{}, err
	}
	// A checkpoint commit may have succeeded despite losing its acknowledgement.
	// Read the actual state before telling JetStream this request is terminal.
	var g Generation
	err = s.Pool.QueryRow(ctx, `SELECT status,step,parts FROM worldbuilder.generation_job WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID).Scan(&g.Status, &g.Step, &g.Parts)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationRunState{Done: true, Status: "missing"}, nil
	}
	return generationRunState(g), err
}

// RunGeneration executes at most one checkpoint. Only a previously authenticated
// human request can supply its owner/input; the worker supplies IDs, never intent.
func (s *Store) RunGeneration(ctx context.Context, ref GenerationRef, validator api.AuthValidator) (GenerationRunState, error) {
	if !s.JobsEnabled {
		return GenerationRunState{}, &Problem{503, "unavailable", "durable generation is disabled"}
	}
	var owner string
	err := s.Pool.QueryRow(ctx, `SELECT owner FROM worldbuilder.generation_job WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID).Scan(&owner)
	if errors.Is(err, pgx.ErrNoRows) {
		return GenerationRunState{Done: true, Status: "missing"}, nil
	}
	if err != nil {
		return GenerationRunState{}, err
	}
	valid := false
	if validator != nil {
		valid, _, err = validator(ctx, owner)
	}
	if err != nil {
		return GenerationRunState{}, err
	}
	if !valid {
		return s.failGeneration(ref, "", "failed", "identity_revoked")
	}
	type claim struct {
		Job     Generation
		Claimed bool
	}
	value, err := s.inWorld(ctx, ref.WorldID, owner, func(sc *scope) (any, error) {
		g, e := sc.generation(ctx, ref.JobID)
		if e != nil {
			return nil, e
		}
		if generationRunState(g).Done {
			return claim{Job: g}, nil
		}
		if sc.world.Archived {
			_, e = sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='failed',error_code='world_archived',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID)
			g.Status = "failed"
			return claim{Job: g}, e
		}
		if g.Status == "running" {
			if !g.ActiveLease {
				unstarted := false
				if s.Budget != nil {
					e = sc.tx.QueryRow(ctx, `SELECT NOT EXISTS(SELECT FROM worldbuilder.inference_attempt WHERE world_id=$1 AND generation_job_id=$2 AND generation_part=$3 AND actor=$4)`, ref.WorldID, ref.JobID, g.Step+1, owner).Scan(&unstarted)
					if e != nil {
						return nil, e
					}
				}
				if unstarted {
					_, e = sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='queued',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID)
					g.Status = "queued"
					return claim{Job: g}, e
				}
				_, e = sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='uncertain',error_code='lease_expired',lease_token=NULL,lease_until=NULL,updated_at=now() WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID)
				g.Status = "uncertain"
			}
			return claim{Job: g}, e
		}
		g.LeaseToken = uuid.NewString()
		g.Status = "running"
		_, e = sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET status='running',lease_token=$3,lease_until=now()+interval '90 seconds',updated_at=now() WHERE world_id=$1 AND id=$2`, ref.WorldID, ref.JobID, g.LeaseToken)
		return claim{Job: g, Claimed: true}, e
	})
	if err != nil {
		if errors.Is(err, missing) {
			return s.failGeneration(ref, "", "failed", "membership_revoked")
		}
		return GenerationRunState{}, err
	}
	claimed := value.(claim)
	if !claimed.Claimed {
		return generationRunState(claimed.Job), nil
	}
	value, err = s.inWorld(ctx, ref.WorldID, owner, func(sc *scope) (any, error) {
		g, e := sc.generation(ctx, ref.JobID)
		if e != nil {
			return nil, e
		}
		if g.Status != "running" || g.LeaseToken != claimed.Job.LeaseToken {
			return generationRunState(g), nil
		}
		if !g.ActiveLease || sc.world.Archived {
			return nil, conflict("generation lease or world state changed")
		}
		gc := &GenerationContext{ID: g.ID, Part: g.Step + 1, Parts: g.Parts, Previous: []string{}}
		for _, part := range g.Results {
			gc.Previous = append(gc.Previous, part.Answer)
		}
		g.Request.ReasonInput.InferenceProfile = &g.InferenceProfile
		result, e := sc.generate(ctx, g.Request.ReasonInput, s.Reasoner, s.Budget, gc)
		if e != nil {
			return nil, e
		}
		g.Results = append(g.Results, result.(ReasonResult))
		g.Step++
		g.Status = "queued"
		if g.Step == g.Parts {
			g.Status = "succeeded"
		}
		raw, _ := json.Marshal(g.Results)
		tag, e := sc.tx.Exec(ctx, `UPDATE worldbuilder.generation_job SET results=$3,step=$4,status=$5,lease_token=NULL,lease_until=NULL,updated_at=now() WHERE world_id=$1 AND id=$2 AND status='running' AND lease_token=$6`, ref.WorldID, ref.JobID, raw, g.Step, g.Status, g.LeaseToken)
		if e != nil {
			return nil, e
		}
		if tag.RowsAffected() != 1 {
			return nil, conflict("generation claim changed")
		}
		if e = sc.audit(ctx, "generation.part", g.ID, map[string]any{"part": g.Step, "status": g.Status, "requestId": g.Results[len(g.Results)-1].Provenance.RequestID}); e != nil {
			return nil, e
		}
		return generationRunState(g), nil
	})
	if err != nil {
		code := "generation_failed"
		var problem *Problem
		if errors.As(err, &problem) {
			code = problem.Code
		}
		return s.failGeneration(ref, claimed.Job.LeaseToken, "failed", code)
	}
	return value.(GenerationRunState), nil
}

const GenerationRunPath = "/api/worldbuilder-jobs/run"

func GenerationHandler(store *Store, token string, validator api.AuthValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		supplied := r.Header.Get("Authorization")
		expected := "Bearer " + token
		if len(token) < 32 || len(supplied) != len(expected) || subtle.ConstantTimeCompare([]byte(supplied), []byte(expected)) != 1 {
			writeError(w, &Problem{401, "unauthenticated", "valid generation worker identity required"})
			return
		}
		if r.Method != "POST" || r.URL.Path != GenerationRunPath || r.URL.RawQuery != "" {
			writeError(w, missing)
			return
		}
		var ref GenerationRef
		if err := decode(w, r, &ref); err != nil {
			writeError(w, err)
			return
		}
		if _, err := uuid.Parse(ref.WorldID); err != nil {
			writeError(w, bad("invalid world id"))
			return
		}
		if _, err := uuid.Parse(ref.JobID); err != nil {
			writeError(w, bad("invalid job id"))
			return
		}
		result, err := store.RunGeneration(r.Context(), ref, validator)
		if err != nil {
			writeError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
