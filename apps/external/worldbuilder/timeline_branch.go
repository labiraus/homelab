package worldbuilder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Branches materialize timeline-scoped records through ordinary reviewed puts.
// World-wide catalog/settings remain shared. Audit, not editable attributes,
// records the parent snapshot and the creation proposal.
type TimelineBranchRequest struct {
	ID             string `json:"id"`
	BaseRevision   *int64 `json:"baseRevision"`
	ParentRevision *int64 `json:"parentRevision"`
	ParentTimeline string `json:"parentTimeline"`
	Timeline       string `json:"timeline"`
	Label          string `json:"label"`
	Reason         string `json:"reason"`
}

func branchRecordID(timeline, source string) string {
	digest := sha256.Sum256([]byte(timeline + "\x00" + source))
	return "b_" + hex.EncodeToString(digest[:])[:62]
}

func branchOperations(parent, current Snapshot, request TimelineBranchRequest) ([]Operation, error) {
	if !identifier.MatchString(request.Timeline) || !identifier.MatchString(request.ParentTimeline) || strings.TrimSpace(request.Label) == "" || len(request.Label) > 500 {
		return nil, bad("branch requires valid parent/new timeline IDs and a label of at most 500 characters")
	}
	if parent[request.ParentTimeline].Kind != "timeline" {
		return nil, bad("parent timeline is absent at the selected revision")
	}
	if _, exists := current[request.Timeline]; exists {
		return nil, conflict("branch timeline ID already exists")
	}
	if request.Timeline == request.ParentTimeline {
		return nil, bad("branch requires a distinct timeline ID")
	}
	ids := []string{}
	for id, r := range parent {
		if r.Timeline == request.ParentTimeline && r.Kind == "timeline" {
			return nil, bad("scoped timeline descriptors require explicit resolution before branching")
		}
		if r.Timeline == request.ParentTimeline && r.Kind != "world_settings" && r.Kind != "timeline" {
			ids = append(ids, id)
		}
	}
	// One timeline plus the complete copy must fit the ordinary governance limit.
	// Never truncate, partition publication, or silently omit rules/history.
	if len(ids) > 99 {
		return nil, bad("branch exceeds the 99 timeline-record limit; no partial branch was created")
	}
	sort.Strings(ids)
	remap := map[string]string{request.ParentTimeline: request.Timeline}
	for _, id := range ids {
		remap[id] = branchRecordID(request.Timeline, id)
		if _, exists := current[remap[id]]; exists || remap[id] == request.Timeline {
			return nil, conflict("branch record ID already exists")
		}
	}
	reference := func(id string) string {
		if mapped, ok := remap[id]; ok {
			return mapped
		}
		return id
	}
	branch := Record{ID: request.Timeline, Kind: "timeline", Label: request.Label}
	ops := []Operation{{Action: "put", ID: branch.ID, Record: &branch}}
	for _, id := range ids {
		r := parent[id]
		r.ID, r.Timeline = remap[id], request.Timeline
		r.Type, r.Subject, r.Target, r.Source = reference(r.Type), reference(r.Subject), reference(r.Target), reference(r.Source)
		if r.ValueType == "entity" {
			if value, ok := r.Value.(string); ok {
				r.Value = reference(value)
			}
		}
		if r.Relations != nil {
			relations := make(map[string]string, len(r.Relations))
			for predicate, target := range r.Relations {
				relations[predicate] = reference(target)
			}
			r.Relations = relations
		}
		ops = append(ops, Operation{Action: "put", ID: r.ID, Record: &r})
	}
	after, err := Apply(current, ops)
	if err != nil {
		return nil, err
	}
	// Shared references use current canon. A historical or cross-timeline
	// dependency must still be visible and valid; there is no hidden fallback.
	seen := map[string]bool{}
	pending := make([]string, 0, len(ops))
	for _, op := range ops {
		pending = append(pending, op.ID)
	}
	// World-wide rules remain authoritative too. Refuse a copy that would
	// detach them from a scoped type/source instead of silently losing policy.
	for id, record := range after {
		if record.Kind == "rule" && record.Timeline == "" {
			pending = append(pending, id)
		}
	}
	for len(pending) > 0 {
		id := pending[0]
		pending = pending[1:]
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		r, ok := after[id]
		if !ok || !Visible(r, request.Timeline, nil) {
			return nil, bad("branch references must be visible in the new timeline")
		}
		pending = append(pending, recordReferences(r)...)
	}
	return ops, nil
}

