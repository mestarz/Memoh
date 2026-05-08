package websearch

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

func buildSearchResults[T any](query string, items []T, mapper func(T) map[string]any) map[string]any {
	results := make([]map[string]any, 0, len(items))
	for _, item := range items {
		results = append(results, mapper(item))
	}
	return map[string]any{"query": query, "results": results}
}

func buildSearchHTTPError(statusCode int, body []byte) error {
	detail := extractJSONErrorMessage(body)
	if detail == "" {
		detail = strings.TrimSpace(string(body))
	}
	if len(detail) > 200 {
		detail = detail[:200] + "..."
	}
	if detail != "" {
		return fmt.Errorf("search request failed (HTTP %d): %s", statusCode, detail)
	}
	return fmt.Errorf("search request failed (HTTP %d)", statusCode)
}

func extractJSONErrorMessage(body []byte) string {
	var obj map[string]any
	if json.Unmarshal(body, &obj) != nil {
		return ""
	}
	for _, key := range []string{"error", "message", "detail", "error_message"} {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch val := v.(type) {
		case string:
			return val
		case map[string]any:
			if msg, ok := val["message"].(string); ok {
				return msg
			}
		}
	}
	return ""
}

func parseSearchTimeout(configJSON []byte, fallback time.Duration) time.Duration {
	cfg := parseSearchConfig(configJSON)
	raw, ok := cfg["timeout_seconds"]
	if !ok {
		return fallback
	}
	switch value := raw.(type) {
	case float64:
		if value > 0 {
			return time.Duration(value * float64(time.Second))
		}
	case int:
		if value > 0 {
			return time.Duration(value) * time.Second
		}
	}
	return fallback
}

func parseSearchConfig(configJSON []byte) map[string]any {
	if len(configJSON) == 0 {
		return map[string]any{}
	}
	var cfg map[string]any
	if err := json.Unmarshal(configJSON, &cfg); err != nil || cfg == nil {
		return map[string]any{}
	}
	return cfg
}

func stringValue(raw any) string {
	if value, ok := raw.(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
