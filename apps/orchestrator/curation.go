package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type documentCurationRequest struct {
	DocumentID string                 `json:"documentId"`
	Metadata   map[string]interface{} `json:"metadata"`
	Replace    bool                   `json:"replace,omitempty"`
}

type documentCurationResponse struct {
	Status     string                 `json:"status"`
	DocumentID string                 `json:"documentId"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func documentCurationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var request documentCurationRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid request body"})
		return
	}

	request.DocumentID = strings.TrimSpace(request.DocumentID)
	if request.DocumentID == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "documentId is required"})
		return
	}
	if request.Metadata == nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "metadata is required"})
		return
	}

	metadata, found, err := updateCurationRecord(r.Context(), request.DocumentID, request.Metadata, request.Replace)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to update document metadata"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, errorResponse{Error: "document not found"})
		return
	}

	writeJSON(w, http.StatusOK, documentCurationResponse{
		Status:     "updated",
		DocumentID: request.DocumentID,
		Metadata:   metadata,
	})
}
