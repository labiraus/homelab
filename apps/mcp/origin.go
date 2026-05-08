package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"strings"
)

const mcpAllowedOriginsEnv = "MCP_ALLOWED_ORIGINS"

func prepareOriginResponse(w http.ResponseWriter, r *http.Request, methods []string) (int, *jsonRPCResponse) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		applyCORSHeaders(w, r, methods)
		return 0, nil
	}

	originURL, err := url.Parse(origin)
	if err != nil || originURL.Scheme == "" || originURL.Host == "" {
		return http.StatusForbidden, originValidationError()
	}

	allowedOrigins := allowedOrigins(r)
	for _, allowedOrigin := range allowedOrigins {
		if sameOrigin(originURL, allowedOrigin) {
			applyCORSHeaders(w, r, methods)
			return 0, nil
		}
	}

	return http.StatusForbidden, originValidationError()
}

func writeCORSPreflight(w http.ResponseWriter, r *http.Request, methods []string) {
	status, response := prepareOriginResponse(w, r, methods)
	if response != nil {
		writeJSONRPC(w, status, response)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func applyCORSHeaders(w http.ResponseWriter, r *http.Request, methods []string) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))
	w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
		"Accept",
		"Authorization",
		"Content-Type",
		"Last-Event-ID",
		"MCP-Protocol-Version",
		"MCP-Session-Id",
	}, ", "))
	w.Header().Set("Access-Control-Expose-Headers", strings.Join([]string{
		"MCP-Protocol-Version",
		"MCP-Session-Id",
		"WWW-Authenticate",
	}, ", "))
	w.Header().Set("Access-Control-Max-Age", "3600")
	w.Header().Add("Vary", "Origin")
}

func writeJSONRPC(w http.ResponseWriter, status int, response *jsonRPCResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

func allowedOrigins(r *http.Request) []*url.URL {
	configured := strings.TrimSpace(os.Getenv(mcpAllowedOriginsEnv))
	requestOrigin, _ := url.Parse(requestBaseURL(r))
	if configured == "" {
		return []*url.URL{requestOrigin}
	}

	origins := []*url.URL{}
	for _, value := range strings.Split(configured, ",") {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			continue
		}
		origins = append(origins, parsed)
	}
	if len(origins) == 0 {
		return []*url.URL{requestOrigin}
	}
	return origins
}

func sameOrigin(left *url.URL, right *url.URL) bool {
	if left == nil || right == nil {
		return false
	}

	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		originPort(left) == originPort(right)
}

func originPort(origin *url.URL) string {
	if origin == nil {
		return ""
	}
	if port := origin.Port(); port != "" {
		return port
	}
	switch strings.ToLower(origin.Scheme) {
	case "http":
		return "80"
	case "https":
		return "443"
	default:
		return ""
	}
}

func originValidationError() *jsonRPCResponse {
	return &jsonRPCResponse{
		JSONRPC: "2.0",
		Error: &jsonRPCError{
			Code:    -32000,
			Message: "Origin is not allowed",
		},
	}
}
