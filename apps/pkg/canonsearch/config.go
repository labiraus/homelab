package canonsearch

import (
	"fmt"
	"strconv"
	"strings"
)

// FromConfig has no implicit model or dimensions. Enabling either application
// requires the same immutable profile and an operator-selected private route.
func FromConfig(get func(string) string, searchURL string) (*Index, error) {
	enabled := get("WORLDBUILDER_SEMANTIC_ENABLED")
	if enabled == "" || strings.EqualFold(enabled, "false") {
		return nil, nil
	}
	if !strings.EqualFold(enabled, "true") {
		return nil, fmt.Errorf("invalid semantic feature flag")
	}
	dimensions, err := strconv.Atoi(get("WORLDBUILDER_EMBEDDING_DIMENSIONS"))
	if err != nil {
		return nil, fmt.Errorf("explicit embedding dimensions required")
	}
	i := &Index{Endpoint: searchURL, Username: get("OPENSEARCH_USERNAME"), Password: get("OPENSEARCH_PASSWORD"), Model: &Embedder{
		Endpoint: get("WORLDBUILDER_EMBEDDING_URL"), Token: get("WORLDBUILDER_EMBEDDING_TOKEN"),
		Profile: Profile{Model: get("WORLDBUILDER_EMBEDDING_MODEL"), Revision: get("WORLDBUILDER_EMBEDDING_REVISION"), Dimensions: dimensions, QueryPrefix: get("WORLDBUILDER_EMBEDDING_QUERY_PREFIX")},
	}}
	if err = i.Validate(); err != nil {
		return nil, err
	}
	return i, nil
}
