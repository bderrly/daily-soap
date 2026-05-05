package email

import (
	"context"
	"log/slog"
	"time"

	"derrclan.com/moravian-soap/internal/store"
)

// StartWorker starts a background worker that polls for pending emails and sends them.
func StartWorker(ctx context.Context, s store.Store, client *Client) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processPendingEmails(ctx, s, client)
		}
	}
}

func processPendingEmails(ctx context.Context, s store.Store, client *Client) {
	emails, err := s.GetPendingEmails(ctx, 10)
	if err != nil {
		slog.Error("error getting pending emails", "error", err)
		return
	}

	for _, e := range emails {
		err := client.send(ctx, e.Recipient, e.Subject, e.BodyHTML, "sent queued email")
		if err != nil {
			slog.Error("error sending email", "email_id", e.ID, "recipient", e.Recipient, "error", err)
			handleFailure(ctx, s, e)
			continue
		}

		// Log success with additional context
		slog.Info("successfully sent queued email", "email_id", e.ID, "recipient", e.Recipient, "user_id", e.UserID, "subject", e.Subject)

		if err := s.MarkEmailSent(ctx, e.ID); err != nil {
			slog.Error("error marking email as sent", "email_id", e.ID, "error", err)
		}
	}
}

func handleFailure(ctx context.Context, s store.Store, e *store.QueuedEmail) {
	backoffs := []int{5, 15, 60, 240, 1440}

	// e.Attempts is the number of previous attempts.
	// This failure is the (e.Attempts + 1)-th attempt.
	newAttempts := e.Attempts + 1

	if newAttempts >= 5 {
		if err := s.UpdateEmailStatus(ctx, e.ID, "failed", nil); err != nil {
			slog.Error("error setting email status to failed", "email_id", e.ID, "error", err)
		}
		return
	}

	// Calculate next attempt time
	backoffMinutes := backoffs[e.Attempts] // use current attempts to index into backoffs
	nextAttempt := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	if err := s.UpdateEmailStatus(ctx, e.ID, "pending", &nextAttempt); err != nil {
		slog.Error("error updating email status", "email_id", e.ID, "error", err)
	}
}
