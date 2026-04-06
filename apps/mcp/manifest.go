package main

import (
	"net/http"
	"pkg/base"
	"strings"
)

const certificateDiscoveryPath = "/.well-known/auth/certificate.json"

var capabilityCatalog = []manifestCapabilitySource{
	{
		ID:      "documents.ingestion",
		Title:   "Document Ingestion",
		Summary: "Queue documents for asynchronous ingestion through the orchestrator control plane.",
		Backend: manifestBackendOrchestrator,
		Operations: []manifestOperationSource{
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
		},
	},
	{
		ID:      "postgres.auth",
		Title:   "Auth Database",
		Summary: "Read authentication and user inventory data directly from Postgres.",
		Backend: manifestBackendPostgres,
		Operations: []manifestOperationSource{
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
			),
		},
	},
	{
		ID:      "minio.documents",
		Title:   "Documents Bucket",
		Summary: "Browse and manage objects in the MinIO documents bucket.",
		Backend: manifestBackendMinIO,
		Operations: []manifestOperationSource{
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
				CertificateDiscoveryURL: baseURL + certificateDiscoveryPath,
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

func plannedResourceTemplate(id string, routeName string, method string, path string, summary string, public bool) manifestOperationSource {
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

func toManifestPrompt(capability manifestCapabilitySource, operation manifestOperationSource) manifestPrompt {
	return manifestPrompt{
		Name:        operation.ID,
		Title:       capability.Title,
		Description: operationDescription(capability, operation),
		Arguments:   []manifestPromptArgument{},
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
