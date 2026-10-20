package worldbuilder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Conversation struct {
	At       *ExactTick `json:"at,omitempty"`
	ID       string     `json:"id"`
	Title    string     `json:"title"`
	Creator  string     `json:"creator"`
	Revision int64      `json:"revision"`
	Timeline string     `json:"timeline"`
}
type ConversationMessage struct {
	ID       string        `json:"id"`
	Actor    string        `json:"actor"`
	Sequence int64         `json:"sequence"`
	Request  ReasonInput   `json:"request"`
	Result   *ReasonResult `json:"result,omitempty"`
	Status   string        `json:"status"`
}

func (sc *scope) conversations(w http.ResponseWriter, r *http.Request, path []string, limit, offset int64) (any, error) {
	ctx := r.Context()
	if len(path) == 1 {
		if r.Method == "POST" {
			var b Conversation
			if err := decode(w, r, &b); err != nil {
				return nil, err
			}
			if _, err := uuid.Parse(b.ID); err != nil || strings.TrimSpace(b.Title) == "" || len(b.Title) > 200 {
				return nil, bad("conversation requires UUID and title")
			}
			snap, err := sc.snapshot(ctx, b.Revision)
			if err != nil {
				return nil, err
			}
			if snap[b.Timeline].Kind != "timeline" {
				return nil, bad("unknown conversation timeline")
			}
			_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.conversation(world_id,id,title,creator,revision,timeline,at) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, sc.world.ID, b.ID, b.Title, sc.actor, b.Revision, b.Timeline, (*int64)(b.At))
			if err != nil {
				return nil, err
			}
			var old Conversation
			err = sc.tx.QueryRow(ctx, `SELECT id::text,title,creator,revision,timeline,at FROM worldbuilder.conversation WHERE world_id=$1 AND id=$2`, sc.world.ID, b.ID).Scan(&old.ID, &old.Title, &old.Creator, &old.Revision, &old.Timeline, &old.At)
			if err == nil && (old.Creator != sc.actor || old.Title != b.Title || old.Revision != b.Revision || old.Timeline != b.Timeline || !reflect.DeepEqual(old.At, b.At)) {
				return nil, conflict("conversation ID reused")
			}
			return old, err
		}
		if r.Method == "GET" {
			rows, err := sc.tx.Query(ctx, `SELECT id::text,title,creator,revision,timeline,at FROM worldbuilder.conversation WHERE world_id=$1 ORDER BY created_at DESC,id LIMIT $2 OFFSET $3`, sc.world.ID, limit, offset)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			result := []Conversation{}
			for rows.Next() {
				var c Conversation
				if err = rows.Scan(&c.ID, &c.Title, &c.Creator, &c.Revision, &c.Timeline, &c.At); err != nil {
					return nil, err
				}
				result = append(result, c)
			}
			return result, rows.Err()
		}
	}
	if len(path) == 3 && path[2] == "messages" {
		if _, err := uuid.Parse(path[1]); err != nil {
			return nil, missing
		}
		var revision int64
		var timeline string
		var at *ExactTick
		err := sc.tx.QueryRow(ctx, `SELECT revision,timeline,at FROM worldbuilder.conversation WHERE world_id=$1 AND id=$2`, sc.world.ID, path[1]).Scan(&revision, &timeline, &at)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, missing
		}
		if err != nil {
			return nil, err
		}
		if r.Method == "POST" {
			var b struct {
				ID      string      `json:"id"`
				Request ReasonInput `json:"request"`
			}
			if err := decode(w, r, &b); err != nil {
				return nil, err
			}
			if _, err := uuid.Parse(b.ID); err != nil || len(b.Request.Question) == 0 || len(b.Request.Question) > 4000 {
				return nil, bad("message requires UUID and text (max 4000 bytes)")
			}
			if b.Request.SessionID != "" || b.Request.ExpectedVersion != nil || b.Request.Workflow == "story_turn" {
				return nil, bad("private story context cannot enter a shared conversation")
			}
			if b.Request.Revision != revision || b.Request.Timeline != timeline || !reflect.DeepEqual(b.Request.At, at) {
				return nil, bad("message must retain conversation scope")
			}
			if b.Request.Workflow != "canon_query" && b.Request.Workflow != "canon_proposal" && b.Request.Workflow != "consistency_check" && b.Request.Workflow != "narrative_generate" {
				return nil, bad("unsupported shared workflow")
			}
			raw, _ := json.Marshal(b.Request)
			_, err = sc.tx.Exec(ctx, `INSERT INTO worldbuilder.conversation_message(world_id,conversation_id,id,actor,request,inference_profile) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT DO NOTHING`, sc.world.ID, path[1], b.ID, sc.actor, raw, sc.budget.pin())
			if err != nil {
				return nil, err
			}
			var same bool
			err = sc.tx.QueryRow(ctx, `SELECT actor=$4 AND request=$5::jsonb FROM worldbuilder.conversation_message WHERE world_id=$1 AND conversation_id=$2 AND id=$3`, sc.world.ID, path[1], b.ID, sc.actor, raw).Scan(&same)
			if err != nil {
				return nil, err
			}
			if !same {
				return nil, conflict("message ID reused")
			}
			return map[string]string{"id": b.ID, "status": "saved"}, nil
		}
		if r.Method == "GET" {
			rows, err := sc.tx.Query(ctx, `SELECT id::text,actor,sequence,request,result,CASE WHEN status='running' AND created_at<now()-interval '2 minutes' THEN 'uncertain' ELSE status END FROM worldbuilder.conversation_message WHERE world_id=$1 AND conversation_id=$2 ORDER BY sequence LIMIT $3 OFFSET $4`, sc.world.ID, path[1], limit, offset)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			result := []ConversationMessage{}
			for rows.Next() {
				var m ConversationMessage
				if err = rows.Scan(&m.ID, &m.Actor, &m.Sequence, &m.Request, &m.Result, &m.Status); err != nil {
					return nil, err
				}
				result = append(result, m)
			}
			return result, rows.Err()
		}
	}
	return nil, missing
}

