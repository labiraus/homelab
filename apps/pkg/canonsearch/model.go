// Package canonsearch implements rebuildable model-derived candidate discovery.
// Callers supply published snapshots and resolve every returned ID against canon.
package canonsearch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const MaxBatch = 32

// The private TEI service accepts eight inputs per HTTP request. A record
// batch can contain 32 records and is streamed through these smaller batches.
const MaxModelInputs = 8
const ChunkBytes = 256
const MaxChunks = 512

// Revision identifies immutable model weights/tokenization/pooling, not a mutable alias.
// Profile identifiers also bind the representation algorithm and query prefix.
type Profile struct {
	Model       string `json:"model"`
	Revision    string `json:"revision"`
	Dimensions  int    `json:"dimensions"`
	QueryPrefix string `json:"queryPrefix"`
}

func (p Profile) Validate() error {
	if strings.TrimSpace(p.Model) == "" || len(p.Model) > 200 || !utf8.ValidString(p.Model) || strings.TrimSpace(p.Revision) == "" || len(p.Revision) > 200 || !utf8.ValidString(p.Revision) || p.Dimensions < 2 || p.Dimensions > 2048 || len(p.QueryPrefix) > 128 || !utf8.ValidString(p.QueryPrefix) {
		return fmt.Errorf("explicit bounded embedding model, immutable revision and dimensions required")
	}
	return nil
}
func (p Profile) ID() string {
	raw, _ := json.Marshal(p)
	sum := sha256.Sum256(append([]byte("canon-mean-chunks-256-v1\n"), raw...))
	return hex.EncodeToString(sum[:])
}

type Usage struct {
	Model       string `json:"model"`
	Requests    int    `json:"requests"`
	InputTokens int64  `json:"inputTokens"`
	LatencyMS   int64  `json:"latencyMs"`
}
type Embedder struct {
	Profile  Profile
	Endpoint string // Full private gateway /v1/embeddings URL; never comes from a user request.
	Token    string
	Client   *http.Client
	once     sync.Once
	gate     chan struct{}
}

func endpoint(value string) error {
	u, err := url.Parse(value)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("invalid fixed service endpoint")
	}
	return nil
}
func (e *Embedder) Validate() error {
	if err := e.Profile.Validate(); err != nil {
		return err
	}
	return endpoint(e.Endpoint)
}
func httpClient(client *http.Client, timeout time.Duration) *http.Client {
	c := http.Client{Timeout: timeout}
	if client != nil {
		c = *client
		if c.Timeout <= 0 || c.Timeout > timeout {
			c.Timeout = timeout
		}
	}
	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return &c
}

// Chunks covers every original UTF-8 byte exactly once; it never truncates a
// record or splits a rune. The conservative byte ceiling bounds tokenizer input.
func Chunks(text string) ([]string, error) {
	if text == "" || !utf8.ValidString(text) || len(text) > ChunkBytes*MaxChunks {
		return nil, fmt.Errorf("embedding input is empty, invalid UTF-8 or exceeds the bounded record size")
	}
	chunks := []string{}
	for len(text) > 0 {
		end := min(ChunkBytes, len(text))
		for end < len(text) && !utf8.RuneStart(text[end]) {
			end--
		}
		chunks = append(chunks, text[:end])
		text = text[end:]
	}
	return chunks, nil
}
func vectorValid(v []float32, dimensions int) bool {
	if len(v) != dimensions {
		return false
	}
	var norm float64
	for _, x := range v {
		if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
			return false
		}
		norm += float64(x) * float64(x)
	}
	return norm > 0 && !math.IsInf(norm, 0)
}
func normalize(v []float32) error {
	if !vectorValid(v, len(v)) {
		return fmt.Errorf("embedding vector is zero, nonfinite or malformed")
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] = float32(float64(v[i]) / norm)
	}
	return nil
}

