package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"
)

func Jina(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://s.jina.ai/")
	apiKey := stringValue(cfg["api_key"])
	if apiKey == "" {
		return nil, errors.New("jina API key is required")
	}
	if count > 10 {
		count = 10
	}
	payload, _ := json.Marshal(map[string]any{"q": query, "count": count})
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Retain-Images", "none")
	req.Header.Set("Authorization", apiKey)
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildSearchHTTPError(resp.StatusCode, body)
	}
	var raw struct {
		Data []struct{ Title, URL, Content string } `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.Data))
	for _, item := range raw.Data {
		results = append(results, map[string]any{"title": item.Title, "url": item.URL, "description": item.Content})
	}
	return map[string]any{"query": query, "results": results}, nil
}
