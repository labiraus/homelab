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
