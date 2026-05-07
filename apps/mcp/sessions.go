package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

const (
	mcpSessionHeader         = "MCP-Session-Id"
	mcpProtocolVersionHeader = "MCP-Protocol-Version"
)

type mcpSession struct {
	ID              string
	ProtocolVersion string
	Subscriptions   map[string]struct{}
	Stream          *sessionStream
}

type sessionStream struct {
	id       uint64
	messages chan []byte
}

type mcpSessionManager struct {
	mu           sync.RWMutex
	sessions     map[string]*mcpSession
	nextStreamID uint64
}

func newSessionManager() *mcpSessionManager {
	return &mcpSessionManager{
		sessions: map[string]*mcpSession{},
	}
}

func (manager *mcpSessionManager) create(protocolVersion string) *mcpSession {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	session := &mcpSession{
		ID:              uuid.NewString(),
		ProtocolVersion: protocolVersion,
		Subscriptions:   map[string]struct{}{},
	}
	manager.sessions[session.ID] = session
	return session
}

func (manager *mcpSessionManager) get(id string) (*mcpSession, bool) {
	manager.mu.RLock()
	defer manager.mu.RUnlock()

	session, ok := manager.sessions[id]
	return session, ok
}

func (manager *mcpSessionManager) subscribe(sessionID string, uri string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	session, ok := manager.sessions[sessionID]
	if !ok {
		return false
	}
	session.Subscriptions[uri] = struct{}{}
	return true
}

func (manager *mcpSessionManager) unsubscribe(sessionID string, uri string) bool {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	session, ok := manager.sessions[sessionID]
	if !ok {
		return false
	}
	delete(session.Subscriptions, uri)
	return true
}

func (manager *mcpSessionManager) registerStream(sessionID string) (*sessionStream, bool) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	session, ok := manager.sessions[sessionID]
	if !ok {
		return nil, false
	}

	stream := &sessionStream{
		id:       atomic.AddUint64(&manager.nextStreamID, 1),
		messages: make(chan []byte, 64),
	}
	if session.Stream != nil {
		close(session.Stream.messages)
	}
	session.Stream = stream
	return stream, true
}

func (manager *mcpSessionManager) unregisterStream(sessionID string, stream *sessionStream) {
	manager.mu.Lock()
	defer manager.mu.Unlock()

	session, ok := manager.sessions[sessionID]
	if !ok || session.Stream == nil || stream == nil || session.Stream.id != stream.id {
		return
	}

	close(session.Stream.messages)
	session.Stream = nil
}

func (manager *mcpSessionManager) notifyResourceUpdated(documentID string) {
	uri := documentNotificationResourceURI(documentID)
	notification, err := json.Marshal(jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/resources/updated",
		Params: mustJSONRawMessage(map[string]any{
			"uri": uri,
		}),
	})
	if err != nil {
		return
	}

	manager.mu.RLock()
	defer manager.mu.RUnlock()

	for _, session := range manager.sessions {
		if session.Stream == nil {
			continue
		}
		if _, ok := session.Subscriptions[uri]; !ok {
			continue
		}

		select {
		case session.Stream.messages <- notification:
		default:
		}
	}
}

func sessionIDFromRequest(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(mcpSessionHeader))
}

func protocolVersionFromHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get(mcpProtocolVersionHeader))
}

func validateSessionRequest(r *http.Request) (string, *mcpSession, int, *jsonRPCResponse) {
	sessionID := sessionIDFromRequest(r)
	if sessionID == "" {
		return "", nil, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "MCP-Session-Id header is required",
			},
		}
	}

	session, ok := sessionRegistry.get(sessionID)
	if !ok {
		return "", nil, http.StatusNotFound, &jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "Unknown MCP session",
			},
		}
	}

	protocolVersion := protocolVersionFromHeader(r)
	if protocolVersion == "" {
		// Some Streamable HTTP clients preserve the session header but omit this
		// protocol header after initialize; use the session's negotiated version.
		return sessionID, session, 0, nil
	}
	if negotiateProtocolVersion(protocolVersion) != protocolVersion || protocolVersion != session.ProtocolVersion {
		return "", nil, http.StatusBadRequest, &jsonRPCResponse{
			JSONRPC: "2.0",
			Error: &jsonRPCError{
				Code:    -32600,
				Message: "Unsupported MCP protocol version",
			},
		}
	}

	return sessionID, session, 0, nil
}

func mustJSONRawMessage(value any) json.RawMessage {
	data, _ := json.Marshal(value)
	return data
}

func documentNotificationResourceURI(documentID string) string {
	return toOperationURI("/documents/notifications/" + documentID)
}

var sessionRegistry = newSessionManager()
