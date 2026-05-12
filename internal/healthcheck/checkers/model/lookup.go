package modelchecker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/memohai/memoh/internal/db"
	dbsqlc "github.com/memohai/memoh/internal/db/postgres/sqlc"
)

// QueriesLookup adapts sqlc.Queries to the BotModelLookup interface.
type QueriesLookup struct {
	queries *dbsqlc.Queries
}

// NewQueriesLookup creates a BotModelLookup backed by sqlc.Queries.
func NewQueriesLookup(queries *dbsqlc.Queries) *QueriesLookup {
	return &QueriesLookup{queries: queries}
}

// GetBotModelIDs fetches model IDs configured directly on the bot.
func (l *QueriesLookup) GetBotModelIDs(ctx context.Context, botID string) (BotModels, error) {
	if strings.TrimSpace(botID) == "" {
		return BotModels{}, errors.New("bot id is required")
	}
	pgID, err := db.ParseUUID(botID)
	if err != nil {
		return BotModels{}, fmt.Errorf("invalid bot id: %w", err)
	}

	bot, err := l.queries.GetBotByID(ctx, pgID)
	if err != nil {
		return BotModels{}, fmt.Errorf("get bot: %w", err)
	}

	var m BotModels
	m.OwnerUserID = bot.OwnerUserID.String()
	if bot.ChatModelID.Valid {
		m.ChatModelID = bot.ChatModelID.String()
	}
	return m, nil
}
