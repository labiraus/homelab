package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// RecordDiff compares complete records. A nil side means the record is absent.
type RecordDiff struct {
	ID     string  `json:"id"`
	Before *Record `json:"before"`
	After  *Record `json:"after"`
}

func sameRecord(a, b *Record) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return string(left) == string(right)
}

func recordAt(s Snapshot, id string) *Record {
	if r, ok := s[id]; ok {
		return &r
	}
	return nil
}

func Diff(before, after Snapshot) []RecordDiff {
	ids := map[string]bool{}
	for id := range before {
		ids[id] = true
	}
	for id := range after {
		ids[id] = true
	}
	result := []RecordDiff{}
	for id := range ids {
		a, b := recordAt(before, id), recordAt(after, id)
		if !sameRecord(a, b) {
			result = append(result, RecordDiff{id, a, b})
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].ID < result[j].ID })
	return result
}

func diffOperations(diff []RecordDiff) []Operation {
	ops := []Operation{}
	for _, d := range diff {
		action := "put"
		if d.After == nil {
			action = "delete"
		}
		ops = append(ops, Operation{Action: action, ID: d.ID, Record: d.After})
	}
	return ops
}

type Revision struct {
	Revision int64     `json:"revision"`
	Actor    string    `json:"actor"`
	Reason   string    `json:"reason"`
	Created  time.Time `json:"createdAt"`
}

