package embeddingutil

import (
	"math"
	"strings"
)

const (
	DefaultModel = "local-embeddings"
	Dimensions   = 384
)

func EmbedText(input string) []float64 {
	vector := make([]float64, Dimensions)
	tokens := tokenize(input)
	if len(tokens) == 0 {
		tokens = []string{strings.ToLower(strings.TrimSpace(input))}
	}

	for _, token := range tokens {
		hash := fnv1a(token)
		index := int(hash % Dimensions)
		weight := 1.0
		if hash&0x80000000 != 0 {
			weight = -1.0
		}
		vector[index] += weight
	}

	normalize(vector)
	return vector
}

func tokenize(input string) []string {
	return strings.FieldsFunc(strings.ToLower(input), func(r rune) bool {
		return !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
}

func fnv1a(value string) uint32 {
	var hash uint32 = 2166136261
	for i := 0; i < len(value); i++ {
		hash ^= uint32(value[i])
		hash *= 16777619
	}
	return hash
}

func normalize(vector []float64) {
	var sum float64
	for _, value := range vector {
		sum += value * value
	}
	if sum == 0 {
		return
	}

	magnitude := math.Sqrt(sum)
	for index := range vector {
		vector[index] /= magnitude
	}
}
