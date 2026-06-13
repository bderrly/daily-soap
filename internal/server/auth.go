package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"time"

	"github.com/bderrly/daily-soap/internal/auth"
	"github.com/bderrly/daily-soap/internal/store"
)

func generateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate random string", "error", err)
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// User registration and authentication helpers

func createUser(ctx context.Context, email, password, token, timezone string) error {
	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	if err := appStore.CreateUser(ctx, email, hashedPassword, token, timezone); err != nil {
		return fmt.Errorf("creating user in store: %w", err)
	}
	return nil
}

func authenticateUser(ctx context.Context, email, password string) (*store.User, error) {
	id, passwordHash, isVerified, timezone, err := appStore.GetAuthUser(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("authenticating user %q: %w", email, err)
	}

	match, needsUpgrade, err := auth.VerifyPassword(password, passwordHash)
	if err != nil || !match {
		return nil, fmt.Errorf("invalid email or password")
	}

	if needsUpgrade {
		newHash, err := auth.HashPassword(password)
		if err == nil {
			err = appStore.UpdateUserPasswordHash(ctx, id, newHash)
			if err != nil {
				slog.Error("failed to migrate password hash", slog.Int64("user_id", id), slog.Any("error", err))
			}
		} else {
			slog.Error("failed to generate new hash for migration", slog.Int64("user_id", id), slog.Any("error", err))
		}
	}

	if !isVerified {
		return nil, fmt.Errorf("email not verified")
	}

	return &store.User{ID: id, Email: email, Timezone: timezone}, nil
}

func createSession(ctx context.Context, userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating session token: %w", err)
	}
	token := base64.URLEncoding.EncodeToString(b)

	// Clean up expired sessions first.
	if err := appStore.DeleteExpiredSessions(ctx); err != nil {
		slog.Error("failed to cleanup expired sessions", "error", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour * 30) // 30 days
	err := appStore.CreateSession(ctx, token, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("saving session for user %d: %w", userID, err)
	}
	return token, nil
}
