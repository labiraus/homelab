package main

import (
	"net/http"
	"pkg/base"
	"strings"
)

const oauthProtectedResourcePath = "/.well-known/oauth-protected-resource"

var capabilityCatalog = []manifestCapabilitySource{
	{
		ID:      "documents.ingestion",
		Title:   "Document Ingestion",
		Summary: "Queue documents for asynchronous ingestion through the orchestrator control plane.",
		Backend: manifestBackendOrchestrator,
		Operations: []manifestOperationSource{
			livePrompt(
				"documents.submit.example",
				"submitDocumentExample",
				"/prompts/documents/submit-example",
				"Show a ready-to-run example for queuing a document through the current ingestion entrypoint.",
				false,
				[]manifestPromptArgument{
					{Name: "documentId", Description: "Stable identifier for the document to submit.", Required: true},
					{Name: "sourceUri", Description: "Canonical source URI for the source document.", Required: true},
					{Name: "contentType", Description: "MIME type for the submitted document.", Required: true},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Use the live `documents.submit` tool on Labiraus to queue a document for ingestion. Prepare a tool call with a body like:\n\n{\n  \"documentId\": \"{{documentId}}\",\n  \"sourceUri\": \"{{sourceUri}}\",\n  \"contentType\": \"{{contentType}}\",\n  \"text\": \"<plain text payload>\",\n  \"bucket\": \"documents\",\n  \"objectKey\": \"incoming/{{documentId}}\"\n}\n\nIf the source is already in MinIO, keep `bucket` and `objectKey`; otherwise omit them and send plain text directly.",
						},
					},
				},
			),
			liveTool(
				"documents.submit",
				"submitDocument",
				http.MethodPost,
				"/documents/submit",
				"Submit a document payload to orchestrator for ingestion.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendOrchestrator,
					ExecutionMode: manifestExecutionModeHTTPProxy,
					Path:          "/documents",
				},
				documentSubmitSchema(),
			),
			plannedTool(
				"documents.scanBucket",
				"scanDocumentsBucket",
				http.MethodPost,
				"/documents/scan-bucket",
				"Scan the documents bucket and reconcile inventory into the document control plane.",
				false,
			),
			plannedPrompt(
				"documents.scanBucket.plan",
				"scanDocumentsBucketPlan",
				"/prompts/documents/scan-bucket-plan",
				"Describe the planned bucket-reconciliation flow that will scan MinIO and upsert document inventory into Postgres.",
				false,
				[]manifestPromptArgument{
					{Name: "prefix", Description: "Optional MinIO prefix to constrain the future scan scope."},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Plan the future `documents.scanBucket` workflow for the Labiraus MCP surface. The end state should scan the external MinIO documents bucket, optionally constrain reconciliation to the `{{prefix}}` prefix, upsert control-plane inventory into Postgres, and only then decide whether processor work should be queued.",
						},
					},
				},
			),
		},
	},
	{
		ID:      "postgres.auth",
		Title:   "Auth Database",
		Summary: "Read authentication and user inventory data directly from Postgres.",
		Backend: manifestBackendPostgres,
		Operations: []manifestOperationSource{
			livePrompt(
				"postgres.auth.userCount.prompt",
				"userCountPrompt",
				"/prompts/postgres/auth/user-count",
				"Summarize how to inspect the current auth-user count through the live Postgres-backed resource.",
				true,
				nil,
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Read the live `homelab://mcp/postgres/auth/users/count` resource to inspect the current auth-user count. Treat this as a fast control-plane health check for the Postgres-backed auth surface exposed by Labiraus.",
						},
					},
				},
			),
			liveResource(
				"postgres.auth.userCount",
				"userCount",
				http.MethodGet,
				"/postgres/auth/users/count",
				"Read the current user count from Postgres.",
				true,
				&manifestOperationBinding{
					Backend:       manifestBackendPostgres,
					ExecutionMode: manifestExecutionModePostgresQuery,
					Query:         "CALL auth.get_user_count(NULL)",
				},
			),
			plannedResourceTemplate(
				"postgres.auth.userByEmail",
				"userByEmail",
				http.MethodGet,
				"/postgres/auth/users/{email}",
				"Read a specific auth user by email from Postgres.",
				false,
				nil,
			),
		},
	},
	{
		ID:      "minio.documents",
		Title:   "Documents Bucket",
		Summary: "Browse and manage objects in the MinIO documents bucket.",
		Backend: manifestBackendMinIO,
		Operations: []manifestOperationSource{
			livePrompt(
				"minio.documents.browse.prompt",
				"browseDocumentObjectsPrompt",
				"/prompts/minio/documents/browse",
				"Show how to inspect a prefix in the documents bucket with the current live MinIO capability surface.",
				false,
				[]manifestPromptArgument{
					{Name: "prefix", Description: "Optional object prefix to inspect."},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Use the live `minio.documents.listFolder` tool to inspect the external documents bucket as folders and files. If you want to narrow the search, start with the `{{prefix}}` prefix, then follow up with `homelab://mcp/minio/documents/objects/{objectKey}` reads for the specific files you need to inspect or `minio.documents.putObject` to upload new content.",
						},
					},
				},
			),
			liveTool(
				"minio.documents.listFolder",
				"listDocumentFolder",
				http.MethodPost,
				"/minio/documents/list-folder",
				"List the folders and files immediately beneath a prefix in the documents bucket.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOListFolder,
					Bucket:        "documents",
				},
				minioListFolderSchema(),
			),
			liveTool(
				"minio.documents.listObjects",
				"listDocumentObjects",
				http.MethodPost,
				"/minio/documents/list-objects",
				"List objects in the documents bucket with an optional prefix.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOList,
					Bucket:        "documents",
				},
				minioListObjectsSchema(),
			),
			liveResourceTemplate(
				"minio.documents.object",
				"documentObject",
				http.MethodGet,
				"/minio/documents/objects/{objectKey}",
				"Read an object from the documents bucket.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOGetObject,
					Bucket:        "documents",
				},
			),
			liveTool(
				"minio.documents.putObject",
				"putDocumentObject",
				http.MethodPut,
				"/minio/documents/objects/{objectKey}",
				"Create or replace a binary or text object in the documents bucket from a base64 payload.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOPutObject,
					Bucket:        "documents",
				},
				minioPutObjectSchema(),
			),
			liveTool(
				"minio.documents.putTextObject",
				"putDocumentTextObject",
				http.MethodPut,
				"/minio/documents/objects/{objectKey}",
				"Create or replace a text object in the documents bucket.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOPutText,
					Bucket:        "documents",
				},
				minioPutTextObjectSchema(),
			),
			liveTool(
				"minio.documents.deleteObject",
				"deleteDocumentObject",
				http.MethodDelete,
				"/minio/documents/objects/{objectKey}",
				"Delete an object from the documents bucket.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIODelete,
					Bucket:        "documents",
				},
				nil,
			),
			plannedTool(
				"minio.documents.moveObject",
				"moveDocumentObject",
				http.MethodPost,
				"/minio/documents/move-object",
				"Move or rename an object in the documents bucket.",
				false,
			),
		},
	},
	{
		ID:      "documents.notifications",
		Title:   "Document Notifications",
		Summary: "Follow planned document lifecycle notifications that will be forwarded from NATS JetStream to MCP subscribers.",
		Backend: manifestBackendNATS,
		Operations: []manifestOperationSource{
			plannedPrompt(
				"documents.notifications.subscribe.prompt",
				"documentNotificationsPrompt",
				"/prompts/documents/notifications/subscribe",
				"Describe the planned NATS-backed subscription flow for document lifecycle notifications.",
				false,
				[]manifestPromptArgument{
					{Name: "documentId", Description: "Document identifier to follow through storage and processing.", Required: true},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Plan the future Labiraus subscription flow for `{{documentId}}`. The MCP server should subscribe to a NATS JetStream event stream, filter lifecycle events for the document, and forward matching updates to MCP subscribers in order: `documents.events.minio.stored`, `documents.events.processor.queued`, `documents.events.processor.started`, and `documents.events.processor.completed`.",
						},
					},
				},
			),
			plannedResourceTemplate(
				"documents.notifications.stream",
				"documentNotificationStream",
				http.MethodGet,
				"/documents/notifications/{documentId}",
				"Subscribe to future document lifecycle notifications for a specific document as NATS-backed updates are forwarded through MCP.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendNATS,
					ExecutionMode: manifestExecutionModeNATSSubscription,
				},
			),
		},
	},
}

