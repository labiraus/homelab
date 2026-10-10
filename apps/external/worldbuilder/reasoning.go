package worldbuilder

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type ReasonRequest struct {
	Story        *StoryContext `json:"story,omitempty"`
	RequestID    string        `json:"requestId,omitempty"`
	WorldID      string        `json:"worldId"`
	Revision     int64         `json:"revision"`
	Timeline     string        `json:"timeline"`
	Question     string        `json:"question"`
	Workflow     string        `json:"workflow"`
	Evidence     []Evidence    `json:"evidence"`
	Provisional  Snapshot      `json:"provisional,omitempty"`
	ContextScope string        `json:"contextScope"`
}
type ReasonResult struct {
	SessionVersion *int64         `json:"sessionVersion,omitempty"`
	Choices        []StoryChoice  `json:"choices,omitempty"`
	MemorySummary  string         `json:"memorySummary,omitempty"`
	Provenance     Provenance     `json:"provenance"`
	BaseRevision   int64          `json:"baseRevision"`
	Answer         string         `json:"answer"`
	Unknown        bool           `json:"unknown"`
	Citations      []string       `json:"citations"`
	Operations     []Operation    `json:"operations"`
	Model          string         `json:"model"`
	Usage          map[string]any `json:"usage"`
	EvidenceScope  EvidenceScope  `json:"evidenceScope"`
}
type Reasoner func(context.Context, ReasonRequest) (ReasonResult, error)

