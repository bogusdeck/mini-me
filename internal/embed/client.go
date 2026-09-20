package embed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	DefaultBaseURL = "http://127.0.0.1:11434"
	DefaultModel   = "nomic-embed-text"
	DocPrefix      = "search_document: "
	QueryPrefix    = "search_query: "
	TargetDims     = 256
	MaxBatchSize   = 64
	DefaultTimeout = 30 * time.Second
	MaxConcurrency = 2
	MaxRetries     = 3
)

type Client struct {
	baseURL    string
	model      string
	httpClient *http.Client
	semaphore  chan struct{}
}

type ClientOption func(*Client)

func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = strings.TrimRight(url, "/")
	}
}

func WithModel(model string) ClientOption {
	return func(c *Client) {
		c.model = model
	}
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

func NewClient(opts ...ClientOption) *Client {
	baseURL := os.Getenv("OLLAMA_HOST")
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   DefaultModel,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
		semaphore: make(chan struct{}, MaxConcurrency),
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

// EmbedRequest payload for Ollama /api/embed endpoint.
type EmbedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// EmbedResponse payload from Ollama /api/embed endpoint.
type EmbedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

// ModelInfo item from Ollama /api/tags endpoint.
type ModelInfo struct {
	Name  string `json:"name"`
	Model string `json:"model"`
}

// TagsResponse payload from Ollama /api/tags endpoint.
type TagsResponse struct {
	Models []ModelInfo `json:"models"`
}

// NormalizeVector slices a vector to 256 dimensions and applies L2 normalization.
func NormalizeVector(raw []float32) ([]float32, error) {
	if len(raw) < TargetDims {
		return nil, fmt.Errorf("vector dimensions %d less than target %d", len(raw), TargetDims)
	}

	sliced := raw[:TargetDims]
	var sumSq float64
	for _, val := range sliced {
		sumSq += float64(val) * float64(val)
	}

	norm := float32(math.Sqrt(sumSq))
	result := make([]float32, TargetDims)
	if norm > 0 {
		for i := 0; i < TargetDims; i++ {
			result[i] = sliced[i] / norm
		}
	} else {
		copy(result, sliced)
	}
	return result, nil
}

// EmbedDocumentChunks embeds document texts using "search_document: " prefix with batching.
func (c *Client) EmbedDocumentChunks(ctx context.Context, texts []string) ([][]float32, error) {
	prefixed := make([]string, len(texts))
	for i, t := range texts {
		prefixed[i] = DocPrefix + t
	}
	return c.EmbedBatch(ctx, prefixed)
}

// EmbedQuery embeds a search query using "search_query: " prefix.
func (c *Client) EmbedQuery(ctx context.Context, query string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{QueryPrefix + query})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, errors.New("no embedding returned for query")
	}
	return vecs[0], nil
}

// EmbedBatch embeds a slice of inputs in batches of up to MaxBatchSize (64) with bounded concurrency (max 2).
func (c *Client) EmbedBatch(ctx context.Context, inputs []string) ([][]float32, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	results := make([][]float32, len(inputs))

	for i := 0; i < len(inputs); i += MaxBatchSize {
		end := i + MaxBatchSize
		if end > len(inputs) {
			end = len(inputs)
		}

		batch := inputs[i:end]
		batchVecs, err := c.executeEmbedRequest(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch embed failed at index %d: %w", i, err)
		}

		if len(batchVecs) != len(batch) {
			return nil, fmt.Errorf("expected %d embeddings, got %d", len(batch), len(batchVecs))
		}

		for j, rawVec := range batchVecs {
			normVec, err := NormalizeVector(rawVec)
			if err != nil {
				return nil, fmt.Errorf("failed normalizing vector at index %d: %w", i+j, err)
			}
			results[i+j] = normVec
		}
	}

	return results, nil
}

func (c *Client) executeEmbedRequest(ctx context.Context, batch []string) ([][]float32, error) {
	// Bounded concurrency via semaphore
	select {
	case c.semaphore <- struct{}{}:
		defer func() { <-c.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	reqBody, err := json.Marshal(EmbedRequest{
		Model: c.model,
		Input: batch,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embed request: %w", err)
	}

	url := c.baseURL + "/api/embed"
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<attempt*100) * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBody))
		if err != nil {
			return nil, fmt.Errorf("failed creating http request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			buf := new(bytes.Buffer)
			_, _ = buf.ReadFrom(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("ollama API error (status %d): %s", resp.StatusCode, buf.String())
			if resp.StatusCode >= 500 {
				continue
			}
			return nil, lastErr
		}

		var embedResp EmbedResponse
		err = json.NewDecoder(resp.Body).Decode(&embedResp)
		_ = resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed decoding embed response: %w", err)
		}

		return embedResp.Embeddings, nil
	}

	return nil, fmt.Errorf("failed after %d retries: %w", MaxRetries, lastErr)
}

// CheckReachability checks if Ollama server is reachable.
func (c *Client) CheckReachability(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/", nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama is unreachable at %s: %w", c.baseURL, err)
	}
	_ = resp.Body.Close()
	return nil
}

// CheckModelPulled checks if configured model (e.g. nomic-embed-text) is installed in Ollama.
func (c *Client) CheckModelPulled(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/tags", nil)
	if err != nil {
		return false, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to list models from Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("Ollama tags API returned status %d", resp.StatusCode)
	}

	var tags TagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return false, fmt.Errorf("failed decoding tags response: %w", err)
	}

	modelPrefix := strings.Split(c.model, ":")[0]
	for _, m := range tags.Models {
		if m.Name == c.model || m.Model == c.model || strings.HasPrefix(m.Name, modelPrefix+":") {
			return true, nil
		}
	}

	return false, nil
}

// ModelName returns the configured embedding model name.
func (c *Client) ModelName() string {
	return c.model
}

// BaseURL returns the configured Ollama base URL.
func (c *Client) BaseURL() string {
	return c.baseURL
}
