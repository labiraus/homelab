package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Session struct {
	DefinitionID   string           `json:"definitionId,omitempty"`
	Definition     *StoryDefinition `json:"definition,omitempty"`
	Memory         StoryMemory      `json:"memory"`
	ID             string           `json:"id"`
	Title          string           `json:"title"`
	Owner          string           `json:"owner"`
	PinnedRevision int64            `json:"pinnedRevision"`
	Timeline       string           `json:"timeline"`
	Version        int64            `json:"version"`
	State          Snapshot         `json:"state"`
}
type Turn struct {
	RequestID       string        `json:"requestId"`
	ExpectedVersion int64         `json:"expectedVersion"`
	Narration       string        `json:"narration"`
	Operations      []Operation   `json:"operations"`
	Direction       string        `json:"direction,omitempty"`
	MemorySummary   string        `json:"memorySummary,omitempty"`
	Choices         []StoryChoice `json:"choices,omitempty"`
}

func (sc *scope) session(ctx context.Context, id string) (Session, error) {
	var s Session
	var raw, memory []byte
	if _, err := uuid.Parse(id); err != nil {
		return s, missing
	}
	err := sc.tx.QueryRow(ctx, `SELECT id::text,title,owner,pinned_revision,timeline_id,version,state,COALESCE(definition_id::text,''),memory FROM worldbuilder.story_session WHERE world_id=$1 AND id=$2 AND owner=$3`, sc.world.ID, id, sc.actor).Scan(&s.ID, &s.Title, &s.Owner, &s.PinnedRevision, &s.Timeline, &s.Version, &raw, &s.DefinitionID, &memory)
	if errors.Is(err, pgx.ErrNoRows) {
		return s, missing
	}
	if err != nil {
		return s, err
	}
	err = json.Unmarshal(raw, &s.State)
	if err != nil {
		return s, err
	}
	if err = json.Unmarshal(memory, &s.Memory); err != nil {
		return s, err
	}
	if s.DefinitionID != "" {
		d, e := sc.definition(ctx, s.DefinitionID)
		if e != nil {
			return s, e
		}
		s.Definition = &d
	}
	return s, err
}
func (sc *scope) createSession(ctx context.Context, s Session) (Session, error) {
	if _, err := uuid.Parse(s.ID); err != nil || strings.TrimSpace(s.Title) == "" || len(s.Title) > 200 {
		return s, bad("session requires UUID id and title (max 200 characters)")
	}
	if old, err := sc.session(ctx, s.ID); err == nil {
		if old.Title != s.Title || old.PinnedRevision != s.PinnedRevision || old.Timeline != s.Timeline || old.DefinitionID != s.DefinitionID {
			return s, conflict("session id reused")
		}
		return old, nil
	} else if err != missing {
		return s, err
	}
	canon, err := sc.snapshot(ctx, s.PinnedRevision)
	if err != nil {
		return s, err
	}
	if canon[s.Timeline].Kind != "timeline" {
		return s, bad("timeline is absent at pinned revision")
	}
	var definitionID *string
	if s.DefinitionID != "" {
		d, e := sc.definition(ctx, s.DefinitionID)
		if e != nil {
			return s, e
		}
		if d.PinnedRevision != s.PinnedRevision || d.Timeline != s.Timeline {
			return s, bad("session must match its definition pin")
		}
		definitionID = &s.DefinitionID
	}
	_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.story_session(world_id,id,owner,title,pinned_revision,timeline_id,definition_id) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, sc.world.ID, s.ID, sc.actor, s.Title, s.PinnedRevision, s.Timeline, definitionID)
	if err != nil {
		return s, err
	}
	s, err = sc.session(ctx, s.ID)
	if err != nil {
		return s, err
	}
	return s, sc.audit(ctx, "story.create", s.ID, map[string]any{"pinnedRevision": s.PinnedRevision, "timeline": s.Timeline})
}
func (sc *scope) turn(ctx context.Context, id string, t Turn) (any, error) {
	s, err := sc.session(ctx, id)
	if err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(t.RequestID); err != nil || strings.TrimSpace(t.Narration) == "" || len(t.Narration) > 16000 {
		return nil, bad("turn requires UUID requestId and narration (max 16000 characters)")
	}
	if len(t.Direction) > 4000 {
		return nil, bad("story direction exceeds budget")
	}
	if err = validateStoryMemory(t.MemorySummary, t.Choices); err != nil {
		return nil, err
	}
	request, _ := json.Marshal(t)
	var oldRequest, oldState, oldMemory []byte
	var oldVersion int64
	err = sc.tx.QueryRow(ctx, `SELECT request,state,version,memory FROM worldbuilder.story_turn WHERE world_id=$1 AND session_id=$2 AND request_id=$3`, sc.world.ID, id, t.RequestID).Scan(&oldRequest, &oldState, &oldVersion, &oldMemory)
	if err == nil {
		var previous Turn
		if err = json.Unmarshal(oldRequest, &previous); err != nil {
			return nil, err
		}
		normalized, _ := json.Marshal(previous)
		if string(normalized) != string(request) {
			return nil, conflict("turn requestId reused with different content")
		}
		return map[string]any{"version": oldVersion, "state": json.RawMessage(oldState), "narration": t.Narration, "memory": json.RawMessage(oldMemory)}, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	if s.Version != t.ExpectedVersion {
		return nil, conflict("story version changed; reload before another turn")
	}
	pinned, err := sc.snapshot(ctx, s.PinnedRevision)
	if err != nil {
		return nil, err
	}
	combined := Snapshot{}
	for k, v := range pinned {
		combined[k] = v
	}
	for k, v := range s.State {
		combined[k] = v
	}
	if err = validateStoryOperations(t.Operations, s.Timeline); err != nil {
		return nil, err
	}
	if len(t.Operations) > 0 {
		combined, err = Apply(combined, t.Operations)
		if err != nil {
			return nil, err
		}
	}
	for _, op := range t.Operations {
		s.State[op.ID] = combined[op.ID]
	}
	state, _ := json.Marshal(s.State)
	next := s.Version + 1
	if t.MemorySummary != "" {
		s.Memory.Summary = t.MemorySummary
		s.Memory.ThroughVersion = next
	}
	s.Memory.Choices = t.Choices
	if s.Memory.Choices == nil {
		s.Memory.Choices = []StoryChoice{}
	}
	memory, _ := json.Marshal(s.Memory)
	if _, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.story_turn(world_id,session_id,version,request_id,request,narration,state,memory) VALUES($1,$2,$3,$4,$5,$6,$7,$8)`, sc.world.ID, id, next, t.RequestID, request, t.Narration, state, memory); err != nil {
		return nil, err
	}
	if _, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.story_session SET version=$3,state=$4,memory=$5 WHERE world_id=$1 AND id=$2`, sc.world.ID, id, next, state, memory); err != nil {
		return nil, err
	}
	if err = sc.audit(ctx, "story.turn", id, map[string]any{"version": next, "requestId": t.RequestID}); err != nil {
		return nil, err
	}
	return map[string]any{"version": next, "state": s.State, "narration": t.Narration, "memory": s.Memory}, nil
}

