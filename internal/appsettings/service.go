// Package appsettings exposes a small key/value store for app-wide settings
// that are not bot- or user-scoped (e.g. global HTTP proxy URL used by
// providers and OAuth flows).
package appsettings

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/jackc/pgx/v5"

	"github.com/memohai/memoh/internal/db"
	dbsqlc "github.com/memohai/memoh/internal/db/postgres/sqlc"
	"github.com/memohai/memoh/internal/httpproxy"
)

const (
	// keyHTTPProxyURL is the canonical key used to store the global HTTP
	// proxy URL applied to providers that opted into proxying.
	keyHTTPProxyURL = "network.http_proxy_url"
)

// Service reads and writes app-wide settings persisted in the app_settings
// table. It maintains an in-memory cache for hot-path lookups (e.g. proxy
// resolution on every outbound request).
type Service struct {
	queries *dbsqlc.Queries
	logger  *slog.Logger

	mu       sync.RWMutex
	cache    map[string]string
	hydrated bool
}

// NewService constructs the Service. The provided queries handle is required;
// passing nil causes Get/Set to return an error.
func NewService(log *slog.Logger, queries *dbsqlc.Queries) *Service {
	return &Service{
		queries: queries,
		logger:  log.With(slog.String("service", "appsettings")),
		cache:   make(map[string]string),
	}
}

// GetHTTPProxyURL returns the configured global HTTP proxy URL, or "" if not set.
func (s *Service) GetHTTPProxyURL(ctx context.Context) (string, error) {
	value, err := s.getString(ctx, keyHTTPProxyURL)
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetHTTPProxyURL validates and persists the global HTTP proxy URL. An empty
// or whitespace-only value clears the configuration.
func (s *Service) SetHTTPProxyURL(ctx context.Context, raw string) (string, error) {
	normalized, err := httpproxy.Validate(raw)
	if err != nil {
		return "", err
	}
	if err := s.setString(ctx, keyHTTPProxyURL, normalized); err != nil {
		return "", err
	}
	return normalized, nil
}

func (s *Service) getString(ctx context.Context, key string) (string, error) {
	if s == nil {
		return "", nil
	}

	s.mu.RLock()
	if s.hydrated {
		value := s.cache[key]
		s.mu.RUnlock()
		return value, nil
	}
	s.mu.RUnlock()

	if s.queries == nil {
		return "", nil
	}
	row, err := s.queries.GetAppSetting(ctx, key)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) || errors.Is(err, pgx.ErrNoRows) {
			s.cacheSet(key, "")
			return "", nil
		}
		return "", err
	}
	value, decodeErr := decodeStringValue(row.Value)
	if decodeErr != nil {
		s.logger.Warn("decode app setting value", slog.String("key", key), slog.String("error", decodeErr.Error()))
		return "", nil
	}
	s.cacheSet(key, value)
	return value, nil
}

func (s *Service) setString(ctx context.Context, key, value string) error {
	if s == nil || s.queries == nil {
		return errors.New("appsettings: queries not configured")
	}
	encoded, err := encodeStringValue(value)
	if err != nil {
		return err
	}
	if _, err := s.queries.UpsertAppSetting(ctx, dbsqlc.UpsertAppSettingParams{
		Key:   key,
		Value: encoded,
	}); err != nil {
		return err
	}
	s.cacheSet(key, value)
	return nil
}

func (s *Service) cacheSet(key, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = make(map[string]string)
	}
	s.cache[key] = value
	s.hydrated = true
}

// encodeStringValue stores the string value inside a small JSON envelope so
// the underlying JSONB column type stays consistent across keys.
func encodeStringValue(value string) ([]byte, error) {
	return json.Marshal(map[string]string{"value": value})
}

// decodeStringValue decodes the JSON envelope. It returns an empty string when
// the payload is empty or doesn't contain a string "value".
func decodeStringValue(raw []byte) (string, error) {
	value := strings.TrimSpace(string(raw))
	if value == "" || value == "{}" || value == "null" {
		return "", nil
	}
	var envelope struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(value), &envelope); err != nil {
		return "", err
	}
	return envelope.Value, nil
}
