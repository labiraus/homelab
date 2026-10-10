package worldbuilder

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Identity func(*http.Request) (string, bool)

func Handler(store *Store, identity Identity) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		actor, ok := identity(r)
		if !ok || actor == "" {
			writeError(w, &Problem{401, "unauthenticated", "valid authenticated identity required"})
			return
		}
		if r.Method != "GET" {
			if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
				writeError(w, forbidden)
				return
			}
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host {
					writeError(w, forbidden)
					return
				}
			}
		}
		actor = strings.ToLower(strings.TrimSpace(actor))
		parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/worlds"), "/"), "/")
		var result any
		var err error
		if parts[0] == "" {
			switch r.Method {
			case "GET":
				var limit, offset int64
				limit, offset, err = page(r)
				if err == nil {
					result, err = store.Worlds(r.Context(), actor, limit, offset)
				}
			case "POST":
				var body struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				}
				if err = decode(w, r, &body); err == nil {
					result, err = store.CreateWorld(r.Context(), actor, body.ID, body.Name)
				}
			default:
				err = &Problem{405, "method", "unsupported method"}
			}
		} else {
			result, err = store.inWorld(r.Context(), parts[0], actor, func(sc *scope) (any, error) {
				if len(parts) == 2 && parts[1] == "reason" && r.Method == "POST" {
					return sc.reason(w, r, store.Reasoner)
				}
				return route(w, r, sc, parts[1:])
			})
		}
		if err != nil {
			writeError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
func route(w http.ResponseWriter, r *http.Request, sc *scope, path []string) (any, error) {
	ctx := r.Context()
	limit, offset, pageErr := page(r)
	if pageErr != nil {
		return nil, pageErr
	}
	resource := strings.Join(path, "/")
	if resource == "" && r.Method == "GET" {
		return sc.world, nil
	}
	if resource == "revisions" && r.Method == "GET" {
		return sc.revisions(ctx, limit, offset)
	}
	if resource == "diff" && r.Method == "GET" {
		from, err := integerQuery(r, "from", -1)
		if err != nil || from < 0 {
			return nil, bad("nonnegative from revision is required")
		}
		to, err := integerQuery(r, "to", sc.world.Revision)
		if err != nil || to < 0 {
			return nil, bad("nonnegative to revision is required")
		}
		before, err := sc.snapshot(ctx, from)
		if err != nil {
			return nil, err
		}
		after, err := sc.snapshot(ctx, to)
		if err != nil {
			return nil, err
		}
		diff := Diff(before, after)
		total := int64(len(diff))
		start := min(offset, total)
		end := start + min(limit, total-start)
		return map[string]any{"fromRevision": from, "toRevision": to, "records": diff[start:end], "total": total}, nil
	}
	if resource == "restore" && r.Method == "POST" {
		var b RestoreRequest
		if err := decode(w, r, &b); err != nil {
			return nil, err
		}
		return sc.restore(ctx, b)
	}
	if len(path) == 3 && (path[0] == "changes" || path[0] == "sessions") {
		if path[2] == "reconciliation" && r.Method == "GET" {
			return sc.reconcilePreview(ctx, path[0], path[1])
		}
		if ((path[0] == "changes" && path[2] == "rebase") || (path[0] == "sessions" && path[2] == "reconcile")) && r.Method == "POST" {
			var b ReconcileRequest
			if err := decode(w, r, &b); err != nil {
				return nil, err
			}
			return sc.reconcile(ctx, path[0], path[1], b)
		}
		if path[0] == "changes" && path[2] == "diff" && r.Method == "GET" {
			c, err := sc.change(ctx, path[1])
			if err != nil {
				return nil, err
			}
			before, err := sc.snapshot(ctx, c.BaseRevision)
			if err != nil {
				return nil, err
			}
			after, err := Apply(before, c.Operations)
			if err != nil {
				return nil, err
			}
			return map[string]any{"baseRevision": c.BaseRevision, "records": Diff(before, after)}, nil
		}
	}
	if resource == "members" && r.Method == "POST" {
		var b struct {
			Principal string `json:"principal"`
			Role      string `json:"role"`
		}
		if err := decode(w, r, &b); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, sc.member(ctx, b.Principal, b.Role)
	}
	if (resource == "canon" || (len(path) == 2 && path[0] == "records")) && r.Method == "GET" {
		rev, err := integerQuery(r, "revision", sc.world.Revision)
		if err != nil {
			return nil, err
		}
		timeline := r.URL.Query().Get("timeline")
		if timeline == "" {
			return nil, bad("timeline is required")
		}
		var at *int64
		if r.URL.Query().Has("at") {
			n, e := integerQuery(r, "at", 0)
			if e != nil {
				return nil, e
			}
			at = &n
		}
		snapshot, err := sc.snapshot(ctx, rev)
		if err != nil {
			return nil, err
		}
		if snapshot[timeline].Kind != "timeline" {
			return nil, bad("unknown timeline at requested revision")
		}
		records := Records(snapshot, timeline, r.URL.Query().Get("q"), at)
		if resource == "canon" && r.URL.Query().Has("kind") {
			kinds := map[string]bool{}
			for _, kind := range strings.Split(r.URL.Query().Get("kind"), ",") {
				switch kind {
				case "type", "entity", "source", "timeline", "fact", "relationship", "event", "rule":
					kinds[kind] = true
				default:
					return nil, bad("unsupported record kind filter")
				}
			}
			filtered := []Record{}
			for _, record := range records {
				if kinds[record.Kind] {
					filtered = append(filtered, record)
				}
			}
			records = filtered
		}
		if len(path) == 2 && path[0] == "records" {
			record, ok := snapshot[path[1]]
			if !ok || !Visible(record, timeline, at) {
				return nil, missing
			}
			return Citations(sc.world.ID, rev, timeline, []Record{record})[0], nil
		}
		offset, err := integerQuery(r, "offset", 0)
		if err != nil || offset < 0 {
			return nil, bad("invalid offset")
		}
		limit, err := integerQuery(r, "limit", 100)
		if err != nil || limit < 1 || limit > 500 {
			return nil, bad("limit must be 1–500")
		}
		total := int64(len(records))
		if offset > total {
			offset = total
		}
		end := offset + limit
		if end > total {
			end = total
		}
		records = records[offset:end]
		return map[string]any{"world": sc.world, "revision": rev, "timeline": timeline, "records": records, "citations": Citations(sc.world.ID, rev, timeline, records), "total": total}, nil
	}
	if resource == "changes" {
		if r.Method == "POST" {
			var b struct {
				ID           string      `json:"id"`
				BaseRevision int64       `json:"baseRevision"`
				Reason       string      `json:"reason"`
				Operations   []Operation `json:"operations"`
			}
			if err := decode(w, r, &b); err != nil {
				return nil, err
			}
			return sc.propose(ctx, Change{ID: b.ID, BaseRevision: b.BaseRevision, Reason: b.Reason, Operations: b.Operations})
		}
		if r.Method == "GET" {
			rows, err := sc.tx.Query(ctx, `SELECT id::text FROM worldbuilder.change_set WHERE world_id=$1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`, sc.world.ID, limit, offset)
			if err != nil {
				return nil, err
			}
			ids := []string{}
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return nil, err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				return nil, err
			}
			result := []Change{}
			for _, id := range ids {
				c, e := sc.change(ctx, id)
				if e != nil {
					return nil, e
				}
				result = append(result, c)
			}
			return result, nil
		}
	}
	if len(path) >= 2 && path[0] == "changes" {
		if len(path) == 2 && r.Method == "GET" {
			return sc.change(ctx, path[1])
		}
		if len(path) == 3 && r.Method == "POST" {
			switch path[2] {
			case "submit":
				if err := emptyBody(w, r); err != nil {
					return nil, err
				}
				return sc.submit(ctx, path[1])
			case "publish":
				if err := emptyBody(w, r); err != nil {
					return nil, err
				}
				return sc.publish(ctx, path[1])
			case "review":
				var b struct {
					Decision string `json:"decision"`
					Reason   string `json:"reason"`
				}
				if err := decode(w, r, &b); err != nil {
					return nil, err
				}
				return sc.review(ctx, path[1], b.Decision, b.Reason)
			}
		}
	}
	if resource == "sessions" {
		if r.Method == "POST" {
			var b struct {
				ID             string `json:"id"`
				Title          string `json:"title"`
				PinnedRevision int64  `json:"pinnedRevision"`
				Timeline       string `json:"timeline"`
			}
			if err := decode(w, r, &b); err != nil {
				return nil, err
			}
			return sc.createSession(ctx, Session{ID: b.ID, Title: b.Title, PinnedRevision: b.PinnedRevision, Timeline: b.Timeline})
		}
		if r.Method == "GET" {
			rows, err := sc.tx.Query(ctx, `SELECT id::text FROM worldbuilder.story_session WHERE world_id=$1 AND owner=$2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`, sc.world.ID, sc.actor, limit, offset)
			if err != nil {
				return nil, err
			}
			ids := []string{}
			for rows.Next() {
				var id string
				if err = rows.Scan(&id); err != nil {
					rows.Close()
					return nil, err
				}
				ids = append(ids, id)
			}
			rows.Close()
			if err = rows.Err(); err != nil {
				return nil, err
			}
			result := []Session{}
			for _, id := range ids {
				s, e := sc.session(ctx, id)
				if e != nil {
					return nil, e
				}
				result = append(result, s)
			}
			return result, nil
		}
	}
	if len(path) >= 2 && path[0] == "sessions" {
		if len(path) == 2 && r.Method == "GET" {
			return sc.session(ctx, path[1])
		}
		if len(path) == 3 && path[2] == "turns" && r.Method == "GET" {
			if _, err := sc.session(ctx, path[1]); err != nil {
				return nil, err
			}
			rows, err := sc.tx.Query(ctx, `SELECT version,narration,state FROM worldbuilder.story_turn WHERE world_id=$1 AND session_id=$2 ORDER BY version DESC LIMIT $3 OFFSET $4`, sc.world.ID, path[1], limit, offset)
			if err != nil {
				return nil, err
			}
			defer rows.Close()
			turns := []any{}
			for rows.Next() {
				var version int64
				var narration string
				var state []byte
				if err = rows.Scan(&version, &narration, &state); err != nil {
					return nil, err
				}
				turns = append(turns, map[string]any{"version": version, "narration": narration, "state": json.RawMessage(state)})
			}
			return turns, rows.Err()
		}
		if len(path) == 3 && r.Method == "POST" {
			switch path[2] {
			case "turns":
				var b Turn
				if err := decode(w, r, &b); err != nil {
					return nil, err
				}
				return sc.turn(ctx, path[1], b)
			case "outcomes":
				var b Outcome
				if err := decode(w, r, &b); err != nil {
					return nil, err
				}
				return sc.outcome(ctx, path[1], b)
			}
		}
	}
	return nil, missing
}
func integerQuery(r *http.Request, key string, fallback int64) (int64, error) {
	if !r.URL.Query().Has(key) {
		return fallback, nil
	}
	n, err := strconv.ParseInt(r.URL.Query().Get(key), 10, 64)
	if err != nil {
		return 0, bad("invalid " + key)
	}
	return n, nil
}
func decode(w http.ResponseWriter, r *http.Request, v any) error {
	r.Body = http.MaxBytesReader(w, r.Body, 256<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return bad("invalid JSON or unknown field")
	}
	if err := d.Decode(&struct{}{}); err != io.EOF {
		return bad("expected exactly one JSON object")
	}
	return nil
}
func emptyBody(w http.ResponseWriter, r *http.Request) error { var b struct{}; return decode(w, r, &b) }
func writeError(w http.ResponseWriter, err error) {
	p := &Problem{500, "internal", "request failed"}
	if !errors.As(err, &p) {
		slog.Error("worldbuilder request failed", "error", err)
		p = &Problem{500, "internal", "request failed"}
	}
	w.WriteHeader(p.Status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": p.Message, "code": p.Code})
}

func page(r *http.Request) (int64, int64, error) {
	limit, err := integerQuery(r, "limit", 100)
	if err != nil || limit < 1 || limit > 500 {
		return 0, 0, bad("limit must be 1–500")
	}
	offset, err := integerQuery(r, "offset", 0)
	if err != nil || offset < 0 {
		return 0, 0, bad("offset must be nonnegative")
	}
	return limit, offset, nil
}
