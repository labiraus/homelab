package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type StoryDefinition struct {
	ID             string   `json:"id"`
	Owner          string   `json:"owner,omitempty"`
	Title          string   `json:"title"`
	PinnedRevision int64    `json:"pinnedRevision"`
	Timeline       string   `json:"timeline"`
	Premise        string   `json:"premise"`
	Objective      string   `json:"objective"`
	Style          string   `json:"style"`
	Participants   []string `json:"participants"`
}
type StoryChoice struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Direction string `json:"direction"`
}
type StoryMemory struct {
	Summary        string        `json:"summary"`
	ThroughVersion int64         `json:"throughVersion"`
	Choices        []StoryChoice `json:"choices"`
}
type StoryContext struct {
	Definition  *StoryDefinition `json:"definition,omitempty"`
	Memory      StoryMemory      `json:"memory"`
	RecentTurns []map[string]any `json:"recentTurns"`
	Version     int64            `json:"version"`
}

func validateStoryOperations(operations []Operation, timeline string) error {
	for _, op := range operations {
		if op.Action != "put" || op.Record == nil || (op.Record.Kind != "fact" && op.Record.Kind != "relationship" && op.Record.Kind != "event") || op.Record.Timeline != timeline {
			return bad("story transitions may only put facts, relationships and events in their pinned timeline")
		}
	}
	return nil
}

func validateStoryMemory(summary string, choices []StoryChoice) error {
	if len(summary) > 4000 || len(choices) > 6 {
		return bad("story summary or choices exceed memory budget")
	}
	ids := map[string]bool{}
	for _, choice := range choices {
		if choice.ID == "" || len(choice.ID) > 80 || ids[choice.ID] || strings.TrimSpace(choice.Label) == "" || len(choice.Label) > 200 || strings.TrimSpace(choice.Direction) == "" || len(choice.Direction) > 1000 {
			return bad("invalid or duplicate story choice")
		}
		ids[choice.ID] = true
	}
	return nil
}

func (sc *scope) definition(ctx context.Context, id string) (StoryDefinition, error) {
	var d StoryDefinition
	if _, err := uuid.Parse(id); err != nil {
		return d, missing
	}
	var raw []byte
	err := sc.tx.QueryRow(ctx, `SELECT definition FROM worldbuilder.story_definition WHERE world_id=$1 AND id=$2 AND owner=$3`, sc.world.ID, id, sc.actor).Scan(&raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, missing
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(raw, &d)
	return d, err
}
func (sc *scope) createDefinition(ctx context.Context, d StoryDefinition) (StoryDefinition, error) {
	if _, err := uuid.Parse(d.ID); err != nil || strings.TrimSpace(d.Title) == "" || len(d.Title) > 200 || len(d.Premise) > 16000 || len(d.Objective) > 2000 || len(d.Style) > 2000 || len(d.Participants) > 20 || d.Owner != "" {
		return d, bad("invalid private story definition")
	}
	d.Owner = sc.actor
	if d.Participants == nil {
		d.Participants = []string{}
	}
	raw, _ := json.Marshal(d)
	if old, err := sc.definition(ctx, d.ID); err == nil {
		before, _ := json.Marshal(old)
		if string(before) != string(raw) {
			return d, conflict("definition id reused; create a new definition to revise it")
		}
		return old, nil
	} else if err != missing {
		return d, err
	}
	canon, err := sc.snapshot(ctx, d.PinnedRevision)
	if err != nil {
		return d, err
	}
	if canon[d.Timeline].Kind != "timeline" {
		return d, bad("definition timeline absent at pinned revision")
	}
	seen := map[string]bool{}
	for _, id := range d.Participants {
		if seen[id] || canon[id].Kind != "entity" || !Visible(canon[id], d.Timeline, nil) {
			return d, bad("definition participants must be distinct entities visible at its pin")
		}
		seen[id] = true
	}
	tag, err := sc.tx.Exec(ctx, `INSERT INTO worldbuilder.story_definition(world_id,id,owner,pinned_revision,definition) VALUES($1,$2,$3,$4,$5) ON CONFLICT DO NOTHING`, sc.world.ID, d.ID, sc.actor, d.PinnedRevision, raw)
	if err != nil {
		return d, err
	}
	if tag.RowsAffected() == 0 {
		return d, conflict("definition id already used")
	}
	return d, sc.audit(ctx, "story.definition", d.ID, map[string]any{"pinnedRevision": d.PinnedRevision, "timeline": d.Timeline})
}
func (sc *scope) definitions(ctx context.Context, limit, offset int64) ([]StoryDefinition, error) {
	rows, err := sc.tx.Query(ctx, `SELECT definition FROM worldbuilder.story_definition WHERE world_id=$1 AND owner=$2 ORDER BY created_at DESC,id LIMIT $3 OFFSET $4`, sc.world.ID, sc.actor, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []StoryDefinition{}
	for rows.Next() {
		var raw []byte
		var d StoryDefinition
		if err = rows.Scan(&raw); err != nil {
			return nil, err
		}
		if err = json.Unmarshal(raw, &d); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
func (sc *scope) storyContext(ctx context.Context, s Session) (StoryContext, error) {
	c := StoryContext{Definition: s.Definition, Memory: s.Memory, Version: s.Version, RecentTurns: []map[string]any{}}
	rows, err := sc.tx.Query(ctx, `SELECT version,narration FROM worldbuilder.story_turn WHERE world_id=$1 AND session_id=$2 ORDER BY version DESC LIMIT 3`, sc.world.ID, s.ID)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		var version int64
		var narration string
		if err = rows.Scan(&version, &narration); err != nil {
			return c, err
		}
		runes := []rune(narration)
		truncated := len(runes) > 2000
		if truncated {
			narration = string(runes[:2000])
		}
		c.RecentTurns = append(c.RecentTurns, map[string]any{"version": version, "narration": narration, "truncated": truncated})
	}
	return c, rows.Err()
}