func (sc *scope) revisions(ctx context.Context, limit, offset int64) ([]Revision, error) {
	rows, err := sc.tx.Query(ctx, `SELECT revision,actor,reason,created_at FROM worldbuilder.revision WHERE world_id=$1 ORDER BY revision DESC LIMIT $2 OFFSET $3`, sc.world.ID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Revision{}
	for rows.Next() {
		var rev Revision
		if err := rows.Scan(&rev.Revision, &rev.Actor, &rev.Reason, &rev.Created); err != nil {
			return nil, err
		}
		result = append(result, rev)
	}
	return result, rows.Err()
}

type ReconcileRecord struct {
	ID       string  `json:"id"`
	Base     *Record `json:"base"`
	Current  *Record `json:"current"`
	Proposed *Record `json:"proposed"`
	Conflict bool    `json:"conflict"`
}

type Reconciliation struct {
	BaseRevision    int64             `json:"baseRevision"`
	CurrentRevision int64             `json:"currentRevision"`
	ExpectedVersion int64             `json:"expectedVersion"`
	Records         []ReconcileRecord `json:"records"`
	ValidationError string            `json:"validationError,omitempty"`
}

// Conflicts are deliberately record-level, including concurrent deletion. Even
// two independently valid edits must be consciously reconciled before review.
func reconciliation(base, current Snapshot, ops []Operation) (Reconciliation, error) {
	proposed, err := Apply(base, ops)
	if err != nil {
		return Reconciliation{}, err
	}
	r := Reconciliation{Records: []ReconcileRecord{}}
	for _, d := range Diff(base, proposed) {
		now := recordAt(current, d.ID)
		r.Records = append(r.Records, ReconcileRecord{d.ID, d.Before, now, d.After, !sameRecord(d.Before, now) && !sameRecord(d.After, now)})
	}
	// Also report cross-record validation failures (removed references, rules or
	// changed schemas) even when there is no same-record write conflict.
	effective := []RecordDiff{}
	for _, record := range r.Records {
		if !sameRecord(record.Current, record.Proposed) {
			effective = append(effective, RecordDiff{record.ID, record.Current, record.Proposed})
		}
	}
	if len(effective) > 0 {
		if _, err = Apply(current, diffOperations(effective)); err != nil {
			r.ValidationError = err.Error()
		}
	}
	return r, nil
}

func (sc *scope) reconcilePreview(ctx context.Context, kind, id string) (Reconciliation, error) {
	var baseRevision, version int64
	var ops []Operation
	if kind == "changes" {
		c, err := sc.change(ctx, id)
		if err != nil {
			return Reconciliation{}, err
		}
		var branch bool
		if err = sc.tx.QueryRow(ctx, `SELECT EXISTS(SELECT FROM worldbuilder.audit WHERE world_id=$1 AND resource=$2 AND action='timeline.branch')`, sc.world.ID, c.ID).Scan(&branch); err != nil {
			return Reconciliation{}, err
		}
		if branch {
			return Reconciliation{}, bad("prepare a new timeline branch proposal against current canon; branch copies cannot be partially rebased")
		}
		baseRevision, ops = c.BaseRevision, c.Operations
	} else {
		s, err := sc.session(ctx, id)
		if err != nil {
			return Reconciliation{}, err
		}
		baseRevision, version = s.PinnedRevision, s.Version
		ops = diffOperations(Diff(Snapshot{}, s.State))
		if len(ops) == 0 {
			return Reconciliation{BaseRevision: baseRevision, CurrentRevision: sc.world.Revision, ExpectedVersion: version, Records: []ReconcileRecord{}}, nil
		}
	}
	base, err := sc.snapshot(ctx, baseRevision)
	if err != nil {
		return Reconciliation{}, err
	}
	current, err := sc.snapshot(ctx, sc.world.Revision)
	if err != nil {
		return Reconciliation{}, err
	}
	r, err := reconciliation(base, current, ops)
	r.BaseRevision, r.CurrentRevision, r.ExpectedVersion = baseRevision, sc.world.Revision, version
	return r, err
}

type ReconcileRequest struct {
	ID              string            `json:"id"`
	BaseRevision    int64             `json:"baseRevision"`
	ExpectedVersion int64             `json:"expectedVersion"`
	Reason          string            `json:"reason"`
	Resolutions     map[string]string `json:"resolutions"`
}

// Reconciliation always creates a fresh draft. The immutable audit stores the
// exact request and lineage so retries remain safe after canon or story advances.
func (sc *scope) reconcile(ctx context.Context, kind, source string, request ReconcileRequest) (Change, error) {
	if !sc.can("author", "reader") {
		return Change{}, forbidden
	}
	// Check source visibility before inspecting a potential retry's audit record.
	if kind == "changes" {
		if change, err := sc.change(ctx, source); err != nil {
			return Change{}, err
		} else if !sc.can("author") && change.Author != sc.actor {
			return Change{}, forbidden
		}
	} else {
		if _, err := sc.session(ctx, source); err != nil {
			return Change{}, err
		}
	}
	detail := struct {
		Kind    string           `json:"kind"`
		Source  string           `json:"source"`
		Request ReconcileRequest `json:"request"`
	}{kind, source, request}
	if old, err := sc.replayedDraft(ctx, request.ID, "change.reconcile", detail); err != nil {
		return Change{}, err
	} else if old != nil {
		return *old, nil
	}
	if request.BaseRevision != sc.world.Revision {
		return Change{}, conflict("canon changed since preview; load reconciliation again")
	}
	preview, err := sc.reconcilePreview(ctx, kind, source)
	if err != nil {
		return Change{}, err
	}
	if kind == "sessions" && request.ExpectedVersion != preview.ExpectedVersion {
		return Change{}, conflict("story changed since preview; load reconciliation again")
	}
	ops := []Operation{}
	known := map[string]bool{}
	for _, record := range preview.Records {
		known[record.ID] = true
		choice := request.Resolutions[record.ID]
		if choice != "" && choice != "current" && choice != "proposed" {
			return Change{}, bad("resolution must be current or proposed")
		}
		if record.Conflict && choice == "" {
			return Change{}, conflict("explicit resolution required for " + record.ID)
		}
		if choice == "current" || sameRecord(record.Current, record.Proposed) {
			continue
		}
		action := "put"
		if record.Proposed == nil {
			action = "delete"
		}
		ops = append(ops, Operation{Action: action, ID: record.ID, Record: record.Proposed})
	}
	for id := range request.Resolutions {
		if !known[id] {
			return Change{}, bad("resolution references an unchanged record")
		}
	}
	if len(ops) == 0 {
		return Change{}, bad("no changes remain after reconciliation")
	}
	c, err := sc.propose(ctx, Change{ID: request.ID, BaseRevision: request.BaseRevision, Reason: request.Reason, Operations: ops})
	if err != nil {
		return Change{}, err
	}
	return c, sc.audit(ctx, "change.reconcile", c.ID, detail)
}

type RestoreRequest struct {
	ID             string `json:"id"`
	BaseRevision   int64  `json:"baseRevision"`
	TargetRevision int64  `json:"targetRevision"`
	Reason         string `json:"reason"`
}

func (sc *scope) restore(ctx context.Context, r RestoreRequest) (Change, error) {
	if !sc.can("owner", "admin", "editor") {
		return Change{}, forbidden
	}
	if old, err := sc.replayedDraft(ctx, r.ID, "change.restore", r); err != nil {
		return Change{}, err
	} else if old != nil {
		return *old, nil
	}
	before, err := sc.snapshot(ctx, r.BaseRevision)
	if err != nil {
		return Change{}, err
	}
	after, err := sc.snapshot(ctx, r.TargetRevision)
	if err != nil {
		return Change{}, err
	}
	if r.TargetRevision >= r.BaseRevision {
		return Change{}, bad("restore target must precede base revision")
	}
	// propose checks current revision or returns the original idempotent result.
	// The normal approval/publish path creates compensation, never edits history.
	c, err := sc.propose(ctx, Change{ID: r.ID, BaseRevision: r.BaseRevision, Reason: r.Reason, Operations: diffOperations(Diff(before, after))})
	if err != nil {
		return Change{}, err
	}
	return c, sc.audit(ctx, "change.restore", c.ID, r)
}

func (sc *scope) replayedDraft(ctx context.Context, id, action string, detail any) (*Change, error) {
	old, err := sc.change(ctx, id)
	if err == missing {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, err
	}
	var same bool
	err = sc.tx.QueryRow(ctx, `SELECT detail=$4::jsonb FROM worldbuilder.audit WHERE world_id=$1 AND resource=$2 AND action=$3`, sc.world.ID, id, action, raw).Scan(&same)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && (!same || old.Author != sc.actor)) {
		return nil, conflict("request id reused with different content")
	}
	return &old, err
}
