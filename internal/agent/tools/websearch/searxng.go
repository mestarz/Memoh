package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

func SearXNG(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	baseURL := stringValue(cfg["base_url"])
	if baseURL == "" {
		return nil, errors.New("SearXNG base URL is required")
	}
	reqURL, _ := url.Parse(strings.TrimRight(baseURL, "/"))
	params := reqURL.Query()
	params.Set("q", query)
	params.Set("format", "json")
	params.Set("pageno", "1")
	if lang := stringValue(cfg["language"]); lang != "" {
		params.Set("language", lang)
	}
	if ss := stringValue(cfg["safesearch"]); ss != "" {
		params.Set("safesearch", ss)
	}
	if cats := stringValue(cfg["categories"]); cats != "" {
		params.Set("categories", cats)
	}
	reqURL.RawQuery = params.Encode()
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	req.Header.Set("Accept", "application/json")
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
		Results []struct {
			Title, URL, Content string
			Score               float64
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	sort.Slice(raw.Results, func(i, j int) bool { return raw.Results[i].Score > raw.Results[j].Score })
	results := make([]map[string]any, 0)
	for i, item := range raw.Results {
		if i >= count {
			break
		}
		results = append(results, map[string]any{"title": item.Title, "url": item.URL, "description": item.Content})
	}
	return map[string]any{"query": query, "results": results}, nil
}
