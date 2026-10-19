package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"pkg/canonsearch"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	Pool        *pgxpool.Pool
	Reasoner    Reasoner
	Discover    Discover
	Vector      *canonsearch.Index
	Budget      *InferenceBudget
	JobsEnabled bool
}
type scope struct {
	tx          pgx.Tx
	world       World
	actor       string
	role        string
	superuser   bool
	discover    Discover
	vector      *canonsearch.Index
	budget      *InferenceBudget
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
	sc := &scope{tx: tx, actor: actor, discover: s.Discover, vector: s.Vector, budget: s.Budget, jobsEnabled: s.JobsEnabled}
	// Serialise membership, publication and session changes within each world.
	err = tx.QueryRow(ctx, `SELECT id::text,name,revision,default_timeline,archived,admin_version,publication_policy,policy_version FROM worldbuilder.world WHERE id=$1 FOR UPDATE`, world).Scan(&sc.world.ID, &sc.world.Name, &sc.world.Revision, &sc.world.DefaultTimeline, &sc.world.Archived, &sc.world.AdminVersion, &sc.world.Policy, &sc.world.PolicyVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	var authority *string
	if err = tx.QueryRow(ctx, `SELECT worldbuilder.lock_superuser($1)::text`, actor).Scan(&authority); err != nil {
		return nil, err
	}
	sc.superuser = authority != nil
	err = tx.QueryRow(ctx, `SELECT role FROM worldbuilder.member WHERE world_id=$1 AND principal=$2`, world, actor).Scan(&sc.role)
	if errors.Is(err, pgx.ErrNoRows) && sc.superuser {
		sc.role = "author"
		err = nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, missing
	}
	if err != nil {
		return nil, err
	}
	sc.world.Role = sc.role
	sc.world.Superuser = sc.superuser
	sc.world.SemanticSearch = s.Vector != nil && s.Budget != nil
	sc.world.Capabilities = map[string]bool{"establishedAccounts": true, "conversations": true, "guides": true, "sandbox": true, "offlineDrafts": true, "suggest": true, "author": sc.can("author"), "administer": sc.can("author"), "configurePublication": sc.superuser}
	result, err := fn(sc)
	if err != nil {
		return nil, err
	}
	return result, tx.Commit(ctx)
}
func (s *Store) Worlds(ctx context.Context, actor string, limit, offset int64) ([]World, error) {
	rows, err := s.Pool.Query(ctx, `SELECT w.id::text,w.name,w.revision,COALESCE(m.role,'author'),w.default_timeline,w.archived,w.admin_version,w.publication_policy,w.policy_version,EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=$1) FROM worldbuilder.world w LEFT JOIN worldbuilder.member m ON m.world_id=w.id AND m.principal=$1 WHERE (m.principal=$1 OR EXISTS(SELECT FROM worldbuilder.superuser WHERE principal=$1)) ORDER BY w.id LIMIT $2 OFFSET $3`, actor, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []World{}
	for rows.Next() {
		var w World
		if err := rows.Scan(&w.ID, &w.Name, &w.Revision, &w.Role, &w.DefaultTimeline, &w.Archived, &w.AdminVersion, &w.Policy, &w.PolicyVersion, &w.Superuser); err != nil {
			return nil, err
		}
		w.SemanticSearch = s.Vector != nil && s.Budget != nil
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
		err = tx.QueryRow(ctx, `SELECT w.id::text,w.name,w.revision,m.role,w.default_timeline,w.archived,w.admin_version FROM worldbuilder.world w JOIN worldbuilder.member m ON m.world_id=w.id WHERE w.id=$1 AND m.principal=$2 AND m.role='author' AND EXISTS(SELECT FROM worldbuilder.audit a WHERE a.world_id=w.id AND a.action='world.create' AND a.actor=$2 AND a.detail->>'name'=$3)`, id, actor, name).Scan(&w.ID, &w.Name, &w.Revision, &w.Role, &w.DefaultTimeline, &w.Archived, &w.AdminVersion)
		if errors.Is(err, pgx.ErrNoRows) {
			return World{}, conflict("world id already used")
		}
		return w, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO worldbuilder.member(world_id,principal,role) VALUES($1,$2,'author')`, id, actor); err != nil {
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
	if err = sc.enqueueCanon(ctx, 0, ""); err != nil {
		return World{}, err
	}
	return World{ID: id, Name: name, Role: "author", DefaultTimeline: "main", Policy: PublicationPolicy{Mode: "independent", RequiredApprovals: 1}, PolicyVersion: 1}, tx.Commit(ctx)
}
func (sc *scope) can(roles ...string) bool {
	for _, r := range roles {
		if r == "owner" || r == "admin" || r == "editor" || r == "reviewer" {
			r = "author"
		}
		if sc.superuser || sc.role == r {
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
	if sc.world.Archived {
		return conflict("reopen the world before changing membership")
	}
	principal = strings.ToLower(strings.TrimSpace(principal))
	if !sc.can("owner") {
		return forbidden
	}
	if !strings.Contains(principal, "@") {
		return bad("valid registered principal required")
	}
	if role == "owner" || role == "admin" || role == "editor" || role == "reviewer" {
		role = "author"
	}
	if role != "author" && role != "reader" {
		return bad("role must be author or reader")
	}
	if role != "author" {
		if err := sc.protectLastAuthor(ctx, principal); err != nil {
			return err
		}
	}
	_, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.member(world_id,principal,role) VALUES($1,$2,$3) ON CONFLICT(world_id,principal) DO UPDATE SET role=excluded.role`, sc.world.ID, principal, role)
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
	err := sc.tx.QueryRow(ctx, `SELECT id::text,author,base_revision,reason,operations,status,coalesce(reviewer,''),coalesce(review_reason,''),published_revision,required_approvals,published_approvals,policy_version FROM worldbuilder.change_set WHERE world_id=$1 AND id=$2`, sc.world.ID, id).Scan(&c.ID, &c.Author, &c.BaseRevision, &c.Reason, &raw, &c.Status, &c.Reviewer, &c.ReviewReason, &c.PublishedRevision, &c.RequiredApprovals, &c.Approvals, &c.PolicyVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, missing
	}
	if err != nil {
		return c, err
	}
	if err = json.Unmarshal(raw, &c.Operations); err != nil {
		return c, err
	}
	snapshot, e := sc.snapshot(ctx, c.BaseRevision)
	if e != nil {
		return c, e
	}
	affected := map[string]bool{}
	for _, op := range c.Operations {
		affected[op.ID] = true
	}
	for _, r := range snapshot {
		if r.Kind == "established_account" {
			for _, ref := range r.References {
				if affected[ref] {
					c.EstablishedWarnings = append(c.EstablishedWarnings, r)
					break
				}
			}
		}
	}
	sort.Slice(c.EstablishedWarnings, func(i, j int) bool { return c.EstablishedWarnings[i].ID < c.EstablishedWarnings[j].ID })
	if err = sc.loadReviews(ctx, &c); err != nil {
		return c, err
	}
	c.EffectiveRequiredApprovals, err = sc.requiredApprovals(ctx)
	if err != nil {
		return c, err
	}
	if c.BaseRevision == sc.world.Revision && c.PolicyVersion == sc.world.PolicyVersion && !sc.world.Archived && (c.Status == "ready_for_review" || c.Status == "approved") {
		c.PublicationReady, err = sc.hasQuorum(ctx, c)
	}
	return c, err
}
func (sc *scope) propose(ctx context.Context, c Change) (Change, error) {
	if !sc.can("author", "reader") {
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
	if err = sc.checkPolicyEdit(before, after); err != nil {
		return c, err
	}
	c.RequiredApprovals, err = sc.requiredApprovals(ctx)
	if err != nil {
		return c, err
	}
	c.PolicyVersion = sc.world.PolicyVersion
	c.Approvals = []string{}
	c.Reviews = []Review{}
	_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.change_set(world_id,id,author,base_revision,reason,operations,required_approvals,policy_version) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, sc.world.ID, c.ID, sc.actor, c.BaseRevision, c.Reason, raw, c.RequiredApprovals, c.PolicyVersion)
	if err != nil {
		return c, err
	}
	c.Author = sc.actor
	c.Status = "draft"
	if err = sc.audit(ctx, "change.draft", c.ID, map[string]any{"reason": c.Reason, "baseRevision": c.BaseRevision}); err != nil {
		return c, err
	}
	return sc.change(ctx, c.ID)
}
func (sc *scope) submit(ctx context.Context, id string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if c.Author != sc.actor || !sc.can("author", "reader") {
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
	if err := sc.checkPolicyEdit(s, after); err != nil {
		return err
	}
	if c.PolicyVersion != sc.world.PolicyVersion {
		return conflict("review policy changed; create a fresh proposal")
	}
	return sc.validateIdentity(ctx, c.Operations)
}
func (sc *scope) review(ctx context.Context, id, decision, reason string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if !sc.can("author") || (c.Author == sc.actor && sc.world.Policy.Mode != "unanimous") {
		return c, forbidden
	}
	if decision != "approved" && decision != "rejected" {
		return c, bad("review decision must be approved or rejected")
	}
	if strings.TrimSpace(reason) == "" || len(reason) > 2000 {
		return c, bad("review reason required (max 2000 characters)")
	}
	for _, r := range c.Reviews {
		if r.Reviewer == sc.actor && r.Eligible {
			if r.Decision == decision && r.Reason == reason {
				return c, nil
			}
			return c, conflict("a review decision is immutable; reconcile into a fresh proposal")
		}
	}
	if c.Status != "ready_for_review" && c.Status != "approved" {
		return c, conflict("change is not awaiting review")
	}
	if len(c.Reviews) >= 100 {
		return c, conflict("review history limit reached; create a fresh proposal")
	}
	if decision == "approved" {
		if err = sc.validateChange(ctx, c); err != nil {
			return c, err
		}
	}
	_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.change_review(world_id,change_id,reviewer,authority_id,decision,reason,policy_version,superuser_authority_id)
 VALUES($1,$2,$3,COALESCE((SELECT authority_id FROM worldbuilder.member WHERE world_id=$1 AND principal=$3),(SELECT authority_id FROM worldbuilder.superuser WHERE principal=$3)),$4,$5,$6,(SELECT authority_id FROM worldbuilder.superuser WHERE principal=$3))`, sc.world.ID, id, sc.actor, decision, reason, sc.world.PolicyVersion)
	if err != nil {
		return c, err
	}
	status := decision
	if decision == "approved" && len(c.Approvals)+1 < c.RequiredApprovals {
		status = "ready_for_review"
	}
	_, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.change_set SET status=$3,reviewer=$4,review_reason=$5 WHERE world_id=$1 AND id=$2`, sc.world.ID, id, status, sc.actor, reason)
	if err != nil {
		return c, err
	}
	if err = sc.audit(ctx, "change.review", id, map[string]any{"decision": decision, "reason": reason, "requiredApprovals": c.RequiredApprovals, "superuserSelfReview": c.Author == sc.actor}); err != nil {
		return c, err
	}
	return sc.change(ctx, id)
}
func (sc *scope) publish(ctx context.Context, id string) (Change, error) {
	return sc.publishWithOverride(ctx, id, "")
}
func (sc *scope) publishWithOverride(ctx context.Context, id, override string) (Change, error) {
	c, err := sc.change(ctx, id)
	if err != nil {
		return c, err
	}
	if !sc.can("owner", "admin", "reviewer") && !(sc.superuser && c.Author == sc.actor) {
		return c, forbidden
	}
	if c.Status == "published" {
		return c, nil
	}
	if override != "" && (!sc.superuser || len(strings.TrimSpace(override)) == 0 || len(override) > 2000) {
		return c, forbidden
	}
	if c.Status == "rejected" || c.Status == "draft" {
		return c, conflict("submit a valid proposal before publication")
	}
	enough, err := sc.hasQuorum(ctx, c)
	if err != nil {
		return c, err
	}
	if override == "" && !enough {
		return c, conflict("approval quorum is no longer satisfied; obtain fresh reviews")
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
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.change_set SET status='published',published_revision=$3,published_approvals=$4 WHERE world_id=$1 AND id=$2`, sc.world.ID, id, next, c.Approvals); err != nil {
		return c, err
	}
	if err = sc.enqueueCanon(ctx, next, id); err != nil {
		return c, err
	}
	c.Status = "published"
	c.PublishedRevision = &next
	return c, sc.audit(ctx, "change.publish", id, map[string]any{"revision": next, "reviewer": c.Reviewer, "approvals": c.Approvals, "requiredApprovals": c.RequiredApprovals, "overrideReason": override})
}
func (sc *scope) enqueueCanon(ctx context.Context, revision int64, changeID string) error {
	eventID := uuid.NewString()
	payload := map[string]any{"eventId": eventID, "worldId": sc.world.ID, "revision": revision, "schemaVersion": 1}
	if changeID != "" {
		payload["changeId"] = changeID
	}
	raw, _ := json.Marshal(payload)
	_, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.outbox(id,world_id,revision,payload) VALUES($1,$2,$3,$4)`, eventID, sc.world.ID, revision, raw)
	return err
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
		var calendar, reviewPolicy []byte
		if r.Calendar != nil {
			calendar, _ = json.Marshal(r.Calendar)
		}
		if r.ReviewPolicy != nil {
			reviewPolicy, _ = json.Marshal(r.ReviewPolicy)
		}
		_, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.record(world_id,id,kind,label,type_id,subject_id,target_id,timeline_id,source_id,predicate,value_type,value,valid_from,valid_until,attributes,fields,relations,calendar,review_policy) VALUES($1,$2,$3,$4,nullif($5,''),nullif($6,''),nullif($7,''),nullif($8,''),nullif($9,''),$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`, sc.world.ID, r.ID, r.Kind, r.Label, r.Type, r.Subject, r.Target, r.Timeline, r.Source, r.Predicate, r.ValueType, value, r.From, r.Until, attrs, fields, relations, calendar, reviewPolicy)
		if err != nil {
			return err
		}
	}
	return nil
}