func buildManifest(r *http.Request) manifestDocument {
	document := newManifestDocument(r)

	for _, capability := range capabilityCatalog {
		for _, operation := range capability.Operations {
			switch operation.Primitive {
			case manifestPrimitivePrompt:
				document.Prompts = append(document.Prompts, toManifestPrompt(capability, operation))
			case manifestPrimitiveResource:
				document.Resources = append(document.Resources, toManifestResource(capability, operation))
			case manifestPrimitiveResourceTemplate:
				document.ResourceTemplates = append(document.ResourceTemplates, toManifestResourceTemplate(capability, operation))
			case manifestPrimitiveTool:
				document.Tools = append(document.Tools, toManifestTool(capability, operation))
			}
		}
	}

	return document
}

func newManifestDocument(r *http.Request) manifestDocument {
	baseURL := requestBaseURL(r)

	return manifestDocument{
		Version: base.BuildVersion,
		Server: manifestServer{
			Name:    "labiraus",
			Version: base.BuildVersion,
		},
		Authorization: &manifestAuthorization{
			Type: "bearer",
			Meta: &manifestAuthorizationMeta{
				ResourceMetadataURL: baseURL + oauthProtectedResourcePath,
				AccessModes: []manifestAuthorizationAccessMode{
					{
						Type:        "bearer",
						Name:        "google-oidc",
						Description: "Browser and bearer-token capable clients can authenticate through the shared Google-backed OIDC flow.",
					},
					{
						Type:        "client-certificate",
						Name:        "mtls-client-cert",
						Description: "MCP clients can also authenticate with a trusted client certificate presented at the edge and forwarded as normalized certificate identity.",
					},
				},
				Requirement: "one-of",
			},
		},
		Transports: []manifestTransport{
			{
				Type: "streamable-http",
				URL:  baseURL + "/mcp",
			},
		},
		Prompts:           []manifestPrompt{},
		Resources:         []manifestResource{},
		ResourceTemplates: []manifestResourceTemplate{},
		Tools:             []manifestTool{},
	}
}

