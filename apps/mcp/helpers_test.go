package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func httptestJSONRequest(t *testing.T, method string, path string, body string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	return request, httptest.NewRecorder()
}
