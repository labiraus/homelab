package embeddingutil

import (
	"math"
	"testing"
)

func TestEmbedTextReturnsNormalizedVector(t *testing.T) {
	vector := EmbedText("Astra keeps field notes")
	if len(vector) != Dimensions {
		t.Fatalf("expected %d dimensions, got %d", Dimensions, len(vector))
	}

	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if math.Abs(math.Sqrt(sum)-1) > 0.000001 {
		t.Fatalf("expected normalized vector, got magnitude %f", math.Sqrt(sum))
	}
}

func TestEmbedTextIsDeterministic(t *testing.T) {
	first := EmbedText("Astra keeps field notes")
	second := EmbedText("Astra keeps field notes")

	for index := range first {
		if first[index] != second[index] {
			t.Fatalf("expected deterministic value at index %d", index)
		}
	}
}
