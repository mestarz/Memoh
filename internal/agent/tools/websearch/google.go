package websearch

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func Google(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := strings.TrimRight(firstNonEmpty(stringValue(cfg["base_url"]), "https://customsearch.googleapis.com/customsearch/v1"), "/")
	reqURL, _ := url.Parse(endpoint)
	cx := stringValue(cfg["cx"])
	if cx == "" {
		return nil, errors.New("google custom search requires cx (search engine ID)")
	}
	if count > 10 {
		count = 10
	}
	params := reqURL.Query()
	params.Set("q", query)
	params.Set("cx", cx)
	params.Set("num", strconv.Itoa(count))
	if apiKey := stringValue(cfg["api_key"]); apiKey != "" {
		params.Set("key", apiKey)
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
		Items []struct {
			Title, Link, Snippet string
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.Items))
	for _, item := range raw.Items {
		results = append(results, map[string]any{"title": item.Title, "url": item.Link, "description": item.Snippet})
	}
	return map[string]any{"query": query, "results": results}, nil
}