func requestBaseURL(r *http.Request) string {
	scheme := forwardedOrDefault(r.Header.Get("X-Forwarded-Proto"), "https")
	host := forwardedOrDefault(r.Header.Get("X-Forwarded-Host"), r.Host)

	return scheme + "://" + host
}

func forwardedOrDefault(value string, fallback string) string {
	if value == "" {
		return fallback
	}

	parts := strings.Split(value, ",")
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			return trimmed
		}
	}

	return fallback
}

func liveResource(id string, routeName string, method string, path string, summary string, public bool, binding *manifestOperationBinding) manifestOperationSource {
	return manifestOperationSource{
		ID:          id,
		Primitive:   manifestPrimitiveResource,
		RouteName:   routeName,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Public:      public,
		ContentMode: "json",
		Lifecycle:   manifestLifecycleLive,
		Binding:     binding,
	}
}

func liveResourceTemplate(id string, routeName string, method string, path string, summary string, public bool, binding *manifestOperationBinding) manifestOperationSource {
	return manifestOperationSource{
		ID:          id,
		Primitive:   manifestPrimitiveResourceTemplate,
		RouteName:   routeName,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Public:      public,
		ContentMode: "json",
		Lifecycle:   manifestLifecycleLive,
		Binding:     binding,
	}
}

func plannedResourceTemplate(id string, routeName string, method string, path string, summary string, public bool, binding *manifestOperationBinding) manifestOperationSource {
	return manifestOperationSource{
		ID:          id,
		Primitive:   manifestPrimitiveResourceTemplate,
		RouteName:   routeName,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Public:      public,
		ContentMode: "json",
		Lifecycle:   manifestLifecyclePlanned,
		Binding:     binding,
	}
}

func liveTool(id string, routeName string, method string, path string, summary string, public bool, binding *manifestOperationBinding, inputSchema *manifestSchema) manifestOperationSource {
	return manifestOperationSource{
		ID:          id,
		Primitive:   manifestPrimitiveTool,
		RouteName:   routeName,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Public:      public,
		ContentMode: "json",
		Lifecycle:   manifestLifecycleLive,
		InputSchema: inputSchema,
		Binding:     binding,
	}
}