func (sc *scope) branchTimeline(ctx context.Context, request TimelineBranchRequest) (Change, error) {
	if !sc.can("owner", "admin", "editor") {
		return Change{}, forbidden
	}
	proposalID, err := uuid.Parse(request.ID)
	if err != nil {
		return Change{}, bad("branch requires a UUID proposal id")
	}
	request.ID = proposalID.String()
	if request.BaseRevision == nil || request.ParentRevision == nil || *request.BaseRevision < 0 || *request.ParentRevision < 0 {
		return Change{}, bad("explicit nonnegative baseRevision and parentRevision are required")
	}
	if old, err := sc.replayedDraft(ctx, request.ID, "timeline.branch", request); err != nil {
		return Change{}, err
	} else if old != nil {
		return *old, nil
	}
	if *request.BaseRevision != sc.world.Revision {
		return Change{}, conflict("canon changed; inspect the branch against the current revision")
	}
	parent, err := sc.snapshot(ctx, *request.ParentRevision)
	if err != nil {
		return Change{}, err
	}
	current, err := sc.snapshot(ctx, *request.BaseRevision)
	if err != nil {
		return Change{}, err
	}
	ops, err := branchOperations(parent, current, request)
	if err != nil {
		return Change{}, err
	}
	ids := make([]string, 0, len(ops))
	for _, op := range ops {
		ids = append(ids, op.ID)
	}
	var used bool
	if err = sc.tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM worldbuilder.record_identity WHERE world_id=$1 AND id=ANY($2))`, sc.world.ID, ids).Scan(&used); err != nil {
		return Change{}, err
	}
	if used {
		return Change{}, conflict("branch requires unused identities, including previously deleted records")
	}
	c, err := sc.propose(ctx, Change{ID: request.ID, BaseRevision: *request.BaseRevision, Reason: request.Reason, Operations: ops})
	if err != nil {
		return Change{}, err
	}
	return c, sc.audit(ctx, "timeline.branch", c.ID, request)
}

type TimelineBranchOrigin struct {
	Request           TimelineBranchRequest `json:"request"`
	PublishedRevision int64                 `json:"publishedRevision"`
	Records           map[string]string     `json:"records"` // source ID -> copied ID
}

func (sc *scope) branchOrigin(ctx context.Context, timeline string, revision int64) (*TimelineBranchOrigin, error) {
	snapshot, err := sc.snapshot(ctx, revision)
	if err != nil {
		return nil, err
	}
	if snapshot[timeline].Kind != "timeline" {
		return nil, missing
	}
	var result TimelineBranchOrigin
	var raw, operations []byte
	err = sc.tx.QueryRow(ctx, `SELECT a.detail,c.published_revision,c.operations FROM worldbuilder.audit a JOIN worldbuilder.change_set c ON c.world_id=a.world_id AND c.id::text=a.resource WHERE a.world_id=$1 AND a.action='timeline.branch' AND a.detail->>'timeline'=$2 AND c.status='published' AND c.published_revision<=$3 ORDER BY c.published_revision LIMIT 1`, sc.world.ID, timeline, revision).Scan(&raw, &result.PublishedRevision, &operations)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil // An ordinary isolated timeline has no branch origin.
	}
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &result.Request); err != nil {
		return nil, err
	}
	if result.Request.ParentRevision == nil {
		return nil, errors.New("branch audit lacks parent revision")
	}
	parent, err := sc.snapshot(ctx, *result.Request.ParentRevision)
	if err != nil {
		return nil, err
	}
	var ops []Operation
	if err = json.Unmarshal(operations, &ops); err != nil {
		return nil, err
	}
	created := map[string]bool{}
	for _, op := range ops {
		created[op.ID] = true
	}
	result.Records = map[string]string{}
	for id, r := range parent {
		if r.Timeline == result.Request.ParentTimeline && r.Kind != "world_settings" && r.Kind != "timeline" {
			copied := branchRecordID(timeline, id)
			if created[copied] {
				result.Records[id] = copied
			}
		}
	}
	return &result, nil
}
