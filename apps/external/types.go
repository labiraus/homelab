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
