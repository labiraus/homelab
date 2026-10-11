package worldbuilder

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"pkg/api"
	"strconv"
	"time"
)

const ToolsPath = "/api/worldbuilder-tools"
const ToolsAudience = "worldbuilder-mcp-v1"

type toolRequest struct {
	Operation    string      `json:"operation"`
	WorldID      string      `json:"worldId,omitempty"`
	Timeline     string      `json:"timeline,omitempty"`
	Revision     *int64      `json:"revision,omitempty"`
	At           *int64      `json:"at,omitempty"`
	Query        string      `json:"query,omitempty"`
	Kind         string      `json:"kind,omitempty"`
	RecordID     string      `json:"recordId,omitempty"`
	Limit        int64       `json:"limit,omitempty"`
	Offset       int64       `json:"offset,omitempty"`
	ID           string      `json:"id,omitempty"`
	BaseRevision int64       `json:"baseRevision,omitempty"`
	Reason       string      `json:"reason,omitempty"`
	Operations   []Operation `json:"operations,omitempty"`
}

// ToolsHandler deliberately has no path through human review/publication,
// administration, inference or private sessions, even for an owner actor.
func ToolsHandler(store *Store, key []byte, validator api.AuthValidator) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "POST" || r.URL.Path != ToolsPath || r.URL.RawQuery != "" {
			writeError(w, missing)
			return
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeError(w, bad("invalid tool request size"))
			return
		}
		actor, err := api.VerifyDelegation(key, r.Header.Get("X-Worldbuilder-Delegation"), ToolsAudience, r.Method, r.URL.RequestURI(), raw, time.Now())
		if err != nil || validator == nil {
			writeError(w, &Problem{401, "unauthenticated", "valid delegated identity required"})
			return
		}
		valid, _, err := validator(r.Context(), actor)
		if err != nil || !valid {
			writeError(w, &Problem{401, "unauthenticated", "delegated identity is not active"})
			return
		}
		var b toolRequest
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err = decoder.Decode(&b); err != nil {
			writeError(w, bad("invalid tool request"))
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			writeError(w, bad("invalid trailing data"))
			return
		}
		if b.Limit == 0 {
			b.Limit = 100
		}
		if b.Limit < 1 || b.Limit > 500 || b.Offset < 0 || b.Offset > 100000 {
			writeError(w, bad("invalid pagination"))
			return
		}
		var result any
		switch b.Operation {
		case "worlds":
			result, err = store.Worlds(r.Context(), actor, b.Limit, b.Offset)
		case "canon", "search", "record", "propose":
			result, err = store.inWorld(r.Context(), b.WorldID, actor, func(sc *scope) (any, error) {
				if b.Operation == "propose" {
					if sc.world.Archived {
						return nil, conflict("world is archived")
					}
					change, e := sc.propose(r.Context(), Change{ID: b.ID, BaseRevision: b.BaseRevision, Reason: b.Reason, Operations: b.Operations})
					if e != nil {
						return nil, e
					}
					return change, sc.audit(r.Context(), "mcp.propose", change.ID, map[string]any{"baseRevision": b.BaseRevision})
				}
				query := url.Values{"timeline": {b.Timeline}, "limit": {strconv.FormatInt(b.Limit, 10)}, "offset": {strconv.FormatInt(b.Offset, 10)}}
				if b.Revision != nil {
					query.Set("revision", strconv.FormatInt(*b.Revision, 10))
				}
				if b.At != nil {
					query.Set("at", strconv.FormatInt(*b.At, 10))
				}
				if b.Query != "" {
					query.Set("q", b.Query)
				}
				if b.Kind != "" {
					query.Set("kind", b.Kind)
				}
				path := []string{b.Operation}
				if b.Operation == "record" {
					path = []string{"records", b.RecordID}
				}
				// Reuse authorised canonical read validation without accepting a caller path.
				read := r.Clone(r.Context())
				read.Method = "GET"
				read.URL = &url.URL{RawQuery: query.Encode()}
				value, e := route(w, read, sc, path)
				if e != nil {
					return nil, e
				}
				return value, sc.audit(r.Context(), "mcp."+b.Operation, b.RecordID, map[string]any{"timeline": b.Timeline, "revision": b.Revision})
			})
		default:
			err = forbidden
		}
		if err != nil {
			writeError(w, err)
			return
		}
		_ = json.NewEncoder(w).Encode(result)
	})
}
