package store

import (
	"context"
	"time"
)

type AccountRecord struct {
	ID              string
	Username        string
	Email           string
	Role            string
	DisplayName     string
	AvatarURL       string
	Timezone        string
	PasswordHash    string
	HasPasswordHash bool
	IsActive        bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastLoginAt     time.Time
}

type CreateUserInput struct {
	IsActive bool
	Metadata []byte
}

type CreateAccountInput struct {
	UserID       string
	Username     string
	Email        string
	PasswordHash string
	Role         string
	DisplayName  string
	AvatarURL    string
	IsActive     bool
	DataRoot     string
}

type UpdateAccountAdminInput struct {
	UserID      string
	Role        string
	DisplayName string
	AvatarURL   string
	IsActive    bool
}

type UpdateAccountProfileInput struct {
	UserID      string
	DisplayName string
	AvatarURL   string
	Timezone    string
	IsActive    bool
}

type UpdateAccountPasswordInput struct {
	UserID       string
	PasswordHash string
}

type AccountStore interface {
	CountAccounts(ctx context.Context) (int64, error)
	GetByUserID(ctx context.Context, userID string) (AccountRecord, error)
	GetByIdentity(ctx context.Context, identity string) (AccountRecord, error)
	List(ctx context.Context) ([]AccountRecord, error)
	Search(ctx context.Context, query string, limit int32) ([]AccountRecord, error)
	CreateUser(ctx context.Context, input CreateUserInput) (AccountRecord, error)
	CreateAccount(ctx context.Context, input CreateAccountInput) (AccountRecord, error)
	UpdateLastLogin(ctx context.Context, accountID string) error
	UpdateAdmin(ctx context.Context, input UpdateAccountAdminInput) (AccountRecord, error)
	UpdateProfile(ctx context.Context, input UpdateAccountProfileInput) (AccountRecord, error)
	UpdatePassword(ctx context.Context, input UpdateAccountPasswordInput) error
}
