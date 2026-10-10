package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"pkg/api"
	"pkg/base"
	"strings"
	"time"
)

const worldbuilderMode manifestExecutionMode = "worldbuilderDelegated"

func worldbuilderEnabled() bool {
	return strings.EqualFold(os.Getenv("WORLDBUILDER_MCP_ENABLED"), "true")
}

func activeCapabilityCatalog() []manifestCapabilitySource {
	if !worldbuilderEnabled() {
		return capabilityCatalog
	}
	result := append([]manifestCapabilitySource{}, capabilityCatalog...)
	return append(result, worldbuilderCapability())
}

func worldbuilderCapability() manifestCapabilitySource {
	capability := manifestCapabilitySource{ID: "worldbuilder.canon", Title: "Worldbuilder canon", Summary: "Scoped canon reads and draft proposals; human review and publication remain separate.", Backend: manifestBackendPostgres}
	text := manifestSchema{Type: "string"}
	integer := manifestSchema{Type: "integer"}
	for _, operation := range []string{"worlds", "canon", "search", "record", "propose"} {
		properties := map[string]manifestSchema{}
		required := []string{}
		if operation != "worlds" {
			properties["worldId"] = text
			required = append(required, "worldId")
		}
		method := http.MethodGet
		description := "Read authorised Worldbuilder " + operation + ". Canon evidence is scoped to world, timeline and revision."
		if operation == "propose" {
			method = http.MethodPost
			description = "Create an idempotent draft change proposal at baseRevision. Does not submit, approve or publish. Human author must review and submit it."
			properties["id"] = manifestSchema{Type: "string", Description: "Stable UUID for exact retries."}
			properties["baseRevision"] = integer
			properties["reason"] = text
			properties["operations"] = manifestSchema{Type: "array", Description: "1–100 typed put/delete operations. Put replaces the complete record; facts/relationships/events require valid timeline and source references.", Items: &manifestSchema{Type: "object", Properties: map[string]manifestSchema{"action": text, "id": text, "record": {Type: "object", AdditionalProperties: true}}, Required: []string{"action", "id"}, AdditionalProperties: false}}
			required = append(required, "id", "baseRevision", "reason", "operations")
		} else {
			properties["limit"] = integer
			properties["offset"] = integer
			if operation != "worlds" {
				properties["timeline"] = text
				properties["revision"] = integer
				properties["at"] = integer
				required = append(required, "timeline")
			}
			if operation == "search" {
				properties["query"] = text
				required = append(required, "query")
			}
			if operation == "canon" {
				properties["kind"] = text
			}
			if operation == "record" {
				properties["recordId"] = text
				required = append(required, "recordId")
			}
		}
		capability.Operations = append(capability.Operations, liveTool("worldbuilder."+operation, "worldbuilder"+operation, method, "/worldbuilder/"+operation, description, false, &manifestOperationBinding{Backend: manifestBackendPostgres, ExecutionMode: worldbuilderMode, Path: operation}, &manifestSchema{Type: "object", Properties: properties, Required: required, AdditionalProperties: false}))
	}
	return capability
}

func proxyWorldbuilder(ctx context.Context, operation string, arguments map[string]any) (operationResponse, *jsonRPCError) {
	fail := func(message string) (operationResponse, *jsonRPCError) {
		return operationResponse{}, &jsonRPCError{Code: -32000, Message: message}
	}
	if !worldbuilderEnabled() {
		return fail("Worldbuilder MCP is disabled")
	}
	status, ok := api.AuthStatusFromContext(ctx)
	if !ok || !status.Valid || status.Email == "" || (status.Mode != api.AuthModeCertificate && status.Mode != api.AuthModeOIDC) {
		return fail("Authenticated MCP identity required")
	}
	request := map[string]any{}
	for k, v := range arguments {
		request[k] = v
	}
	request["operation"] = operation
	raw, err := json.Marshal(request)
	if err != nil || len(raw) > 1<<20 {
		return fail("Invalid Worldbuilder request")
	}
	const path = "/api/worldbuilder-tools"
	token, err := api.SignDelegation([]byte(os.Getenv("WORLDBUILDER_MCP_KEY")), status.Email, "worldbuilder-mcp-v1", "POST", path, raw, time.Now())
	if err != nil {
		return fail("Worldbuilder delegation is not configured")
	}
	endpoint := strings.TrimRight(base.GetEnv("API_BASE_URL", defaultAPIBaseURL), "/") + path
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(raw))
	if err != nil {
		return fail("Invalid Worldbuilder upstream")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worldbuilder-Delegation", token)
	client := &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(req)
	if err != nil {
		return fail("Worldbuilder upstream unavailable")
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil || len(body) > 2<<20 {
		return fail("Invalid Worldbuilder response size")
	}
	result := operationResponse{ContentType: "application/json", Body: string(body)}
	if response.StatusCode != http.StatusOK {
		return result, &jsonRPCError{Code: -32000, Message: "Worldbuilder request denied or invalid", Data: map[string]any{"status": response.StatusCode}}
	}
	return result, nil
}