// Claim is committed before inference. A lost process leaves an uncertain request,
// never an automatic second provider call. Only the requesting human can run it.
func (s *Store) runConversation(w http.ResponseWriter, r *http.Request, actor string, parts []string) {
	for _, id := range []string{parts[2], parts[4]} {
		if _, err := uuid.Parse(id); err != nil {
			writeError(w, missing)
			return
		}
	}
	var input ReasonInput
	var profile InferenceProfile
	var cached *ReasonResult
	var claimed bool
	_, err := s.inWorld(r.Context(), parts[0], actor, func(sc *scope) (any, error) {
		if sc.world.Archived {
			return nil, conflict("world is archived")
		}
		var owner, status string
		err := sc.tx.QueryRow(r.Context(), `SELECT actor,request,result,status,inference_profile FROM worldbuilder.conversation_message WHERE world_id=$1 AND conversation_id=$2 AND id=$3`, parts[0], parts[2], parts[4]).Scan(&owner, &input, &cached, &status, &profile)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, missing
		}
		if err != nil {
			return nil, err
		}
		if owner != actor {
			return nil, forbidden
		}
		if status == "complete" {
			return nil, nil
		}
		if status != "saved" {
			return nil, conflict("request already started; inspect its saved result, do not replay inference")
		}
		_, err = sc.tx.Exec(r.Context(), `UPDATE worldbuilder.conversation_message SET status='running' WHERE world_id=$1 AND conversation_id=$2 AND id=$3`, parts[0], parts[2], parts[4])
		claimed = err == nil
		return nil, err
	})
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	emit := func(event string, data any) {
		raw, _ := json.Marshal(data)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, raw)
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
	if !claimed {
		emit("result", cached)
		return
	}
	emit("progress", map[string]string{"status": "running", "message": "Checking scoped evidence and preparing a response"})
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 65*time.Second)
	defer cancel()
	result, err := s.inWorld(ctx, parts[0], actor, func(sc *scope) (any, error) {
		if sc.world.Archived {
			return nil, conflict("world is archived")
		}
		input.InferenceProfile = &profile
		value, err := sc.generate(ctx, input, s.Reasoner, s.Budget, nil)
		if err != nil {
			return nil, err
		}
		raw, _ := json.Marshal(value)
		_, err = sc.tx.Exec(ctx, `UPDATE worldbuilder.conversation_message SET status='complete',result=$4 WHERE world_id=$1 AND conversation_id=$2 AND id=$3`, parts[0], parts[2], parts[4], raw)
		return value, err
	})
	if err != nil {
		finish, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_, _ = s.Pool.Exec(finish, `UPDATE worldbuilder.conversation_message SET status='failed' WHERE world_id=$1 AND conversation_id=$2 AND id=$3 AND status='running'`, parts[0], parts[2], parts[4])
		emit("error", map[string]string{"error": "Generation did not complete. The request is retained; inspect it before starting a new attempt."})
		return
	}
	emit("result", result)
}
