package mongoutil

import "testing"

func TestParseMongoConfig(t *testing.T) {
	config, err := ParseMongoConfig(map[string]string{
		"default": `
host: mongo
port: "27017"
database: app
user: app
password: secret
authSource: admin
`,
	})
	if err != nil {
		t.Fatalf("expected config to parse: %v", err)
	}

	got := config["default"]
	if got.Host != "mongo" {
		t.Fatalf("expected host mongo, got %q", got.Host)
	}
	if got.Database != "app" {
		t.Fatalf("expected database app, got %q", got.Database)
	}
	if got.AuthSource != "admin" {
		t.Fatalf("expected authSource admin, got %q", got.AuthSource)
	}
}

func TestBuildURI(t *testing.T) {
	uri := buildURI(MongoConfig{
		Host:       "mongo",
		Port:       "27017",
		User:       "app",
		Password:   "secret",
		AuthSource: "admin",
		ReplicaSet: "rs0",
		Direct:     true,
	})

	expected := "mongodb://app:secret@mongo:27017/?authSource=admin&directConnection=true&replicaSet=rs0"
	if uri != expected {
		t.Fatalf("expected %q, got %q", expected, uri)
	}
}
