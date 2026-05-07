package main

import "net/http"

const mcpOptionsHandlerName = "OPTIONS /mcp"
const legacySSEOptionsHandlerName = "OPTIONS /sse"
const legacyMessagesOptionsHandlerName = "OPTIONS /messages"
const legacyMessageOptionsHandlerName = "OPTIONS /message"

func mcpOptionsAPI(w http.ResponseWriter, r *http.Request) {
	writeCORSPreflight(w, r, []string{http.MethodPost, http.MethodGet, http.MethodDelete, http.MethodOptions})
}

func legacyMCPOptionsAPI(w http.ResponseWriter, r *http.Request) {
	writeCORSPreflight(w, r, []string{http.MethodPost, http.MethodGet, http.MethodOptions})
}
