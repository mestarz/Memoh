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

func Exa(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://api.exa.ai/search")
	apiKey := stringValue(cfg["api_key"])
	if apiKey == "" {
		return nil, errors.New("exa API key is required")
	}
	payload, _ := json.Marshal(map[string]any{"query": query, "numResults": count, "contents": map[string]any{"text": true, "highlights": true}, "type": "auto"})
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
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
		Results []struct{ Title, URL, Text string } `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.Results))
	for _, item := range raw.Results {
		results = append(results, map[string]any{"title": item.Title, "url": item.URL, "description": item.Text})
	}
	return map[string]any{"query": query, "results": results}, nil
}
