package providers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	memohcopilot "github.com/memohai/memoh/internal/copilot"
	dbsqlc "github.com/memohai/memoh/internal/db/postgres/sqlc"
	"github.com/memohai/memoh/internal/models"
)

const openAIAuthClaimPath = "https://api.openai.com/auth"

type ModelCredentials struct {
	APIKey         string //nolint:gosec // runtime credential material used to construct SDK providers
	CodexAccountID string
	// HTTPClient is set when the provider has opted into using the global
	// HTTP proxy. Callers should pass it through to NewSDKChatModel /
	// NewSDKProvider so that all SDK traffic for this provider is proxied.
	// When nil, callers should construct the default HTTP client.
	HTTPClient *http.Client
}

func SupportsOpenAICodexOAuth(provider dbsqlc.Provider) bool {
	return supportsOAuth(provider)
}

// ResolveProviderHTTPClient returns an HTTP client routed through the global
// HTTP proxy when the provider has opted in and a proxy URL is configured.
// Returns nil when no proxy override is needed; callers should then build a
// default client.
func (s *Service) ResolveProviderHTTPClient(ctx context.Context, provider dbsqlc.Provider) *http.Client {
	if !ProviderUsesProxy(provider) || s.appSettings == nil {
		return nil
	}
	proxyURL, err := s.appSettings.GetHTTPProxyURL(ctx)
	if err != nil || strings.TrimSpace(proxyURL) == "" {
		return nil
	}
	return models.NewProviderHTTPClientWithProxy(0, proxyURL)
}

func (s *Service) ResolveModelCredentials(ctx context.Context, provider dbsqlc.Provider) (ModelCredentials, error) {
	httpClient := s.ResolveProviderHTTPClient(ctx, provider)

	switch models.ClientType(provider.ClientType) {
	case models.ClientTypeGitHubCopilot:
		githubToken, err := s.GetValidAccessToken(ctx, provider.ID.String())
		if err != nil {
			return ModelCredentials{}, err
		}
		copilotToken, err := memohcopilot.ResolveToken(ctx, githubToken)
		if err != nil {
			return ModelCredentials{}, err
		}
		return ModelCredentials{APIKey: copilotToken, HTTPClient: httpClient}, nil

	case models.ClientTypeOpenAICodex:
		token, err := s.GetValidAccessToken(ctx, provider.ID.String())
		if err != nil {
			return ModelCredentials{}, err
		}
		accountID, err := codexAccountIDFromToken(token)
		if err != nil {
			return ModelCredentials{}, err
		}
		return ModelCredentials{
			APIKey:         token,
			CodexAccountID: accountID,
			HTTPClient:     httpClient,
		}, nil

	default:
		apiKey := ProviderConfigString(provider, "api_key")
		return ModelCredentials{
			APIKey:     apiKey,
			HTTPClient: httpClient,
		}, nil
	}
}

func codexAccountIDFromToken(token string) (string, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid oauth access token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("decode oauth token payload: %w", err)
	}
	var claims struct {
		OpenAIAuth struct {
			ChatGPTAccountID string `json:"chatgpt_account_id"`
		} `json:"https://api.openai.com/auth"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return "", fmt.Errorf("parse oauth token payload: %w", err)
	}
	accountID := strings.TrimSpace(claims.OpenAIAuth.ChatGPTAccountID)
	if accountID == "" {
		return "", fmt.Errorf("oauth access token missing %s.chatgpt_account_id", openAIAuthClaimPath)
	}
	return accountID, nil
}
