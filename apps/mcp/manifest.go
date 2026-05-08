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
							Text: "Use the live `documents.submit` tool on Labiraus to queue a document reference for ingestion. Prepare a tool call with a body like:\n\n{\n  \"documentId\": \"{{documentId}}\",\n  \"bucket\": \"documents\",\n  \"objectKey\": \"incoming/{{documentId}}.txt\",\n  \"sourceUri\": \"{{sourceUri}}\",\n  \"contentType\": \"{{contentType}}\"\n}\n\nThis ingestion path is reference-based and currently supports MinIO-backed `text/*` documents only.",
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
			liveTool(
				"documents.scanBucket",
				"scanDocumentsBucket",
				http.MethodPost,
				"/documents/scan-bucket",
				"Scan the documents bucket and reconcile inventory into the document control plane.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendOrchestrator,
					ExecutionMode: manifestExecutionModeHTTPProxy,
					Path:          "/documents/scan-bucket",
				},
				documentScanBucketSchema(),
			),
			liveTool(
				"documents.reprocess",
				"reprocessDocument",
				http.MethodPost,
				"/documents/reprocess",
				"Queue an existing inventory document for a newer processing version.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendOrchestrator,
					ExecutionMode: manifestExecutionModeHTTPProxy,
					Path:          "/documents/reprocess",
				},
				documentReprocessSchema(),
			),
			livePrompt(
				"documents.scanBucket.plan",
				"scanDocumentsBucketPlan",
				"/prompts/documents/scan-bucket-plan",
				"Describe the bucket-reconciliation flow that scans MinIO and upserts document inventory into Postgres.",
				false,
				[]manifestPromptArgument{
					{Name: "prefix", Description: "Optional MinIO prefix to constrain the scan scope."},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Use the live `documents.scanBucket` tool on Labiraus to reconcile the external MinIO documents bucket. Optionally constrain reconciliation to the `{{prefix}}` prefix. The scan upserts control-plane inventory into Postgres, marks non-text objects as unsupported, and queues new or changed `text/*` objects for processor work.",
						},
					},
				},
			),
		},
	},
	{
		ID:      "documents.curation",
		Title:   "Document Curation",
		Summary: "Curate document inventory metadata through the orchestrator control plane.",
		Backend: manifestBackendOrchestrator,
		Operations: []manifestOperationSource{
			liveTool(
				"documents.curation.update",
				"updateDocumentCuration",
				http.MethodPost,
				"/documents/curation",
				"Update curated metadata for an existing inventory document.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendOrchestrator,
					ExecutionMode: manifestExecutionModeHTTPProxy,
					Path:          "/documents/curation",
				},
				documentCurationSchema(),
			),
		},
	},
	{
		ID:      "documents.retrieval",
		Title:   "Document Retrieval",
		Summary: "Inspect indexed document state and search processed chunks from Postgres-backed retrieval data.",
		Backend: manifestBackendPostgres,
		Operations: []manifestOperationSource{
			livePrompt(
				"documents.search.prompt",
				"searchDocumentsPrompt",
				"/prompts/documents/search",
				"Show how to search processed document chunks through the live MCP retrieval tool.",
				false,
				[]manifestPromptArgument{
					{Name: "query", Description: "Natural-language retrieval query.", Required: true},
					{Name: "prefix", Description: "Optional documents-bucket object-key prefix."},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Use the live `documents.search` tool on Labiraus with query `{{query}}`. If a prefix is useful, set `prefix` to `{{prefix}}`. Results come from pgvector similarity search over processed chunks and include document IDs, object keys, chunk text, scores, processing versions, and citation objects that identify the source URI plus chunk.",
						},
					},
				},
			),
			liveTool(
				"documents.inventory.list",
				"listDocumentInventory",
				http.MethodPost,
				"/documents/inventory/list",
				"List document inventory and processing state from Postgres.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendPostgres,
					ExecutionMode: manifestExecutionModeDocumentInventory,
				},
				documentInventorySchema(),
			),
			liveTool(
				"documents.search",
				"searchDocuments",
				http.MethodPost,
				"/documents/search",
				"Search processed document chunks by embedding a query and ranking pgvector matches.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendPostgres,
					ExecutionMode: manifestExecutionModeDocumentSearch,
				},
				documentSearchSchema(),
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
			liveResourceTemplate(
				"postgres.auth.userByEmail",
				"userByEmail",
				http.MethodGet,
				"/postgres/auth/users/{email}",
				"Read a specific auth user by email from Postgres.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendPostgres,
					ExecutionMode: manifestExecutionModePostgresUserByEmail,
				},
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
			liveTool(
				"minio.documents.moveObject",
				"moveDocumentObject",
				http.MethodPost,
				"/minio/documents/move-object",
				"Move or rename an object in the documents bucket.",
				false,
				&manifestOperationBinding{
					Backend:       manifestBackendMinIO,
					ExecutionMode: manifestExecutionModeMinIOMove,
					Bucket:        "documents",
				},
				minioMoveObjectSchema(),
			),
		},
	},
	{
		ID:      "documents.notifications",
		Title:   "Document Notifications",
		Summary: "Follow live document lifecycle notifications that are forwarded from NATS JetStream to MCP subscribers.",
		Backend: manifestBackendNATS,
		Operations: []manifestOperationSource{
			livePrompt(
				"documents.notifications.subscribe.prompt",
				"documentNotificationsPrompt",
				"/prompts/documents/notifications/subscribe",
				"Describe the live NATS-backed subscription flow for document lifecycle notifications.",
				false,
				[]manifestPromptArgument{
					{Name: "documentId", Description: "Document identifier to follow through storage and processing.", Required: true},
				},
				[]manifestPromptMessage{
					{
						Role: "user",
						Content: manifestPromptMessagePart{
							Type: "text",
							Text: "Describe the live Labiraus subscription flow for `{{documentId}}`. The MCP server exposes `homelab://mcp/documents/notifications/{{documentId}}`, accepts `resources/subscribe` and `resources/unsubscribe`, maintains a Streamable HTTP session with `MCP-Session-Id`, and forwards matching NATS lifecycle updates over `GET /mcp` SSE. The current event sequence is `documents.events.processor.queued`, `documents.events.processor.started`, `documents.events.processor.completed`, and `documents.events.processor.failed`, with `documents.events.minio.stored` reserved for a future ingest-boundary emitter.",
						},
					},
				},
			),
			liveResourceTemplate(
				"documents.notifications.stream",
				"documentNotificationStream",
				http.MethodGet,
				"/documents/notifications/{documentId}",
				"Subscribe to document lifecycle notifications for a specific document as NATS-backed updates are forwarded through MCP.",
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
			{
				Type: "sse",
				URL:  baseURL + "/sse",
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
		ReadOnlyHint:    operationIsReadOnly(operation),
		DestructiveHint: operationIsDestructive(operation),
		IdempotentHint:  operationIsIdempotent(operation),
		OpenWorldHint:   false,
	}
}

func operationIsReadOnly(operation manifestOperationSource) bool {
	if operation.Method == http.MethodGet {
		return true
	}
	if operation.Binding == nil {
		return false
	}

	switch operation.Binding.ExecutionMode {
	case manifestExecutionModeDiscovery,
		manifestExecutionModeMinIOListFolder,
		manifestExecutionModeMinIOList,
		manifestExecutionModePostgresQuery:
		return true
	default:
		return false
	}
}

func operationIsDestructive(operation manifestOperationSource) bool {
	if operation.Method == http.MethodDelete {
		return true
	}
	if operation.Binding == nil {
		return false
	}

	switch operation.Binding.ExecutionMode {
	case manifestExecutionModeMinIODelete,
		manifestExecutionModeMinIOPutObject,
		manifestExecutionModeMinIOPutText:
		return true
	default:
		return false
	}
}

func operationIsIdempotent(operation manifestOperationSource) bool {
	if operation.Method == http.MethodGet || operation.Method == http.MethodPut || operation.Method == http.MethodDelete {
		return true
	}
	if operation.Binding == nil {
		return false
	}

	switch operation.Binding.ExecutionMode {
	case manifestExecutionModeDiscovery,
		manifestExecutionModeMinIOListFolder,
		manifestExecutionModeMinIOList,
		manifestExecutionModePostgresQuery:
		return true
	default:
		return false
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
					"contentType":       {Type: "string", Description: "Document MIME type. This flow currently supports text/* only."},
					"versionMarker":     {Type: "string", Description: "Optional source version marker."},
					"etag":              {Type: "string", Description: "Optional source ETag."},
					"sizeBytes":         {Type: "integer", Description: "Optional source size in bytes."},
					"lastModified":      {Type: "string", Description: "Optional source last-modified timestamp."},
					"metadata":          {Type: "object", Description: "Optional document metadata.", AdditionalProperties: true},
					"processingVersion": {Type: "integer", Description: "Optional desired processing version."},
				},
				Required: []string{"documentId", "bucket", "objectKey", "sourceUri", "contentType"},
			},
		},
		Required: []string{"body"},
	}
}

func documentScanBucketSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"body": {
				Type:        "object",
				Description: "Bucket reconciliation request.",
				Properties: map[string]manifestSchema{
					"bucket":            {Type: "string", Description: "Optional bucket name. Defaults to the configured documents bucket."},
					"prefix":            {Type: "string", Description: "Optional object prefix to scan."},
					"maxKeys":           {Type: "integer", Description: "Optional maximum number of objects to scan."},
					"processingVersion": {Type: "integer", Description: "Optional desired processing version for queued text documents."},
				},
			},
		},
	}
}

func documentCurationSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"body": {
				Type:        "object",
				Description: "Document metadata curation request.",
				Properties: map[string]manifestSchema{
					"documentId": {Type: "string", Description: "Existing document identifier from inventory or search results."},
					"metadata":   {Type: "object", Description: "Curated metadata fields to merge into the document inventory record.", AdditionalProperties: true},
					"replace":    {Type: "boolean", Description: "Replace the full metadata object instead of merging fields."},
				},
				Required: []string{"documentId", "metadata"},
			},
		},
		Required: []string{"body"},
	}
}

func documentReprocessSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"body": {
				Type:        "object",
				Description: "Existing document reprocessing request.",
				Properties: map[string]manifestSchema{
					"documentId":        {Type: "string", Description: "Existing document identifier from inventory or search results."},
					"processingVersion": {Type: "integer", Description: "Optional explicit processing version. Defaults to the next version after the current desired version."},
				},
				Required: []string{"documentId"},
			},
		},
		Required: []string{"body"},
	}
}

func documentInventorySchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"status":     {Type: "string", Description: "Optional document lifecycle status filter."},
			"prefix":     {Type: "string", Description: "Optional object-key prefix filter."},
			"documentId": {Type: "string", Description: "Optional exact document identifier filter."},
			"limit":      {Type: "integer", Description: "Optional maximum number of inventory rows to return."},
		},
	}
}

func documentSearchSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"query":      {Type: "string", Description: "Natural-language search query."},
			"prefix":     {Type: "string", Description: "Optional object-key prefix filter."},
			"documentId": {Type: "string", Description: "Optional exact document identifier filter."},
			"limit":      {Type: "integer", Description: "Optional maximum number of chunk hits to return."},
		},
		Required: []string{"query"},
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

func minioMoveObjectSchema() *manifestSchema {
	return &manifestSchema{
		Type: "object",
		Properties: map[string]manifestSchema{
			"sourceObjectKey":      {Type: "string", Description: "Existing object key within the documents bucket."},
			"destinationObjectKey": {Type: "string", Description: "Destination object key within the documents bucket."},
		},
		Required: []string{"sourceObjectKey", "destinationObjectKey"},
	}
}
