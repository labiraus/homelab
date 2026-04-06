package minioutil

import "testing"

func TestParseConfig(t *testing.T) {
	config, err := ParseConfig(map[string]string{
		"default": `
endpoint: minio.data.svc.cluster.local:9000
accessKey: user
secretKey: secret
useSSL: false
region: us-east-1
bucket: app
`,
	})
	if err != nil {
		t.Fatalf("expected config to parse: %v", err)
	}

	got := config["default"]
	if got.Endpoint != "minio.data.svc.cluster.local:9000" {
		t.Fatalf("expected endpoint to match, got %q", got.Endpoint)
	}
	if got.Bucket != "app" {
		t.Fatalf("expected bucket app, got %q", got.Bucket)
	}
	if got.Region != "us-east-1" {
		t.Fatalf("expected region us-east-1, got %q", got.Region)
	}
}
