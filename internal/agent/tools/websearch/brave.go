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

func Brave(ctx context.Context, configJSON []byte, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := strings.TrimRight(firstNonEmpty(stringValue(cfg["base_url"]), "https://api.search.brave.com/res/v1/web/search"), "/")
	reqURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, errors.New("invalid search provider base_url")
	}
	params := reqURL.Query()
	params.Set("q", query)
	params.Set("count", strconv.Itoa(count))
	reqURL.RawQuery = params.Encode()
	timeout := parseSearchTimeout(configJSON, 15*time.Second)
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if apiKey := stringValue(cfg["api_key"]); strings.TrimSpace(apiKey) != "" {
		req.Header.Set("X-Subscription-Token", strings.TrimSpace(apiKey))
	}
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, buildSearchHTTPError(resp.StatusCode, body)
	}
	var raw struct {
		Web struct {
			Results []struct {
				Title, URL, Description string
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, errors.New("invalid search response")
	}
	return buildSearchResults(query, raw.Web.Results, func(r struct{ Title, URL, Description string }) map[string]any {
		return map[string]any{"title": r.Title, "url": r.URL, "description": r.Description}
	}), nil
}
