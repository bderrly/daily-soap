// Package expunger provides a background service to remove expired entries from the ESV cache.
package expunger

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"derrclan.com/moravian-soap/internal/store"
)

// Start initializes the cache expunger service.
// It runs an initial expunge immediately in a background goroutine and then schedules
// it to run every 24 hours.
func Start(ctx context.Context, s store.Store) {
	go func() {
		slog.Debug("starting initial cache expunge")
		if err := Expunge(ctx, s); err != nil {
			slog.Error("failed to expunge cache", "error", err)
		}

		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				slog.Debug("starting scheduled cache expunge")
				if err := Expunge(ctx, s); err != nil {
					slog.Error("failed to expunge cache", "error", err)
				}
			case <-ctx.Done():
				slog.Info("stopping cache expunger service")
				return
			}
		}
	}()
}

// Expunge removes old and excess entries from the esv_cache table.
func Expunge(ctx context.Context, s store.Store) error {
	if err := s.ExpungeCache(ctx, 28*24*time.Hour, 500); err != nil {
		return fmt.Errorf("expunging cache: %w", err)
	}
	return nil
}
