// Package store defines the data store interfaces and common types for the application.
package store

import (
	"context"
	"time"
)

// User represents a system user.
type User struct {
	ID         int64
	Email      string
	IsVerified bool
	Timezone   string
}

// SOAPData represents the SOAP journal entry.
type SOAPData struct {
	Date           string   `json:"date"`
	Observation    string   `json:"observation"`
	Application    string   `json:"application"`
	Prayer         string   `json:"prayer"`
	SelectedVerses []string `json:"selectedVerses"`
}

// Store defines the interface for database operations.
type Store interface {
	ConfirmUser(ctx context.Context, token string) (int64, error) // returns rows affected
	CreatePasswordResetToken(ctx context.Context, token string, userID int64, expiresAt time.Time) error
	CreateSession(ctx context.Context, token string, userID int64, expiresAt time.Time) error
	CreateUser(ctx context.Context, email, passwordHash, token, timezone string) error
	DeleteExpiredSessions(ctx context.Context) error
	DeletePasswordResetToken(ctx context.Context, token string) error
	ExpungeCache(ctx context.Context, olderThan time.Duration, keepMax int) error
	GetAuthUser(ctx context.Context, email string) (id int64, passwordHash string, isVerified bool, timezone string, err error)
	GetCachedESV(ctx context.Context, key string) (string, error)
	GetPasswordResetToken(ctx context.Context, token string) (int64, time.Time, error) // returns userID, expiresAt
	GetSOAPData(ctx context.Context, userID int64, dateStr string) (*SOAPData, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserFromSession(ctx context.Context, token string) (*User, error)
	SaveCachedESV(ctx context.Context, key string, content string) error
	SaveSOAPData(ctx context.Context, userID int64, soapData *SOAPData) error
	UpdateUserPassword(ctx context.Context, userID int64, passwordHash string) error
	UpdateUserPasswordHash(ctx context.Context, userID int64, newHash string) error
	UpdateUserTimezone(ctx context.Context, userID int64, timezone string) error
}
