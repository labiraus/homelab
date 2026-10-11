package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool        *pgxpool.Pool
	Reasoner    Reasoner
	Discover    Discover
	Budget      *InferenceBudget
	JobsEnabled bool
}
type scope struct {
	tx          pgx.Tx
	world       World
	actor       string
	role        string
	discover    Discover
	jobsEnabled bool
}

func (s *Store) inWorld(ctx context.Context, world, actor string, fn func(*scope) (any, error)) (any, error) {
	if _, err := uuid.Parse(world); err != nil {
		return nil, missing
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	sc := &scope{tx: tx, actor: actor, discover: s.Discover, jobsEnabled: s.JobsEnabled}
	// Serialise membership, publication and session changes within each world.
	err = tx.QueryRow(ctx, `SELECT id::text,name,revision,default_timeline,archived,admin_version FROM worldbuilder.world WHERE id=$1 FOR UPDATE`, world).Scan(&sc.world.ID, &sc.world.Name, &sc.world.Revision, &sc.world.DefaultTimeline, &sc.world.Archived, &sc.world.AdminVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	err = tx.QueryRow(ctx, `SELECT role FROM worldbuilder.member WHERE world_id=$1 AND principal=$2`, world, actor).Scan(&sc.role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	sc.world.Role = sc.role
	result, err := fn(sc)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}
func (s *Store) Worlds(ctx context.Context, actor string, limit, offset int64) ([]World, error) {
	rows, err := s.Pool.Query(ctx, `SELECT w.id::text,w.name,w.revision,m.role,w.default_timeline,w.archived,w.admin_version FROM worldbuilder.world w JOIN worldbuilder.member m ON m.world_id=w.id WHERE m.principal=$1 ORDER BY w.id LIMIT $2 OFFSET $3`, actor, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []World{}
	for rows.Next() {
		var w World
		if err := rows.Scan(&w.ID, &w.Name, &w.Revision, &w.Role, &w.DefaultTimeline, &w.Archived, &w.AdminVersion); err != nil {
			return nil, err
		}
		result = append(result, w)
	}
	return result, rows.Err()
}
func (s *Store) CreateWorld(ctx context.Context, actor, id, name string) (World, error) {
	if _, err := uuid.Parse(id); err != nil || strings.TrimSpace(name) == "" || len(name) > 200 {
		return World{}, bad("world requires UUID id and name of 1–200 characters")
	}
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return World{}, err
	}
	defer tx.Rollback(ctx)
	// Client-supplied stable ID makes create retries safe without upserting another owner's world.
	tag, err := tx.Exec(ctx, `INSERT INTO worldbuilder.world(id,name) VALUES($1,$2) ON CONFLICT DO NOTHING`, id, name)
	if err != nil {
		return World{}, err
	}
	if tag.RowsAffected() == 0 {
		var w World
		err = tx.QueryRow(ctx, `SELECT w.id::text,w.name,w.revision,m.role,w.default_timeline,w.archived,w.admin_version FROM worldbuilder.world w JOIN worldbuilder.member m ON m.world_id=w.id WHERE w.id=$1 AND m.principal=$2 AND m.role='owner' AND EXISTS(SELECT FROM worldbuilder.audit a WHERE a.world_id=w.id AND a.action='world.create' AND a.actor=$2 AND a.detail->>'name'=$3)`, id, actor, name).Scan(&w.ID, &w.Name, &w.Revision, &w.Role, &w.DefaultTimeline, &w.Archived, &w.AdminVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return World{}, conflict("world id already used")
		}
		return w, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO worldbuilder.member VALUES($1,$2,'owner')`, id, actor); err != nil {
		return World{}, err
	}
	snapshot := Snapshot{"main": {ID: "main", Kind: "timeline", Label: "Main continuity"}, "world_settings": {ID: "world_settings", Kind: "world_settings", Label: name, Timeline: "main"}}
	raw, _ := json.Marshal(snapshot)
	if _, err = tx.Exec(ctx, `INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES($1,0,$2,$3,'World created')`, id, raw, actor); err != nil {
		return World{}, err
	}
	sc := &scope{tx: tx, actor: actor, world: World{ID: id}}
	if err = sc.project(ctx, snapshot); err != nil {
		return World{}, err
	}
	if err = sc.audit(ctx, "world.create", id, map[string]string{"name": name}); err != nil {
		return World{}, err
	}
	return World{ID: id, Name: name, Role: "owner", DefaultTimeline: "main"}, tx.Commit(ctx)
}
func (sc *scope) can(roles ...string) bool {
	for _, r := range roles {
		if sc.role == r {
			return true
		}
	}
	return false
}
func (sc *scope) audit(ctx context.Context, action, resource string, detail any) error {
	raw, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.audit(world_id,actor,action,resource,detail) VALUES($1,$2,$3,$4,$5)`, sc.world.ID, sc.actor, action, resource, raw)
	return err
}
func (sc *scope) snapshot(ctx context.Context, revision int64) (Snapshot, error) {
	var raw []byte
	err := sc.tx.QueryRow(ctx, `SELECT snapshot FROM worldbuilder.revision WHERE world_id=$1 AND revision=$2`, sc.world.ID, revision).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	var s Snapshot
	err = json.Unmarshal(raw, &s)
	return s, err
}
func (sc *scope) member(ctx context.Context, principal, role string) error {
	principal = strings.ToLower(strings.TrimSpace(principal))
	if !sc.can("owner") {
		return forbidden
	}
	if principal == sc.actor || !strings.Contains(principal, "@") {
		return bad("cannot modify owner or use an invalid principal")
	}
	if role != "admin" && role != "editor" && role != "reviewer" && role != "reader" {
		return bad("unsupported membership role")
	}
	_, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.member VALUES($1,$2,$3) ON CONFLICT(world_id,principal) DO UPDATE SET role=excluded.role`, sc.world.ID, principal, role)
	if err != nil {
		return err
	}
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.world SET admin_version=admin_version+1 WHERE id=$1`, sc.world.ID); err != nil {
		return err
	}
	return sc.audit(ctx, "member.set", principal, map[string]string{"role": role})
}
func (sc *scope) change(ctx context.Context, id string) (Change, error) {
	if _, err := uuid.Parse(id); err != nil {
		return Change{}, missing
	}
	var c Change
	var raw []byte
	err := sc.tx.QueryRow(ctx, `SELECT id::text,author,base_revision,reason,operations,status,coalesce(reviewer,''),coalesce(review_reason,''),published_revision FROM worldbuilder.change_set WHERE world_id=$1 AND id=$2`, sc.world.ID, id).Scan(&c.ID, &c.Author, &c.BaseRevision, &c.Reason, &raw, &c.Status, &c.Reviewer, &c.ReviewReason, &c.PublishedRevision)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, missing
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(raw, &c.Operations)
	return c, err
}
func (sc *scope) propose(ctx context.Context, c Change) (Change, error) {
	if !sc.can("owner", "admin", "editor") {
		return c, forbidden
	}
	if _, err := uuid.Parse(c.ID); err != nil || strings.TrimSpace(c.Reason) == "" || len(c.Reason) > 2000 {
		return c, bad("proposal requires UUID id and reason (max 2000 characters)")
	}
	raw, err := json.Marshal(c.Operations)
	if err != nil {
		return c, err
	}
	if existing, e := sc.change(ctx, c.ID); e == nil {
		prev, _ := json.Marshal(existing.Operations)
		if existing.Author != sc.actor || existing.BaseRevision != c.BaseRevision || existing.Reason != c.Reason || string(prev) != string(raw) {
			return c, conflict("proposal id reused with different content")
		}
		return existing, nil
	} else if e != missing {
		return c, e
	}
	if c.BaseRevision != sc.world.Revision {
		return c, conflict("proposal is based on stale canon; rebase and review again")
	}
	before, err := sc.snapshot(ctx, c.BaseRevision)
	if err != nil {
		return c, err
	}
	after, err := Apply(before, c.Operations)
	if err != nil {
		return c, err
	}
	if err = sc.validateDefaultTimeline(after); err != nil {
		return c, err
	}
	if err = sc.validateIdentity(ctx, c.Operations); err != nil {
		return c, err
	}
	_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.change_set(world_id,id,author,base_revision,reason,operations) VALUES($1,$2,$3,$4,$5,$6)`, sc.world.ID, c.ID, sc.actor, c.BaseRevision, c.Reason, raw)
	if err != nil {
		return c, err
	}
	c.Author = sc.actor
	c.Status = "draft"
	return c, sc.audit(ctx, "change.draft", c.ID, map[string]any{"reason": c.Reason, "baseRevision": c.BaseRevision})
}
func (sc *scope) submit(ctx context.Context, id string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if c.Author != sc.actor || !sc.can("owner", "admin", "editor") {
		return c, forbidden
	}
	if c.Status == "ready_for_review" {
		return c, nil
	}
	if c.Status != "draft" {
		return c, conflict("only drafts can be submitted")
	}
	if err = sc.validateChange(ctx, c); err != nil {
		return c, err
	}
	_, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.change_set SET status='ready_for_review' WHERE world_id=$1 AND id=$2`, sc.world.ID, id)
	if err != nil {
		return c, err
	}
	c.Status = "ready_for_review"
	return c, sc.audit(ctx, "change.submit", id, map[string]string{})
}
func (sc *scope) validateChange(ctx context.Context, c Change) error {
	if c.BaseRevision != sc.world.Revision {
		return conflict("canon changed; create a rebased proposal and obtain new approval")
	}
	s, err := sc.snapshot(ctx, sc.world.Revision)
	if err != nil {
		return err
	}
	after, err := Apply(s, c.Operations)
	if err != nil {
		return err
	}
	if err = sc.validateDefaultTimeline(after); err != nil {
		return err
	}
	return sc.validateIdentity(ctx, c.Operations)
}
func (sc *scope) review(ctx context.Context, id, decision, reason string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if !sc.can("owner", "admin", "reviewer") || c.Author == sc.actor {
		return c, forbidden
	}
	if decision != "approved" && decision != "rejected" {
		return c, bad("review decision must be approved or rejected")
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 2000 {
		return c, bad("review reason required (max 2000 characters)")
	}
	if c.Status == decision && c.Reviewer == sc.actor && c.ReviewReason == reason {
		return c, nil
	}
	if c.Status != "ready_for_review" {
		return c, conflict("change is not awaiting review")
	}
	if decision == "approved" {
		if err = sc.validateChange(ctx, c); err != nil {
			return c, err
		}
	}
	_, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.change_set SET status=$3,reviewer=$4,review_reason=$5 WHERE world_id=$1 AND id=$2`, sc.world.ID, id, decision, sc.actor, reason)
	if err != nil {
		return c, err
	}
	c.Status = decision
	c.Reviewer = sc.actor
	c.ReviewReason = reason
	return c, sc.audit(ctx, "change.review", id, map[string]string{"decision": decision, "reason": reason})
}
func (sc *scope) publish(ctx context.Context, id string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if !sc.can("owner", "admin", "reviewer") {
		return c, forbidden
	}
	if c.Status == "published" {
		return c, nil
	}
	if c.Status != "approved" {
		return c, conflict("publication requires independent human approval")
	}
	var reviewerRole string
	err = sc.tx.QueryRow(ctx, `SELECT role FROM worldbuilder.member WHERE world_id=$1 AND principal=$2`, sc.world.ID, c.Reviewer).Scan(&reviewerRole)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && reviewerRole != "owner" && reviewerRole != "admin" && reviewerRole != "reviewer") {
		return c, conflict("reviewer no longer has approval authority")
	}
	if err != nil {
		return c, err
	}
	if err = sc.validateChange(ctx, c); err != nil {
		return c, err
	}
	before, err := sc.snapshot(ctx, sc.world.Revision)
	if err != nil {
		return c, err
	}
	after, err := Apply(before, c.Operations)
	if err != nil {
		return c, err
	}
	next := sc.world.Revision + 1
	raw, _ := json.Marshal(after)
	if _, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.revision(world_id,revision,snapshot,actor,reason) VALUES($1,$2,$3,$4,$5)`, sc.world.ID, next, raw, sc.actor, c.Reason); err != nil {
		return c, err
	}
	if err = sc.project(ctx, after); err != nil {
		return c, err
	}
	name, timeline := sc.world.Name, sc.world.DefaultTimeline
	for _, r := range after {
		if r.Kind == "world_settings" {
			name, timeline = r.Label, r.Timeline
		}
	}
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.world SET revision=$2,name=$3,default_timeline=$4 WHERE id=$1`, sc.world.ID, next, name, timeline); err != nil {
		return c, err
	}
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.change_set SET status='published',published_revision=$3 WHERE world_id=$1 AND id=$2`, sc.world.ID, id, next); err != nil {
		return c, err
	}
	eventID := uuid.NewString()
	payload, _ := json.Marshal(map[string]any{"eventId": eventID, "worldId": sc.world.ID, "revision": next, "changeId": id, "schemaVersion": 1})
	if _, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.outbox(id,world_id,revision,payload) VALUES($1,$2,$3,$4)`, eventID, sc.world.ID, next, payload); err != nil {
		return c, err
	}
	c.Status = "published"
	c.PublishedRevision = &next
	return c, sc.audit(ctx, "change.publish", id, map[string]any{"revision": next, "reviewer": c.Reviewer})
}
func (sc *scope) project(ctx context.Context, s Snapshot) error {
	if _, err := sc.tx.Exec(ctx, `DELETE FROM worldbuilder.record WHERE world_id=$1`, sc.world.ID); err != nil {
		return err
	}
	for _, r := range s {
		value, _ := json.Marshal(r.Value)
		attrs, _ := json.Marshal(r.Attributes)
		fields, _ := json.Marshal(r.Fields)
		relations, _ := json.Marshal(r.Relations)
		var calendar []byte
		if r.Calendar != nil {
			calendar, _ = json.Marshal(r.Calendar)
		}
		_, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.record(world_id,id,kind,label,type_id,subject_id,target_id,timeline_id,source_id,predicate,value_type,value,valid_from,valid_until,attributes,fields,relations,calendar) VALUES($1,$2,$3,$4,nullif($5,''),nullif($6,''),nullif($7,''),nullif($8,''),nullif($9,''),$10,$11,$12,$13,$14,$15,$16,$17,$18)`, sc.world.ID, r.ID, r.Kind, r.Label, r.Type, r.Subject, r.Target, r.Timeline, r.Source, r.Predicate, r.ValueType, value, r.From, r.Until, attrs, fields, relations, calendar)
		if err != nil {
			return err
		}
	}
	return nil
}
