package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSourceDecoderPreservesIntegersAndRefusesDuplicateKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "source.template")
	if err := os.WriteFile(path, []byte("# empty document\n---\napiVersion: sample.invalid/v1\nkind: Sample\nspec:\n  exact: 9007199254740993\n"), 0600); err != nil {
		t.Fatal(err)
	}
	docs, err := documents(dir, "*.template")
	if err != nil || len(docs) != 1 {
		t.Fatalf("decode: %v %v", docs, err)
	}
	exact, found, err := unstructured.NestedInt64(docs[0].object.Object, "spec", "exact")
	if err != nil || !found || exact != 9007199254740993 {
		t.Fatalf("source number changed: %v %v", exact, err)
	}
	if err = os.WriteFile(path, []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: first\n  name: second\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = documents(dir, "*.template"); err == nil {
		t.Fatal("duplicate YAML key silently changed source")
	}
}

func TestExternalConversionRequiresExplicitLocalFixture(t *testing.T) {
	crd := object(`{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition","metadata":{"name":"sample.invalid"},"spec":{"conversion":{"strategy":"Webhook","webhook":{"clientConfig":{"url":"https://unreachable.invalid"}}}}}`)
	// A nil client proves refusal occurs before any API request or source change.
	err := installCRD(context.Background(), nil, crd)
	if err == nil || !strings.Contains(err.Error(), "local conversion webhook fixture") {
		t.Fatalf("unexpected result: %v", err)
	}
	strategy, _, _ := unstructured.NestedString(crd.Object, "spec", "conversion", "strategy")
	if strategy != "Webhook" {
		t.Fatal("source conversion silently rewritten")
	}
}
