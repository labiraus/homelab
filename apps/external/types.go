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

type DocumentSearchRequest struct {
	Query      string `json:"query"`
	Prefix     string `json:"prefix,omitempty"`
	DocumentID string `json:"documentId,omitempty"`
	Limit      int    `json:"limit,omitempty"`
}

type DocumentContextRequest struct {
	Query      string `json:"query"`
	Prefix     string `json:"prefix,omitempty"`
	DocumentID string `json:"documentId,omitempty"`
	Limit      int    `json:"limit,omitempty"`
	MaxChars   int    `json:"maxChars,omitempty"`
}

type DocumentCitation struct {
	ID                string `json:"id"`
	Label             string `json:"label"`
	SourceURI         string `json:"sourceUri,omitempty"`
	ObjectKey         string `json:"objectKey,omitempty"`
	ChunkID           int64  `json:"chunkId"`
	ChunkIndex        int    `json:"chunkIndex"`
	ProcessingVersion int    `json:"processingVersion"`
}

type DocumentSearchHit struct {
	DocumentID        string            `json:"documentId"`
	SourceURI         string            `json:"sourceUri,omitempty"`
	ObjectKey         string            `json:"objectKey,omitempty"`
	ContentType       string            `json:"contentType,omitempty"`
	ChunkID           int64             `json:"chunkId"`
	ChunkIndex        int               `json:"chunkIndex"`
	ChunkText         string            `json:"chunkText"`
	ProcessingVersion int               `json:"processingVersion"`
	Distance          float64           `json:"distance"`
	Similarity        float64           `json:"similarity"`
	LastProcessedAt   string            `json:"lastProcessedAt,omitempty"`
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
