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

func Bing(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := strings.TrimRight(firstNonEmpty(stringValue(cfg["base_url"]), "https://api.bing.microsoft.com/v7.0/search"), "/")
	reqURL, _ := url.Parse(endpoint)
	params := reqURL.Query()
	params.Set("q", query)
	params.Set("count", strconv.Itoa(count))
	reqURL.RawQuery = params.Encode()
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	req.Header.Set("Accept", "application/json")
	if apiKey := stringValue(cfg["api_key"]); apiKey != "" {
		req.Header.Set("Ocp-Apim-Subscription-Key", apiKey)
	}
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
		WebPages struct {
			Value []struct {
				Name, URL, Snippet string
			} `json:"value"`
		} `json:"webPages"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.WebPages.Value))
	for _, item := range raw.WebPages.Value {
		results = append(results, map[string]any{"title": item.Name, "url": item.URL, "description": item.Snippet})
	}
	return map[string]any{"query": query, "results": results}, nil
}
