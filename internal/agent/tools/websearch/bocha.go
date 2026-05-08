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

func Bocha(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://api.bochaai.com/v1/web-search")
	apiKey := stringValue(cfg["api_key"])
	if apiKey == "" {
		return nil, errors.New("bocha API key is required")
	}
	payload, _ := json.Marshal(map[string]any{"query": query, "summary": true, "freshness": "noLimit", "count": count})
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
		Data struct {
			WebPages struct {
				Value []struct{ Name, URL, Summary string } `json:"value"`
			} `json:"webPages"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.Data.WebPages.Value))
	for _, item := range raw.Data.WebPages.Value {
		results = append(results, map[string]any{"title": item.Name, "url": item.URL, "description": item.Summary})
	}
	return map[string]any{"query": query, "results": results}, nil
}
