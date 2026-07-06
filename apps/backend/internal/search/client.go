package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	searchTimeout      = 5 * time.Second
	searchMaxResponse  = 1 << 20 // 1 MiB
	searchScopedKeyTTL = 365 * 24 * time.Hour
)

var searchScopedIndexes = []string{"users", "posts", "hashtags"}

// SearchClient uses a startup-provisioned scoped key for all document and
// search operations; the master key is not retained.
type SearchClient struct {
	baseURL    string
	scopedKey  string
	httpClient *http.Client
}

// NewSearchClient creates a SearchClient, provisions a scoped API key using the
// master key, and configures index settings for users, posts, and hashtags.
func NewSearchClient(ctx context.Context, baseURL, masterKey string) (*SearchClient, error) {
	c := &SearchClient{
		baseURL:    baseURL,
		httpClient: &http.Client{},
	}

	if err := c.applySettings(ctx, masterKey); err != nil {
		return nil, fmt.Errorf("search: apply index settings: %w", err)
	}

	scopedKey, err := c.provisionScopedKey(ctx, masterKey)
	if err != nil {
		return nil, fmt.Errorf("search: provision scoped key: %w", err)
	}
	c.scopedKey = scopedKey
	return c, nil
}

// provisionScopedKey creates a finite-lived scoped API key with document and
// search actions on the indexes owned by this service using the master key.
func (c *SearchClient) provisionScopedKey(ctx context.Context, masterKey string) (string, error) {
	body := map[string]any{
		"actions":     []string{"search", "documents.add", "documents.delete"},
		"indexes":     searchScopedIndexes,
		"expiresAt":   time.Now().UTC().Add(searchScopedKeyTTL).Format(time.RFC3339),
		"description": "phasma-backend scoped key",
	}
	var result struct {
		Key string `json:"key"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/keys", masterKey, body, &result); err != nil {
		return "", err
	}
	if result.Key == "" {
		return "", fmt.Errorf("search: empty key in response")
	}
	return result.Key, nil
}

// ApplySettings configures searchable, filterable, and sortable attributes for
// each index.
func (c *SearchClient) ApplySettings(ctx context.Context) error {
	return c.applySettings(ctx, c.scopedKey)
}

func (c *SearchClient) applySettings(ctx context.Context, key string) error {
	type indexSettings struct {
		index    string
		settings map[string]any
	}
	configs := []indexSettings{
		{
			index: "users",
			settings: map[string]any{
				"searchableAttributes": []string{"username", "name"},
			},
		},
		{
			index: "posts",
			settings: map[string]any{
				"searchableAttributes": []string{"description", "username"},
				"filterableAttributes": []string{"hashtags"},
				"sortableAttributes":   []string{"created"},
			},
		},
		{
			index: "hashtags",
			settings: map[string]any{
				"searchableAttributes": []string{"name"},
				"sortableAttributes":   []string{"post_count"},
			},
		},
	}
	for _, cfg := range configs {
		if err := c.applyIndexSettings(ctx, key, cfg.index, cfg.settings); err != nil {
			return fmt.Errorf("search: apply settings for index %q: %w", cfg.index, err)
		}
	}
	return nil
}

func (c *SearchClient) applyIndexSettings(ctx context.Context, key, index string, settings map[string]any) error {
	return c.doJSON(ctx, http.MethodPatch, "/indexes/"+index+"/settings", key, settings, nil)
}

// UpsertDocuments adds or replaces documents in the given index.
func (c *SearchClient) UpsertDocuments(ctx context.Context, index string, documents any) error {
	return c.doJSON(ctx, http.MethodPost, "/indexes/"+index+"/documents", c.scopedKey, documents, nil)
}

// DeleteDocument removes a single document by its ID from the given index.
func (c *SearchClient) DeleteDocument(ctx context.Context, index, id string) error {
	return c.doJSON(ctx, http.MethodDelete, "/indexes/"+index+"/documents/"+id, c.scopedKey, nil, nil)
}

// SearchResult holds a raw search response body.
type SearchResult struct {
	Hits json.RawMessage `json:"hits"`
}

// Search executes a query against the given index and returns the parsed response.
func (c *SearchClient) Search(ctx context.Context, index string, params map[string]any) (*SearchResult, error) {
	var result SearchResult
	if err := c.doJSON(ctx, http.MethodPost, "/indexes/"+index+"/search", c.scopedKey, params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// doJSON sends bounded JSON requests and optionally decodes the response. The
// bearer key is never included in returned errors.
func (c *SearchClient) doJSON(ctx context.Context, method, path, key string, body any, out any) error {
	tctx, cancel := context.WithTimeout(ctx, searchTimeout)
	defer cancel()

	var bodyReader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("search: marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(tctx, method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("search: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+key)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("search: request failed")
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(io.LimitReader(resp.Body, searchMaxResponse))
	if err != nil {
		return fmt.Errorf("search: read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("search: unexpected status %d", resp.StatusCode)
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("search: decode response: %w", err)
		}
	}
	return nil
}