func plannedTool(id string, routeName string, method string, path string, summary string, public bool) manifestOperationSource {
	return manifestOperationSource{
		ID:          id,
		Primitive:   manifestPrimitiveTool,
		RouteName:   routeName,
		Method:      method,
		Path:        path,
		Summary:     summary,
		Public:      public,
		ContentMode: "json",
		Lifecycle:   manifestLifecyclePlanned,
	}
}

func livePrompt(id string, routeName string, path string, summary string, public bool, arguments []manifestPromptArgument, messages []manifestPromptMessage) manifestOperationSource {
	return manifestOperationSource{
		ID:              id,
		Primitive:       manifestPrimitivePrompt,
		RouteName:       routeName,
		Path:            path,
		Summary:         summary,
		Public:          public,
		ContentMode:     "text/markdown",
		Lifecycle:       manifestLifecycleLive,
		PromptArguments: arguments,
		PromptMessages:  messages,
	}
}

func plannedPrompt(id string, routeName string, path string, summary string, public bool, arguments []manifestPromptArgument, messages []manifestPromptMessage) manifestOperationSource {
	return manifestOperationSource{
		ID:              id,
		Primitive:       manifestPrimitivePrompt,
		RouteName:       routeName,
		Path:            path,
		Summary:         summary,
		Public:          public,
		ContentMode:     "text/markdown",
		Lifecycle:       manifestLifecyclePlanned,
		PromptArguments: arguments,
		PromptMessages:  messages,
	}
}

func toManifestPrompt(capability manifestCapabilitySource, operation manifestOperationSource) manifestPrompt {
	return manifestPrompt{
		Name:        operation.ID,
		Title:       capability.Title,
		Description: operationDescription(capability, operation),
		Arguments:   operation.PromptArguments,
		Meta:        toManifestMeta(capability, operation),
	}
}

func toManifestResource(capability manifestCapabilitySource, operation manifestOperationSource) manifestResource {
	return manifestResource{
		Name:        operation.ID,
		Title:       capability.Title,
		URI:         toOperationURI(operation.Path),
		Description: operationDescription(capability, operation),
		MIMEType:    "application/json",
		Annotations: toManifestAnnotations(operation),
		Meta:        toManifestMeta(capability, operation),
	}
}

func toManifestResourceTemplate(capability manifestCapabilitySource, operation manifestOperationSource) manifestResourceTemplate {
	return manifestResourceTemplate{
		Name:        operation.ID,
		Title:       capability.Title,
		URITemplate: toOperationURI(operation.Path),
		Description: operationDescription(capability, operation),
		MIMEType:    "application/json",
		Annotations: toManifestAnnotations(operation),
		Meta:        toManifestMeta(capability, operation),
	}
}

func toManifestTool(capability manifestCapabilitySource, operation manifestOperationSource) manifestTool {
	return manifestTool{
		Name:        operation.ID,
		Title:       capability.Title,
		Description: operationDescription(capability, operation),
		InputSchema: toToolInputSchema(operation),
		Annotations: toManifestToolHints(capability, operation),
		Meta:        toManifestMeta(capability, operation),
	}
}

func toManifestMeta(capability manifestCapabilitySource, operation manifestOperationSource) manifestOperationMeta {
	meta := manifestOperationMeta{
		CapabilityID: capability.ID,
		RouteName:    operation.RouteName,
		Method:       operation.Method,
		Path:         operation.Path,
		Public:       operation.Public,
		ContentMode:  operation.ContentMode,
		Backend:      capability.Backend,
		Lifecycle:    operation.Lifecycle,
	}

	if operation.Binding != nil {
		meta.ExecutionMode = operation.Binding.ExecutionMode
	}

	return meta
}

func toManifestAnnotations(operation manifestOperationSource) *manifestAnnotations {
	if !operation.Public {
		return nil
	}

	return &manifestAnnotations{Priority: 0.9}
}

func toManifestToolHints(capability manifestCapabilitySource, operation manifestOperationSource) *manifestToolHints {
	return &manifestToolHints{
		Title:           capability.Title,
		ReadOnlyHint:    operation.Method == http.MethodGet,
		DestructiveHint: operation.Method == http.MethodDelete,
		IdempotentHint:  operation.Method == http.MethodPut || operation.Method == http.MethodDelete,
		OpenWorldHint:   false,
	}
}

