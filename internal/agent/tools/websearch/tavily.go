package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	tavilyKeyCounters   sync.Map // providerID → *atomic.Uint64
	tavilyExhaustedKeys sync.Map // "providerID:keyIdx" → struct{}
	errAllKeysExhausted = errors.New("all tavily API keys are exhausted")
)

// TavilyKeyUsage holds usage stats for a single API key, used by the probe endpoint.
type TavilyKeyUsage struct {
	Index     int    `json:"index"`
	MaskedKey string `json:"masked_key"`
	Usage     int    `json:"usage"`
	Limit     *int   `json:"limit"` // nil = unlimited
	Error     string `json:"error,omitempty"`
}

func tavilyExhaustMapKey(providerID string, idx int) string {
	return fmt.Sprintf("%s:%d", providerID, idx)
}

func isKeyExhausted(providerID string, idx int) bool {
	_, ok := tavilyExhaustedKeys.Load(tavilyExhaustMapKey(providerID, idx))
	return ok
}

func markKeyExhausted(providerID string, idx int) {
	tavilyExhaustedKeys.Store(tavilyExhaustMapKey(providerID, idx), struct{}{})
}

func clearKeyExhausted(providerID string, idx int) {
	tavilyExhaustedKeys.Delete(tavilyExhaustMapKey(providerID, idx))
}

func isQuotaError(statusCode int, body []byte) bool {
	if statusCode != http.StatusTooManyRequests && statusCode != http.StatusPaymentRequired {
		return false
	}
	lower := strings.ToLower(string(body))
	return strings.Contains(lower, "quota") ||
		strings.Contains(lower, "credit") ||
		strings.Contains(lower, "limit exceeded") ||
		strings.Contains(lower, "usage limit")
}

// MaskAPIKey masks an API key for display: shows first 8 and last 4 characters.
func MaskAPIKey(key string) string {
	if len(key) <= 12 {
		return strings.Repeat("*", len(key))
	}
	return key[:8] + "..." + key[len(key)-4:]
}

// tavilyUsageEndpoint derives the /usage endpoint from the search base URL.
func tavilyUsageEndpoint(searchEndpoint string) string {
	base := strings.TrimRight(searchEndpoint, "/")
	if strings.HasSuffix(base, "/search") {
		return strings.TrimSuffix(base, "search") + "usage"
	}
	if idx := strings.LastIndex(base, "/"); idx >= 0 {
		return base[:idx] + "/usage"
	}
	return "https://api.tavily.com/usage"
}

// QueryKeyUsage fetches usage stats for a Tavily API key from the /usage endpoint.
func QueryKeyUsage(ctx context.Context, key, searchBaseURL string) TavilyKeyUsage {
	usageURL := tavilyUsageEndpoint(searchBaseURL)
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, usageURL, nil)
	if err != nil {
		return TavilyKeyUsage{Error: err.Error()}
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return TavilyKeyUsage{Error: err.Error()}
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Include a snippet of the response body to help diagnose the failure.
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 120 {
			snippet = snippet[:120] + "..."
		}
		return TavilyKeyUsage{Error: fmt.Sprintf("HTTP %d: %s", resp.StatusCode, snippet)}
	}
	var result struct {
		Key struct {
			Usage int  `json:"usage"`
			Limit *int `json:"limit"`
		} `json:"key"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		snippet := strings.TrimSpace(string(body))
		if len(snippet) > 120 {
			snippet = snippet[:120] + "..."
		}
		return TavilyKeyUsage{Error: fmt.Sprintf("invalid response: %s", snippet)}
	}
	return TavilyKeyUsage{
		Usage: result.Key.Usage,
		Limit: result.Key.Limit,
	}
}

// tavilySearchOnce performs a single Tavily search. Returns (result, isQuotaError, error).
func tavilySearchOnce(ctx context.Context, endpoint, key, query string, count int, timeout time.Duration) (any, bool, error) {
	payload, _ := json.Marshal(map[string]any{"query": query, "max_results": count})
	client := &http.Client{Timeout: timeout}
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := client.Do(req) //nolint:gosec
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if isQuotaError(resp.StatusCode, body) {
			return nil, true, errors.New("quota exceeded")
		}
		return nil, false, buildSearchHTTPError(resp.StatusCode, body)
	}
	var raw struct {
		Results []struct {
			Title, URL, Content string
		} `json:"results"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, false, errors.New("invalid search response")
	}
	results := make([]map[string]any, 0, len(raw.Results))
	for _, item := range raw.Results {
		results = append(results, map[string]any{"title": item.Title, "url": item.URL, "description": item.Content})
	}
	return map[string]any{"query": query, "results": results}, false, nil
}

