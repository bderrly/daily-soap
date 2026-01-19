package email

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/mailgun/mailgun-go/v5"
)

// SendWelcomeEmail sends a welcome email with a confirmation link.
func SendWelcomeEmail(recipientEmail, confirmationURL string) error {
	domain := os.Getenv("MAILGUN_DOMAIN")
	apiKey := os.Getenv("MAILGUN_API_KEY")
	sender := os.Getenv("MAILGUN_SENDER")

	if domain == "" || apiKey == "" || sender == "" {
		return fmt.Errorf("mailgun configuration missing")
	}

	mg := mailgun.NewMailgun(apiKey)

	subject := "Welcome to your Daily SOAP Journal - Please Confirm Your Email"
	body := fmt.Sprintf(`
<html>
<body>
	<h1>Welcome!</h1>
	<p>Thank you for registering for your Daily SOAP Journal.</p>
	<p>Please click the link below to confirm your email address and activate your account:</p>
	<p><a href="%s">Confirm Email</a></p>
	<p>Or copy and paste this link into your browser:</p>
	<p>%s</p>
	<p>This link will expire in 24 hours.</p>
</body>
</html>
`, confirmationURL, confirmationURL)

	message := mailgun.NewMessage(domain, sender, subject, "")
	message.AddRecipient(recipientEmail)
	message.SetHTML(body)

	var lastErr error
	maxRetries := 5
	backoff := time.Second

	for i := 0; i < maxRetries; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		_, lastErr = mg.Send(ctx, message)
		cancel()

		if lastErr == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return fmt.Errorf("failed to send welcome email after %d attempts: %w", maxRetries, lastErr)
}