// Batch performs one model attempt with no redirect, retry, hash fallback or
// optimistic usage refund. Callers reserve interactive allowance before calling.
func (e *Embedder) Batch(ctx context.Context, inputs []string, requestID string) ([][]float32, Usage, error) {
	usage := Usage{}
	if err := e.Validate(); err != nil {
		return nil, usage, err
	}
	if len(inputs) < 1 || len(inputs) > MaxModelInputs || strings.TrimSpace(requestID) == "" || len(requestID) > 200 {
		return nil, usage, fmt.Errorf("invalid embedding batch or correlation ID")
	}
	for _, text := range inputs {
		if text == "" || len(text) > ChunkBytes+128 || !utf8.ValidString(text) {
			return nil, usage, fmt.Errorf("invalid bounded embedding input")
		}
	}
	e.once.Do(func() { e.gate = make(chan struct{}, 1) })
	select {
	case e.gate <- struct{}{}:
		defer func() { <-e.gate }()
	case <-ctx.Done():
		return nil, usage, ctx.Err()
	}
	raw, _ := json.Marshal(map[string]any{"model": e.Profile.Model, "input": inputs, "encoding_format": "float"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.Endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, usage, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", requestID)
	if e.Token != "" {
		req.Header.Set("Authorization", "Bearer "+e.Token)
	}
	started := time.Now()
	usage.Requests = 1
	usage.Model = e.Profile.Model
	res, err := httpClient(e.Client, 20*time.Second).Do(req)
	usage.LatencyMS = time.Since(started).Milliseconds()
	if err != nil {
		return nil, usage, fmt.Errorf("embedding transport failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, usage, fmt.Errorf("embedding returned status %d", res.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, (4<<20)+1))
	usage.LatencyMS = time.Since(started).Milliseconds()
	if err != nil || len(body) > 4<<20 {
		return nil, usage, fmt.Errorf("invalid embedding response size")
	}
	var result struct {
		Model string `json:"model"`
		Data  []struct {
			Index     *int      `json:"index"`
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
		Usage struct {
			PromptTokens *int64 `json:"prompt_tokens"`
		} `json:"usage"`
	}
	if err = json.Unmarshal(body, &result); err != nil {
		return nil, usage, fmt.Errorf("invalid embedding response")
	}
	if result.Model != e.Profile.Model || len(result.Data) != len(inputs) || result.Usage.PromptTokens == nil || *result.Usage.PromptTokens < 0 || *result.Usage.PromptTokens > 1000000 {
		return nil, usage, fmt.Errorf("embedding model, count or usage mismatch")
	}
	vectors := make([][]float32, len(inputs))
	for _, item := range result.Data {
		if item.Index == nil || *item.Index < 0 || *item.Index >= len(inputs) || vectors[*item.Index] != nil || !vectorValid(item.Embedding, e.Profile.Dimensions) {
			return nil, usage, fmt.Errorf("embedding index/dimension/vector mismatch")
		}
		if err = normalize(item.Embedding); err != nil {
			return nil, usage, err
		}
		vectors[*item.Index] = item.Embedding
	}
	usage.Model = result.Model
	usage.InputTokens = *result.Usage.PromptTokens
	return vectors, usage, nil
}

// Texts covers each complete text, including final UTF-8 chunks, with a
// byte-weighted mean of normalized model vectors. It batches across records.
func (e *Embedder) Texts(ctx context.Context, texts []string, requestID string, query bool) ([][]float32, Usage, error) {
	usage := Usage{}
	if err := e.Validate(); err != nil {
		return nil, usage, err
	}
	if len(texts) < 1 || len(texts) > MaxBatch {
		return nil, usage, fmt.Errorf("invalid record batch")
	}
	type part struct {
		owner  int
		text   string
		weight float32
	}
	parts := []part{}
	result := make([][]float32, len(texts))
	for i, text := range texts {
		chunks, err := Chunks(text)
		if err != nil {
			return nil, usage, err
		}
		result[i] = make([]float32, e.Profile.Dimensions)
		for _, chunk := range chunks {
			parts = append(parts, part{i, chunk, float32(len(chunk)) / float32(len(text))})
		}
	}
	for start := 0; start < len(parts); start += MaxModelInputs {
		end := min(start+MaxModelInputs, len(parts))
		inputs := make([]string, end-start)
		for i, p := range parts[start:end] {
			inputs[i] = p.text
			if query {
				inputs[i] = e.Profile.QueryPrefix + p.text
			}
		}
		values, u, err := e.Batch(ctx, inputs, requestID)
		usage.Requests += u.Requests
		usage.LatencyMS += u.LatencyMS
		usage.InputTokens += u.InputTokens
		if u.Model != "" {
			usage.Model = u.Model
		}
		if err != nil {
			return nil, usage, err
		}
		for i, v := range values {
			p := parts[start+i]
			for j := range v {
				result[p.owner][j] += p.weight * v[j]
			}
		}
	}
	for _, v := range result {
		if err := normalize(v); err != nil {
			return nil, usage, err
		}
	}
	return result, usage, nil
}
func (e *Embedder) Mean(ctx context.Context, text, requestID string, query bool) ([]float32, Usage, error) {
	result, usage, err := e.Texts(ctx, []string{text}, requestID, query)
	if err != nil {
		return nil, usage, err
	}
	return result[0], usage, nil
}