// tavilyTryPool tries all non-exhausted keys in the pool starting from startIdx.
func tavilyTryPool(ctx context.Context, endpoint string, pool []string, providerID string, startIdx int, query string, count int, timeout time.Duration) (any, error) {
	for i := range pool {
		idx := (startIdx + i) % len(pool)
		if isKeyExhausted(providerID, idx) {
			continue
		}
		result, isQuota, err := tavilySearchOnce(ctx, endpoint, pool[idx], query, count, timeout)
		if err == nil {
			return result, nil
		}
		if isQuota {
			markKeyExhausted(providerID, idx)
			continue
		}
		return nil, err // non-quota error: propagate immediately
	}
	return nil, errAllKeysExhausted
}

// tavilyRecheckExhausted re-queries usage for all exhausted keys and unmarks those with remaining credits.
func tavilyRecheckExhausted(ctx context.Context, pool []string, providerID, endpoint string) int {
	recovered := 0
	for i, key := range pool {
		if !isKeyExhausted(providerID, i) {
			continue
		}
		usage := QueryKeyUsage(ctx, key, endpoint)
		if usage.Error == "" && (usage.Limit == nil || usage.Usage < *usage.Limit) {
			clearKeyExhausted(providerID, i)
			recovered++
		}
	}
	return recovered
}

func Tavily(ctx context.Context, configJSON []byte, providerID string, query string, count int) (any, error) {
	cfg := parseSearchConfig(configJSON)
	endpoint := firstNonEmpty(stringValue(cfg["base_url"]), "https://api.tavily.com/search")
	timeout := parseSearchTimeout(configJSON, 15*time.Second)

	pool := TavilyAPIKeyPool(cfg)
	if len(pool) == 0 {
		// Single key mode (legacy)
		key := stringValue(cfg["api_key"])
		if key == "" {
			return nil, errors.New("tavily API key is required")
		}
		result, _, err := tavilySearchOnce(ctx, endpoint, key, query, count, timeout)
		return result, err
	}

	actual, _ := tavilyKeyCounters.LoadOrStore(providerID, new(atomic.Uint64))
	startIdx := int(actual.(*atomic.Uint64).Add(1) % uint64(len(pool))) //nolint:gosec // result is bounded by len(pool).

	result, err := tavilyTryPool(ctx, endpoint, pool, providerID, startIdx, query, count, timeout)
	if errors.Is(err, errAllKeysExhausted) {
		// All exhausted: re-check usage, unmark recovered keys, retry once
		if recovered := tavilyRecheckExhausted(ctx, pool, providerID, endpoint); recovered > 0 {
			result, err = tavilyTryPool(ctx, endpoint, pool, providerID, startIdx, query, count, timeout)
		}
	}
	return result, err
}

func PickAPIKey(cfg map[string]any, providerID string) string {
	if pool := TavilyAPIKeyPool(cfg); len(pool) > 0 {
		actual, _ := tavilyKeyCounters.LoadOrStore(providerID, new(atomic.Uint64))
		idx := actual.(*atomic.Uint64).Add(1) % uint64(len(pool))
		return pool[idx]
	}
	return stringValue(cfg["api_key"])
}

func TavilyAPIKeyPool(cfg map[string]any) []string {
	raw, ok := cfg["api_keys"]
	if !ok {
		return nil
	}
	var pool []string
	switch v := raw.(type) {
	case []any:
		for _, item := range v {
			if k := strings.TrimSpace(fmt.Sprintf("%v", item)); k != "" {
				pool = append(pool, k)
			}
		}
	case []string:
		for _, k := range v {
			if k = strings.TrimSpace(k); k != "" {
				pool = append(pool, k)
			}
		}
	}
	return pool
}
