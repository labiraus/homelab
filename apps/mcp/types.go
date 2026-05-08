package main

import "encoding/json"

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string        `json:"jsonrpc"`
	ID      any           `json:"id"`
	Result  any           `json:"result,omitempty"`
	Error   *jsonRPCError `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type initializeParams struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities,omitempty"`
	ClientInfo      map[string]any `json:"clientInfo,omitempty"`
}

type getPromptParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type readResourceParams struct {
	URI string `json:"uri"`
}

type subscribeResourceParams struct {
	URI string `json:"uri"`
}

type unsubscribeResourceParams struct {
	URI string `json:"uri"`
}

type callToolParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments,omitempty"`
}

type manifestPrimitiveKind string

const (
	manifestPrimitivePrompt           manifestPrimitiveKind = "prompt"
	manifestPrimitiveResource         manifestPrimitiveKind = "resource"
	manifestPrimitiveResourceTemplate manifestPrimitiveKind = "resourceTemplate"
	manifestPrimitiveTool             manifestPrimitiveKind = "tool"
)

type manifestLifecycle string

const (
	manifestLifecycleLive    manifestLifecycle = "live"
	manifestLifecyclePlanned manifestLifecycle = "planned"
)

type manifestBackend string

const (
	manifestBackendAuth         manifestBackend = "auth"
	manifestBackendMinIO        manifestBackend = "minio"
	manifestBackendNATS         manifestBackend = "nats"
	manifestBackendOrchestrator manifestBackend = "orchestrator"
	manifestBackendPostgres     manifestBackend = "postgres"
)

type manifestExecutionMode string

const (
	manifestExecutionModeDiscovery           manifestExecutionMode = "discovery"
	manifestExecutionModeHTTPProxy           manifestExecutionMode = "httpProxy"
	manifestExecutionModeMinIODelete         manifestExecutionMode = "minioDeleteObject"
	manifestExecutionModeMinIOListFolder     manifestExecutionMode = "minioListFolder"
	manifestExecutionModeMinIOGetObject      manifestExecutionMode = "minioGetObject"
	manifestExecutionModeMinIOList           manifestExecutionMode = "minioListObjects"
	manifestExecutionModeMinIOMove           manifestExecutionMode = "minioMoveObject"
	manifestExecutionModeMinIOPutObject      manifestExecutionMode = "minioPutObject"
	manifestExecutionModeMinIOPutText        manifestExecutionMode = "minioPutTextObject"
	manifestExecutionModeNATSSubscription    manifestExecutionMode = "natsSubscription"
	manifestExecutionModeDocumentInventory   manifestExecutionMode = "documentInventory"
	manifestExecutionModeDocumentSearch      manifestExecutionMode = "documentSearch"
	manifestExecutionModePostgresQuery       manifestExecutionMode = "postgresQuery"
	manifestExecutionModePostgresUserByEmail manifestExecutionMode = "postgresUserByEmail"
)

type manifestDocument struct {
	Version           string                     `json:"version"`
	Server            manifestServer             `json:"server"`
	Authorization     *manifestAuthorization     `json:"authorization,omitempty"`
	Transports        []manifestTransport        `json:"transports,omitempty"`
	Prompts           []manifestPrompt           `json:"prompts,omitempty"`
	Resources         []manifestResource         `json:"resources,omitempty"`
	ResourceTemplates []manifestResourceTemplate `json:"resourceTemplates,omitempty"`
	Tools             []manifestTool             `json:"tools,omitempty"`
}

type manifestServer struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type manifestAuthorization struct {
	Type string                     `json:"type"`
	Meta *manifestAuthorizationMeta `json:"_meta,omitempty"`
}

type manifestAuthorizationAccessMode struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

type manifestTransport struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

type manifestPrompt struct {
	Name        string                   `json:"name"`
	Title       string                   `json:"title,omitempty"`
	Description string                   `json:"description,omitempty"`
	Arguments   []manifestPromptArgument `json:"arguments,omitempty"`
	Meta        manifestOperationMeta    `json:"_meta"`
}

type manifestPromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

type manifestPromptMessage struct {
	Role    string                    `json:"role"`
	Content manifestPromptMessagePart `json:"content"`
}

type manifestPromptMessagePart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type manifestResource struct {
	Name        string                `json:"name"`
	Title       string                `json:"title,omitempty"`
	URI         string                `json:"uri"`
	Description string                `json:"description,omitempty"`
	MIMEType    string                `json:"mimeType,omitempty"`
	Annotations *manifestAnnotations  `json:"annotations,omitempty"`
	Meta        manifestOperationMeta `json:"_meta"`
}

type manifestResourceTemplate struct {
	Name        string                `json:"name"`
	Title       string                `json:"title,omitempty"`
	URITemplate string                `json:"uriTemplate"`
	Description string                `json:"description,omitempty"`
	MIMEType    string                `json:"mimeType,omitempty"`
	Annotations *manifestAnnotations  `json:"annotations,omitempty"`
	Meta        manifestOperationMeta `json:"_meta"`
}

type manifestTool struct {
	Name        string                `json:"name"`
	Title       string                `json:"title,omitempty"`
	Description string                `json:"description,omitempty"`
	InputSchema manifestSchema        `json:"inputSchema"`
	Annotations *manifestToolHints    `json:"annotations,omitempty"`
	Meta        manifestOperationMeta `json:"_meta"`
}

type manifestSchema struct {
	Type                 string                    `json:"type"`
	Properties           map[string]manifestSchema `json:"properties,omitempty"`
	Required             []string                  `json:"required,omitempty"`
	AdditionalProperties any                       `json:"additionalProperties,omitempty"`
	Description          string                    `json:"description,omitempty"`
}

type manifestAnnotations struct {
	Priority float64 `json:"priority,omitempty"`
}

type manifestToolHints struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint,omitempty"`
	DestructiveHint bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   bool   `json:"openWorldHint"`
}

type manifestAuthorizationMeta struct {
	ResourceMetadataURL string                            `json:"resourceMetadataUrl,omitempty"`
	AccessModes         []manifestAuthorizationAccessMode `json:"accessModes,omitempty"`
	Requirement         string                            `json:"requirement,omitempty"`
}

type manifestOperationMeta struct {
	CapabilityID  string                `json:"capabilityID"`
	RouteName     string                `json:"routeName,omitempty"`
	Method        string                `json:"method,omitempty"`
	Path          string                `json:"path"`
	Public        bool                  `json:"public"`
	ContentMode   string                `json:"contentMode"`
	Backend       manifestBackend       `json:"backend,omitempty"`
	Lifecycle     manifestLifecycle     `json:"lifecycle,omitempty"`
	ExecutionMode manifestExecutionMode `json:"executionMode,omitempty"`
}

type manifestOperationSource struct {
	ID              string
	Primitive       manifestPrimitiveKind
	RouteName       string
	Method          string
	Path            string
	Summary         string
	Public          bool
	ContentMode     string
	Lifecycle       manifestLifecycle
	InputSchema     *manifestSchema
	PromptArguments []manifestPromptArgument
	PromptMessages  []manifestPromptMessage
	Binding         *manifestOperationBinding
}

type manifestCapabilitySource struct {
	ID         string
	Title      string
	Summary    string
	Backend    manifestBackend
	Operations []manifestOperationSource
}

type manifestOperationBinding struct {
	Backend       manifestBackend
	ExecutionMode manifestExecutionMode
	Path          string
	Bucket        string
	Query         string
}
