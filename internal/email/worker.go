package email

import (
	"context"
	"log"
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
		log.Printf("error getting pending emails: %v", err)
		return
	}

	for _, e := range emails {
		err := client.send(ctx, e.Recipient, e.Subject, e.BodyHTML)
		if err != nil {
			log.Printf("error sending email %d to %s: %v", e.ID, e.Recipient, err)
			handleFailure(ctx, s, e)
			continue
		}

		if err := s.MarkEmailSent(ctx, e.ID); err != nil {
			log.Printf("error marking email %d as sent: %v", e.ID, err)
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
			log.Printf("error setting email %d status to failed: %v", e.ID, err)
		}
		return
	}

	// Calculate next attempt time
	backoffMinutes := backoffs[e.Attempts] // use current attempts to index into backoffs
	nextAttempt := time.Now().Add(time.Duration(backoffMinutes) * time.Minute)

	if err := s.UpdateEmailStatus(ctx, e.ID, "pending", &nextAttempt); err != nil {
		log.Printf("error updating email %d status: %v", e.ID, err)
	}
}
