package tools

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"

	sdk "github.com/memohai/twilight-ai/sdk"

	"github.com/memohai/memoh/internal/agent/tools/websearch"
	"github.com/memohai/memoh/internal/channel"
	"github.com/memohai/memoh/internal/db/postgres/sqlc"
	"github.com/memohai/memoh/internal/searchproviders"
	"github.com/memohai/memoh/internal/settings"
)

type WebProvider struct {
	logger          *slog.Logger
	settings        *settings.Service
	searchProviders *searchproviders.Service
}

func NewWebProvider(log *slog.Logger, settingsSvc *settings.Service, searchSvc *searchproviders.Service) *WebProvider {
	if log == nil {
		log = slog.Default()
	}
	return &WebProvider{
		logger:          log.With(slog.String("tool", "web")),
		settings:        settingsSvc,
		searchProviders: searchSvc,
	}
}

func (p *WebProvider) Tools(_ context.Context, session SessionContext) ([]sdk.Tool, error) {
	if p.settings == nil || p.searchProviders == nil {
		return nil, nil
	}
	sess := session
	return []sdk.Tool{
		{
			Name:        "web_search",
			Description: "Search web results via configured search provider.",
			Parameters: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "Search query"},
					"count": map[string]any{"type": "integer", "description": "Number of results, default 5"},
				},
				"required": []string{"query"},
			},
			Execute: func(ctx *sdk.ToolExecContext, input any) (any, error) {
				return p.execWebSearch(ctx.Context, sess, inputAsMap(input))
			},
		},
	}, nil
}

func (p *WebProvider) execWebSearch(ctx context.Context, session SessionContext, args map[string]any) (any, error) {
	botID := strings.TrimSpace(session.BotID)
	if botID == "" {
		return nil, errors.New("bot_id is required")
	}
	botSettings, err := p.settings.GetBot(ctx, botID)
	if err != nil {
		return nil, err
	}
	searchProviderID := strings.TrimSpace(botSettings.SearchProviderID)
	if searchProviderID == "" {
		return nil, errors.New("search provider not configured for this bot")
	}
	provider, err := p.searchProviders.GetRawByID(ctx, searchProviderID)
	if err != nil {
		return nil, err
	}
	registerSearchProviderSecrets(provider)

	query := strings.TrimSpace(StringArg(args, "query"))
	if query == "" {
		return nil, errors.New("query is required")
	}
	count := 5
	if value, ok, err := IntArg(args, "count"); err != nil {
		return nil, err
	} else if ok && value > 0 {
		count = value
	}
	if count > 20 {
		count = 20
	}
	return callSearch(ctx, provider.Provider, provider.ID.String(), provider.Config, query, count)
}

func callSearch(ctx context.Context, providerName, providerID string, configJSON []byte, query string, count int) (any, error) {
	switch strings.TrimSpace(providerName) {
	case string(searchproviders.ProviderBrave):
		return websearch.Brave(ctx, configJSON, query, count)
	case string(searchproviders.ProviderBing):
		return websearch.Bing(ctx, configJSON, query, count)
	case string(searchproviders.ProviderGoogle):
		return websearch.Google(ctx, configJSON, query, count)
	case string(searchproviders.ProviderTavily):
		return websearch.Tavily(ctx, configJSON, providerID, query, count)
	case string(searchproviders.ProviderSogou):
		return websearch.Sogou(ctx, configJSON, query, count)
	case string(searchproviders.ProviderSerper):
		return websearch.Serper(ctx, configJSON, query, count)
	case string(searchproviders.ProviderSearXNG):
		return websearch.SearXNG(ctx, configJSON, query, count)
	case string(searchproviders.ProviderJina):
		return websearch.Jina(ctx, configJSON, query, count)
	case string(searchproviders.ProviderExa):
		return websearch.Exa(ctx, configJSON, query, count)
	case string(searchproviders.ProviderBocha):
		return websearch.Bocha(ctx, configJSON, query, count)
	case string(searchproviders.ProviderDuckDuckGo):
		return websearch.DuckDuckGo(ctx, configJSON, query, count)
	case string(searchproviders.ProviderYandex):
		return websearch.Yandex(ctx, configJSON, query, count)
	default:
		return nil, errors.New("unsupported search provider")
	}
}

var searchProviderSecretFields = []string{"api_key", "secret_id", "secret_key"}

func registerSearchProviderSecrets(provider sqlc.SearchProvider) {
	var cfg map[string]any
	if len(provider.Config) > 0 {
		_ = json.Unmarshal(provider.Config, &cfg)
	}
	if cfg == nil {
		cfg = map[string]any{}
	}
	var secrets []string
	for _, key := range searchProviderSecretFields {
		if v, ok := cfg[key].(string); ok && strings.TrimSpace(v) != "" {
			secrets = append(secrets, strings.TrimSpace(v))
		}
	}
	secrets = append(secrets, websearch.TavilyAPIKeyPool(cfg)...)
	if len(secrets) > 0 {
		channel.SetIMErrorSecrets("search:"+provider.ID.String(), secrets...)
	}
}