func toToolInputSchema(operation manifestOperationSource) manifestSchema {
	if operation.InputSchema != nil {
		return *operation.InputSchema
	}

	properties := map[string]manifestSchema{}
	required := []string{}

	for _, parameter := range extractPathParameters(operation.Path) {
		properties[parameter] = manifestSchema{
			Type:        "string",
			Description: "HTTP path parameter.",
		}
		required = append(required, parameter)
	}

	if operation.Method == http.MethodPost || operation.Method == http.MethodPut {
		properties["body"] = manifestSchema{
			Type:                 "object",
			Description:          "JSON request body forwarded to the backing endpoint.",
			AdditionalProperties: true,
		}
	}

	return manifestSchema{
		Type:       "object",
		Properties: properties,
		Required:   required,
	}
}

func operationDescription(capability manifestCapabilitySource, operation manifestOperationSource) string {
	return capability.Summary + " " + operation.Summary
}

func toOperationURI(path string) string {
	return "homelab://mcp" + path
}

func extractPathParameters(path string) []string {
	parts := strings.Split(path, "/")
	parameters := []string{}

	for _, part := range parts {
		if len(part) < 3 {
			continue
		}

		if !strings.HasPrefix(part, "{") || !strings.HasSuffix(part, "}") {
			continue
		}

		parameters = append(parameters, strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"))
	}

	return parameters
}

func documentSubmitSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"body": {
				Type:        "object",
				Description: "Document ingestion payload.",
				Properties: map[string]manifestSchema{
					"documentId":        {Type: "string", Description: "Stable document identifier."},
					"bucket":            {Type: "string", Description: "Source bucket name when ingesting from object storage."},
					"objectKey":         {Type: "string", Description: "Source object key when ingesting from object storage."},
					"sourceUri":         {Type: "string", Description: "Canonical source URI such as s3://bucket/key."},
					"contentType":       {Type: "string", Description: "Document MIME type."},
					"versionMarker":     {Type: "string", Description: "Optional source version marker."},
					"etag":              {Type: "string", Description: "Optional source ETag."},
					"sizeBytes":         {Type: "integer", Description: "Optional source size in bytes."},
					"lastModified":      {Type: "string", Description: "Optional source last-modified timestamp."},
					"text":              {Type: "string", Description: "Plain text payload for the document."},
					"metadata":          {Type: "object", Description: "Optional document metadata.", AdditionalProperties: true},
					"processingVersion": {Type: "integer", Description: "Optional desired processing version."},
				},
				Required: []string{"documentId", "sourceUri", "contentType", "text"},
			},
		},
		Required: []string{"body"},
	}
}

func minioListObjectsSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"prefix":    {Type: "string", Description: "Optional object prefix filter."},
			"recursive": {Type: "boolean", Description: "Whether to traverse the prefix recursively."},
			"maxKeys":   {Type: "integer", Description: "Optional maximum number of objects to return."},
		},
	}
}

func minioListFolderSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"prefix":  {Type: "string", Description: "Optional folder prefix to inspect."},
			"maxKeys": {Type: "integer", Description: "Optional maximum number of immediate children to return."},
		},
	}
}

func minioPutObjectSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"objectKey": {Type: "string", Description: "Object key within the documents bucket."},
			"body": {
				Type:        "object",
				Description: "Object content and metadata.",
				Properties: map[string]manifestSchema{
					"base64":      {Type: "string", Description: "Base64-encoded object payload."},
					"contentType": {Type: "string", Description: "Optional MIME type for the stored object."},
				},
				Required: []string{"base64"},
			},
		},
		Required: []string{"objectKey", "body"},
	}
}

func minioPutTextObjectSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"objectKey": {Type: "string", Description: "Object key within the documents bucket."},
			"body": {
				Type:        "object",
				Description: "Object content and metadata.",
				Properties: map[string]manifestSchema{
					"text":        {Type: "string", Description: "Text body to store in MinIO."},
					"contentType": {Type: "string", Description: "Optional MIME type for the stored object."},
				},
				Required: []string{"text"},
			},
		},
		Required: []string{"objectKey", "body"},
	}
}
