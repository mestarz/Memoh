package accounts

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/memohai/memoh/internal/db"
	dbsqlc "github.com/memohai/memoh/internal/db/postgres/sqlc"
)

// PostgresStore is a postgres-backed implementation of AccountStore.
type PostgresStore struct {
	queries *dbsqlc.Queries
}

// NewPostgresStore returns an AccountStore backed by the given sqlc queries.
func NewPostgresStore(queries *dbsqlc.Queries) *PostgresStore {
	return &PostgresStore{queries: queries}
}

func (s *PostgresStore) CountAccounts(ctx context.Context) (int64, error) {
	return s.queries.CountAccounts(ctx)
}

func (s *PostgresStore) GetByUserID(ctx context.Context, userID string) (AccountRecord, error) {
	id, err := db.ParseUUID(userID)
	if err != nil {
		return AccountRecord{}, err
	}
	row, err := s.queries.GetAccountByUserID(ctx, id)
	if err != nil {
		return AccountRecord{}, mapQueryErr(err)
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) GetByIdentity(ctx context.Context, identity string) (AccountRecord, error) {
	row, err := s.queries.GetAccountByIdentity(ctx, pgtype.Text{String: identity, Valid: identity != ""})
	if err != nil {
		return AccountRecord{}, mapQueryErr(err)
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) List(ctx context.Context) ([]AccountRecord, error) {
	rows, err := s.queries.ListAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return accountRecords(rows), nil
}

func (s *PostgresStore) Search(ctx context.Context, query string, limit int32) ([]AccountRecord, error) {
	rows, err := s.queries.SearchAccounts(ctx, dbsqlc.SearchAccountsParams{
		Query:      query,
		LimitCount: limit,
	})
	if err != nil {
		return nil, err
	}
	return accountRecords(rows), nil
}

func (s *PostgresStore) CreateUser(ctx context.Context, input CreateUserInput) (AccountRecord, error) {
	row, err := s.queries.CreateUser(ctx, dbsqlc.CreateUserParams{
		IsActive: input.IsActive,
		Metadata: input.Metadata,
	})
	if err != nil {
		return AccountRecord{}, err
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) CreateAccount(ctx context.Context, input CreateAccountInput) (AccountRecord, error) {
	userID, err := db.ParseUUID(input.UserID)
	if err != nil {
		return AccountRecord{}, err
	}
	row, err := s.queries.CreateAccount(ctx, dbsqlc.CreateAccountParams{
		UserID:       userID,
		Username:     text(input.Username),
		Email:        optionalText(input.Email),
		PasswordHash: text(input.PasswordHash),
		Role:         input.Role,
		DisplayName:  optionalText(input.DisplayName),
		AvatarUrl:    optionalText(input.AvatarURL),
		IsActive:     input.IsActive,
		DataRoot:     optionalText(input.DataRoot),
	})
	if err != nil {
		return AccountRecord{}, err
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) UpdateLastLogin(ctx context.Context, accountID string) error {
	id, err := db.ParseUUID(accountID)
	if err != nil {
		return err
	}
	_, err = s.queries.UpdateAccountLastLogin(ctx, id)
	return err
}

func (s *PostgresStore) UpdateAdmin(ctx context.Context, input UpdateAccountAdminInput) (AccountRecord, error) {
	userID, err := db.ParseUUID(input.UserID)
	if err != nil {
		return AccountRecord{}, err
	}
	row, err := s.queries.UpdateAccountAdmin(ctx, dbsqlc.UpdateAccountAdminParams{
		UserID:      userID,
		Role:        input.Role,
		DisplayName: optionalText(input.DisplayName),
		AvatarUrl:   optionalText(input.AvatarURL),
		IsActive:    input.IsActive,
	})
	if err != nil {
		return AccountRecord{}, mapQueryErr(err)
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) UpdateProfile(ctx context.Context, input UpdateAccountProfileInput) (AccountRecord, error) {
	userID, err := db.ParseUUID(input.UserID)
	if err != nil {
		return AccountRecord{}, err
	}
	row, err := s.queries.UpdateAccountProfile(ctx, dbsqlc.UpdateAccountProfileParams{
		ID:          userID,
		DisplayName: optionalText(input.DisplayName),
		AvatarUrl:   optionalText(input.AvatarURL),
		Timezone:    input.Timezone,
		IsActive:    input.IsActive,
	})
	if err != nil {
		return AccountRecord{}, mapQueryErr(err)
	}
	return accountRecord(row), nil
}

func (s *PostgresStore) UpdatePassword(ctx context.Context, input UpdateAccountPasswordInput) error {
	userID, err := db.ParseUUID(input.UserID)
	if err != nil {
		return err
	}
	_, err = s.queries.UpdateAccountPassword(ctx, dbsqlc.UpdateAccountPasswordParams{
		ID:           userID,
		PasswordHash: text(input.PasswordHash),
	})
	return mapQueryErr(err)
}

func mapQueryErr(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return db.ErrNotFound
	}
	return err
}

func text(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}

func optionalText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: value != ""}
}

func accountRecords(rows []dbsqlc.User) []AccountRecord {
	items := make([]AccountRecord, 0, len(rows))
	for _, row := range rows {
		items = append(items, accountRecord(row))
	}
	return items
}

func accountRecord(row dbsqlc.User) AccountRecord {
	rec := AccountRecord{
		ID:              row.ID.String(),
		Username:        row.Username.String,
		Email:           row.Email.String,
		Role:            row.Role,
		DisplayName:     row.DisplayName.String,
		AvatarURL:       row.AvatarUrl.String,
		Timezone:        row.Timezone,
		PasswordHash:    row.PasswordHash.String,
		HasPasswordHash: row.PasswordHash.Valid,
		IsActive:        row.IsActive,
	}
	if row.CreatedAt.Valid {
		rec.CreatedAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		rec.UpdatedAt = row.UpdatedAt.Time
	}
	if row.LastLoginAt.Valid {
		rec.LastLoginAt = row.LastLoginAt.Time
	}
	return rec
}
