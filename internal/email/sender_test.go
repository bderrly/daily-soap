package email

import (
	"os"
	"sync"
	"testing"
)

func TestGetClientSenderFormatting(t *testing.T) {
	// Save original env vars
	origDomain := os.Getenv("MAILGUN_DOMAIN")
	origKey := os.Getenv("MAILGUN_API_KEY")
	origSender := os.Getenv("MAILGUN_SENDER")
	defer func() {
		_ = os.Setenv("MAILGUN_DOMAIN", origDomain)
		_ = os.Setenv("MAILGUN_API_KEY", origKey)
		_ = os.Setenv("MAILGUN_SENDER", origSender)
	}()

	if err := os.Setenv("MAILGUN_DOMAIN", "example.com"); err != nil {
		t.Fatalf("failed to set MAILGUN_DOMAIN: %v", err)
	}
	if err := os.Setenv("MAILGUN_API_KEY", "key-123"); err != nil {
		t.Fatalf("failed to set MAILGUN_API_KEY: %v", err)
	}
	if err := os.Setenv("MAILGUN_SENDER", "no-reply@example.com"); err != nil {
		t.Fatalf("failed to set MAILGUN_SENDER: %v", err)
	}

	// We need to reset the once and the client because it might have been initialized already
	defaultClient = nil
	clientOnce = sync.Once{}

	client, err := GetClient()
	if err != nil {
		t.Fatalf("GetClient failed: %v", err)
	}

	expectedSender := "My SOAP <no-reply@example.com>"
	if client.sender != expectedSender {
		t.Errorf("expected sender %q, got %q", expectedSender, client.sender)
	}
}