func (sc *scope) reason(w http.ResponseWriter, r *http.Request, fn Reasoner, budget *InferenceBudget) (response any, returnErr error) {
	if fn == nil {
		return nil, &Problem{503, "unavailable", "Worldbuilder inference is not configured"}
	}
	var b struct {
		Workflow        string `json:"workflow"`
		Question        string `json:"question"`
		Timeline        string `json:"timeline"`
		Revision        int64  `json:"revision"`
		SessionID       string `json:"sessionId,omitempty"`
		ExpectedVersion *int64 `json:"expectedVersion,omitempty"`
	}
	if err := decode(w, r, &b); err != nil {
		return nil, err
	}
	if len(b.Question) == 0 || len(b.Question) > 4000 {
		return nil, bad("question required (max 4000 characters)")
	}
	if b.Workflow != "canon_query" && b.Workflow != "canon_proposal" && b.Workflow != "consistency_check" && b.Workflow != "story_turn" && b.Workflow != "narrative_generate" {
		return nil, bad("unsupported reasoning workflow")
	}
	if b.Workflow == "canon_proposal" && !sc.can("owner", "admin", "editor") {
		return nil, forbidden
	}
	if b.Workflow == "canon_proposal" && b.Revision != sc.world.Revision {
		return nil, conflict("AI proposals require current canon; reload before generating")
	}
	var provisional Snapshot
	var story *StoryContext
	var requiredRefs []string
	if b.Workflow == "story_turn" {
		s, err := sc.session(r.Context(), b.SessionID)
		if err != nil {
			return nil, err
		}
		if s.PinnedRevision != b.Revision || s.Timeline != b.Timeline {
			return nil, bad("story scope does not match pinned canon")
		}
		if b.ExpectedVersion == nil || *b.ExpectedVersion != s.Version {
			return nil, conflict("story version changed; reload before generating")
		}
		provisional = s.State
		context, e := sc.storyContext(r.Context(), s)
		if e != nil {
			return nil, e
		}
		story = &context
		if s.Definition != nil {
			requiredRefs = s.Definition.Participants
		}
	}
	snapshot, err := sc.snapshot(r.Context(), b.Revision)
	if err != nil {
		return nil, err
	}
	if snapshot[b.Timeline].Kind != "timeline" {
		return nil, bad("unknown timeline")
	}
	records, evidenceScope, err := sc.evidenceContext(r.Context(), snapshot, b.Revision, b.Timeline, b.Question, provisional, requiredRefs...)
	if err != nil {
		return nil, err
	}
	request := ReasonRequest{WorldID: sc.world.ID, Revision: b.Revision, Timeline: b.Timeline, Question: b.Question, Workflow: b.Workflow, Evidence: Citations(sc.world.ID, b.Revision, b.Timeline, records), Provisional: provisional, ContextScope: evidenceScope.Coverage}
	request.Story = story
	raw, _ := json.Marshal(request)
	if len(raw)+100 > 100000 { // Reserve space for the correlation ID.
		return nil, bad("canon context exceeds inference budget")
	}
	provenance := Provenance{Workflow: b.Workflow}
	if budget != nil {
		provenance, err = budget.reserve(r.Context(), sc.actor, b.SessionID, request)
		if err != nil {
			return nil, err
		}
		request.RequestID = provenance.RequestID
	}
	started := time.Now()
	var completed ReasonResult
	defer func() {
		if budget == nil {
			return
		}
		provenance.LatencyMS = time.Since(started).Milliseconds()
		slog.InfoContext(r.Context(), "worldbuilder inference completed", "requestId", provenance.RequestID, "workflow", b.Workflow, "latencyMs", provenance.LatencyMS, "valid", returnErr == nil)
		if e := budget.finish(provenance, completed, returnErr == nil); e != nil {
			slog.ErrorContext(r.Context(), "inference ledger completion failed", "requestId", provenance.RequestID)
			if returnErr == nil {
				response = nil
				returnErr = &Problem{503, "inference_ledger", "inference completion could not be recorded"}
			}
		}
	}()
	result, err := fn(r.Context(), request)
	completed = result
	if err != nil {
		return nil, err
	}
	allowed := map[string]bool{}
	for _, e := range request.Evidence {
		allowed[e.ID] = true
	}
	for _, id := range result.Citations {
		if !allowed[id] {
			return nil, bad("inference returned an unresolvable citation")
		}
	}
	if !result.Unknown && len(result.Citations) == 0 {
		return nil, bad("inference returned an ungrounded answer")
	}
	if len(result.Answer) > 16000 || strings.TrimSpace(result.Answer) == "" {
		return nil, bad("invalid inference response")
	}
	if (b.Workflow == "canon_query" || b.Workflow == "consistency_check" || b.Workflow == "narrative_generate") && len(result.Operations) > 0 {
		return nil, bad("read-only workflow returned mutations")
	}
	if err = validateStoryMemory(result.MemorySummary, result.Choices); err != nil {
		return nil, err
	}
	if result.MemorySummary != "" && b.Workflow != "story_turn" {
		return nil, bad("only story turns may propose a memory summary")
	}
	if len(result.Choices) > 0 && b.Workflow != "story_turn" && b.Workflow != "narrative_generate" {
		return nil, bad("choices require a creative workflow")
	}
	result.SessionVersion = nil
	if story != nil {
		result.SessionVersion = &story.Version
	}
	if len(result.Operations) > 0 {
		if b.Workflow == "story_turn" {
			if err = validateStoryOperations(result.Operations, b.Timeline); err != nil {
				return nil, err
			}
		}
		combined := Snapshot{}
		for k, v := range snapshot {
			combined[k] = v
		}
		for k, v := range provisional {
			combined[k] = v
		}
		if _, err = Apply(combined, result.Operations); err != nil {
			return nil, err
		}
	}
	result.BaseRevision = b.Revision
	result.EvidenceScope = evidenceScope
	provenance.LatencyMS = time.Since(started).Milliseconds()
	result.Provenance = provenance
	// Output remains a candidate. UI explicitly creates a draft or persists a provisional turn.
	err = sc.audit(r.Context(), "ai."+b.Workflow, b.SessionID, map[string]any{"revision": b.Revision, "timeline": b.Timeline, "model": result.Model, "usage": result.Usage, "citations": result.Citations, "evidenceScope": evidenceScope, "provenance": provenance})
	return result, err
}
