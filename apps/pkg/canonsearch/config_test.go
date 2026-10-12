package canonsearch

import "testing"

func TestConfigurationRequiresPinnedProfile(t *testing.T) {
	values := map[string]string{}
	get := func(k string) string { return values[k] }
	if index, err := FromConfig(get, ""); err != nil || index != nil {
		t.Fatal("default must remain disabled")
	}
	values["WORLDBUILDER_SEMANTIC_ENABLED"] = "true"
	if _, err := FromConfig(get, "http://127.0.0.1:9200"); err == nil {
		t.Fatal("missing model accepted")
	}
	values["WORLDBUILDER_EMBEDDING_URL"] = "http://127.0.0.1:8080/v1/embeddings"
	values["WORLDBUILDER_EMBEDDING_MODEL"] = "fixture"
	values["WORLDBUILDER_EMBEDDING_REVISION"] = "immutable-fixture"
	values["WORLDBUILDER_EMBEDDING_DIMENSIONS"] = "2"
	if _, err := FromConfig(get, "http://127.0.0.1:9200"); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"WORLDBUILDER_SEMANTIC_ENABLED": "maybe", "WORLDBUILDER_EMBEDDING_REVISION": "", "WORLDBUILDER_EMBEDDING_DIMENSIONS": "0", "WORLDBUILDER_EMBEDDING_URL": "http://user:password@127.0.0.1/v1/embeddings"} {
		old := values[key]
		values[key] = value
		if _, err := FromConfig(get, "http://127.0.0.1:9200"); err == nil {
			t.Errorf("invalid %s accepted", key)
		}
		values[key] = old
	}
}
