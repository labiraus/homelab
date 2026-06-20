package main

type HelloResponse struct {
	Data string `json:"data"`
}

type AuthStatusResponse struct {
	Mode          string `json:"mode"`
	Email         string `json:"email,omitempty"`
	Valid         bool   `json:"valid"`
	InvalidReason string `json:"invalidReason,omitempty"`
}

type AuthProvider struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	Issuer           string `json:"issuer"`
	AuthorizationURL string `json:"authorizationUrl,omitempty"`
	Configured       bool   `json:"configured"`
}

type AuthProvidersResponse struct {
	Providers []AuthProvider `json:"providers"`
}

type DocumentBreadcrumb struct {
	Name   string `json:"name"`
	Prefix string `json:"prefix"`
}

type DocumentEntry struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	ObjectKey    string `json:"objectKey,omitempty"`
	Prefix       string `json:"prefix,omitempty"`
	SizeBytes    int64  `json:"sizeBytes,omitempty"`
	LastModified string `json:"lastModified,omitempty"`
	ContentType  string `json:"contentType,omitempty"`
}

type DocumentTreeResponse struct {
	Bucket      string               `json:"bucket"`
	Prefix      string               `json:"prefix"`
	Breadcrumbs []DocumentBreadcrumb `json:"breadcrumbs"`
	Entries     []DocumentEntry      `json:"entries"`
}

type DocumentUploadResponse struct {
	ObjectKey   string `json:"objectKey"`
	SizeBytes   int64  `json:"sizeBytes"`
	ContentType string `json:"contentType,omitempty"`
}

type DocumentInventoryRequest struct {
	Status     string         `json:"status,omitempty"`
	Prefix     string         `json:"prefix,omitempty"`
	DocumentID string         `json:"documentId,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Limit      int            `json:"limit,omitempty"`
}

type DocumentInventoryRecord struct {
	DocumentID               string         `json:"documentId"`
	Bucket                   string         `json:"bucket,omitempty"`
	ObjectKey                string         `json:"objectKey,omitempty"`
	SourceURI                string         `json:"sourceUri,omitempty"`
	ContentType              string         `json:"contentType,omitempty"`
	Status                   string         `json:"status"`
	Metadata                 map[string]any `json:"metadata,omitempty"`
	DesiredProcessingVersion int            `json:"desiredProcessingVersion"`
	CurrentProcessingVersion int            `json:"currentProcessingVersion"`
	LastReconciledAt         string         `json:"lastReconciledAt,omitempty"`
	LastProcessedAt          string         `json:"lastProcessedAt,omitempty"`
	LastEventSubject         string         `json:"lastEventSubject,omitempty"`
	LastEventAt              string         `json:"lastEventAt,omitempty"`
	LastError                string         `json:"lastError,omitempty"`
}

type DocumentInventoryResponse struct {
	Documents []DocumentInventoryRecord `json:"documents"`
	Count     int                       `json:"count"`
}

type DocumentSearchRequest struct {
	Query      string         `json:"query"`
	Prefix     string         `json:"prefix,omitempty"`
	DocumentID string         `json:"documentId,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Limit      int            `json:"limit,omitempty"`
}

type DocumentContextRequest struct {
	Query      string         `json:"query"`
	Prefix     string         `json:"prefix,omitempty"`
	DocumentID string         `json:"documentId,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	MaxChars   int            `json:"maxChars,omitempty"`
}

type DocumentCitation struct {
	ID                string         `json:"id"`
	Label             string         `json:"label"`
	SourceURI         string         `json:"sourceUri,omitempty"`
	ObjectKey         string         `json:"objectKey,omitempty"`
	ChunkID           int64          `json:"chunkId"`
	ChunkIndex        int            `json:"chunkIndex"`
	ProcessingVersion int            `json:"processingVersion"`
	ChunkMetadata     map[string]any `json:"chunkMetadata,omitempty"`
}

type DocumentSearchHit struct {
	DocumentID        string            `json:"documentId"`
	SourceURI         string            `json:"sourceUri,omitempty"`
	ObjectKey         string            `json:"objectKey,omitempty"`
	ContentType       string            `json:"contentType,omitempty"`
	Metadata          map[string]any    `json:"metadata,omitempty"`
	ChunkID           int64             `json:"chunkId"`
	ChunkIndex        int               `json:"chunkIndex"`
	ChunkText         string            `json:"chunkText"`
	ProcessingVersion int               `json:"processingVersion"`
	Distance          float64           `json:"distance"`
	Similarity        float64           `json:"similarity"`
	LastProcessedAt   string            `json:"lastProcessedAt,omitempty"`
	ChunkMetadata     map[string]any    `json:"chunkMetadata,omitempty"`
	Citation          *DocumentCitation `json:"citation,omitempty"`
}

type DocumentSearchResponse struct {
	Query string              `json:"query"`
	Hits  []DocumentSearchHit `json:"hits"`
}

type DocumentContextCitation struct {
	Reference string            `json:"reference"`
	Citation  *DocumentCitation `json:"citation"`
}

type DocumentContextResponse struct {
	Query     string                    `json:"query"`
	Context   string                    `json:"context"`
	Citations []DocumentContextCitation `json:"citations"`
	Hits      []DocumentSearchHit       `json:"hits"`
	MaxChars  int                       `json:"maxChars"`
	Truncated bool                      `json:"truncated"`
}

type DocumentHistoryRequest struct {
	DocumentID        string `json:"documentId"`
	ProcessingVersion int    `json:"processingVersion,omitempty"`
	Limit             int    `json:"limit,omitempty"`
}

type DocumentLifecycleHistoryEvent struct {
	ID                int64          `json:"id"`
	DocumentID        string         `json:"documentId"`
	Subject           string         `json:"subject"`
	ProcessingVersion int            `json:"processingVersion"`
	OccurredAt        string         `json:"occurredAt"`
	CreatedAt         string         `json:"createdAt"`
	Payload           map[string]any `json:"payload"`
}

type DocumentHistoryResponse struct {
	DocumentID string                          `json:"documentId"`
	Events     []DocumentLifecycleHistoryEvent `json:"events"`
	Count      int                             `json:"count"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type UserRequest struct {
	UserID int `json:"userid"`
}

type UserResponse struct {
	UserID   int    `json:"userid"`
	Username string `json:"username"`
	Email    string `json:"email"`
}
