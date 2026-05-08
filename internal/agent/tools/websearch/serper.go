package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sort"
	"time"
)

func Serper(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://google.serper.dev/search")
	apiKey := stringValue(cfg["api_key"])
	if apiKey == "" {
		return nil, errors.New("serper API key is required")
	}
	payload, _ := json.Marshal(map[string]any{"q": query})
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-KEY", apiKey)
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
		Organic []struct {
			Title, Link, Description string
			Position                 int
		} `json:"organic"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	sort.Slice(raw.Organic, func(i, j int) bool { return raw.Organic[i].Position < raw.Organic[j].Position })
	results := make([]map[string]any, 0)
	for i, item := range raw.Organic {
		if i >= count {
			break
		}
		results = append(results, map[string]any{"title": item.Title, "url": item.Link, "description": item.Description})
	}
	return map[string]any{"query": query, "results": results}, nil
}
