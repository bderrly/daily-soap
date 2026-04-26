package email

import (
	"context"
	"testing"
	"time"

	"derrclan.com/moravian-soap/internal/store"
)

type mockStore struct {
	store.Store
	pendingEmails []*store.QueuedEmail
	sentEmails    []int64
	updatedEmails []updatedEmail
}

type updatedEmail struct {
	id          int64
	status      string
	nextAttempt *time.Time
}

func (m *mockStore) GetPendingEmails(ctx context.Context, limit int) ([]*store.QueuedEmail, error) {
	if len(m.pendingEmails) > limit {
		return m.pendingEmails[:limit], nil
	}
	return m.pendingEmails, nil
}

func (m *mockStore) MarkEmailSent(ctx context.Context, id int64) error {
	m.sentEmails = append(m.sentEmails, id)
	return nil
}

func (m *mockStore) UpdateEmailStatus(ctx context.Context, id int64, status string, nextAttempt *time.Time) error {
	m.updatedEmails = append(m.updatedEmails, updatedEmail{id, status, nextAttempt})
	return nil
}

type mockMailgun struct {
	sendErr error
}

func (m *mockMailgun) Send(ctx context.Context, message any) (string, string, error) {
	return "", "", m.sendErr
}

// Implement other Mailgun methods if needed, or use a more targeted mock if possible.
// Since client.send uses c.mg.Send, we just need that.
// Actually mailgun.Mailgun is a huge interface.

func TestHandleFailure(t *testing.T) {
	ms := &mockStore{}

	tests := []struct {
		name          string
		attempts      int
		expectStatus  string
		expectBackoff bool
	}{
		{"first failure", 0, "pending", true},
		{"second failure", 1, "pending", true},
		{"third failure", 2, "pending", true},
		{"fourth failure", 3, "pending", true},
		{"fifth failure", 4, "failed", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms.updatedEmails = nil
			e := &store.QueuedEmail{ID: 1, Attempts: tt.attempts}
			handleFailure(context.Background(), ms, e)

			if len(ms.updatedEmails) != 1 {
				t.Fatalf("expected 1 update, got %d", len(ms.updatedEmails))
			}
			if ms.updatedEmails[0].status != tt.expectStatus {
				t.Errorf("expected status %s, got %s", tt.expectStatus, ms.updatedEmails[0].status)
			}
			if tt.expectBackoff && ms.updatedEmails[0].nextAttempt == nil {
				t.Errorf("expected next attempt to be set")
			}
			if !tt.expectBackoff && ms.updatedEmails[0].nextAttempt != nil {
				t.Errorf("expected next attempt to be nil")
			}
		})
	}
}