type Outcome struct {
	ID              string `json:"id"`
	ExpectedVersion int64  `json:"expectedVersion"`
	BaseRevision    int64  `json:"baseRevision"`
	Reason          string `json:"reason"`
}

func (sc *scope) outcome(ctx context.Context, id string, o Outcome) (Change, error) {
	s, err := sc.session(ctx, id)
	if err != nil {
		return Change{}, err
	}
	if _, err := uuid.Parse(o.ID); err != nil {
		return Change{}, bad("outcome requires UUID id")
	}
	raw, _ := json.Marshal(o)
	var old []byte
	var changeID string
	err = sc.tx.QueryRow(ctx, `SELECT change_id::text,request FROM worldbuilder.story_outcome WHERE world_id=$1 AND session_id=$2 AND request_id=$3`, sc.world.ID, id, o.ID).Scan(&changeID, &old)
	if err == nil {
		var prev Outcome
		if err = json.Unmarshal(old, &prev); err != nil {
			return Change{}, err
		}
		norm, _ := json.Marshal(prev)
		if string(norm) != string(raw) {
			return Change{}, conflict("outcome request id reused")
		}
		return sc.change(ctx, changeID)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return Change{}, err
	}
	if s.Version != o.ExpectedVersion {
		return Change{}, conflict("story version changed")
	}
	// Never silently rebase a story onto changed canon; author must reconcile explicitly.
	if o.BaseRevision != s.PinnedRevision || sc.world.Revision != s.PinnedRevision {
		return Change{}, conflict("canon advanced since this story began; compare pinned/current snapshots and create a reconciled proposal")
	}
	ops := []Operation{}
	for _, r := range Records(s.State, s.Timeline, "", nil) {
		copy := r
		ops = append(ops, Operation{Action: "put", ID: r.ID, Record: &copy})
	}
	c, err := sc.propose(ctx, Change{ID: o.ID, BaseRevision: o.BaseRevision, Reason: o.Reason, Operations: ops})
	if err != nil {
		return c, err
	}
	if _, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.story_outcome(world_id,session_id,request_id,change_id,request) VALUES($1,$2,$3,$3,$4)`, sc.world.ID, id, o.ID, raw); err != nil {
		return c, err
	}
	return c, sc.audit(ctx, "story.outcome", id, map[string]string{"changeId": c.ID})
}
