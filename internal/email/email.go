package email

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/mailgun/mailgun-go/v5"
)

// Client holds the Mailgun client and sender configuration.
type Client struct {
	mg     mailgun.Mailgun
	sender string
	domain string
}

var (
	defaultClient *Client
	clientOnce    sync.Once
	clientErr     error
)

// GetClient returns the shared email client instance, initializing it if necessary.
func GetClient() (*Client, error) {
	clientOnce.Do(func() {
		domain := os.Getenv("MAILGUN_DOMAIN")
		apiKey := os.Getenv("MAILGUN_API_KEY")
		sender := os.Getenv("MAILGUN_SENDER")

		if domain == "" || apiKey == "" || sender == "" {
			clientErr = fmt.Errorf("mailgun configuration missing")
			return
		}

		mg := mailgun.NewMailgun(apiKey)

		defaultClient = &Client{
			mg:     mg,
			sender: sender,
			domain: domain,
		}
	})

	return defaultClient, clientErr
}

// send handles the actual email sending with exponential backoff retry logic.
func (c *Client) send(recipient, subject, htmlBody string) error {
	message := mailgun.NewMessage(c.domain, c.sender, subject, "")
	message.AddRecipient(recipient)
	message.SetHTML(htmlBody)

	var lastErr error
	maxRetries := 5
	backoff := time.Second

	for i := range maxRetries {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		_, lastErr = c.mg.Send(ctx, message)
		cancel()

		if lastErr == nil {
			return nil
		}

		if i < maxRetries-1 {
			time.Sleep(backoff)
			backoff *= 2
		}
	}

	return fmt.Errorf("failed to send email after %d attempts: %w", maxRetries, lastErr)
}

// SendWelcomeEmail sends a welcome email using the client instance.
func (c *Client) SendWelcomeEmail(recipientEmail, confirmationURL string) error {
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

	return c.send(recipientEmail, subject, body)
}

// SendPasswordResetEmail sends a password reset email using the client instance.
func (c *Client) SendPasswordResetEmail(recipientEmail, resetURL string) error {
	subject := "Reset Your Password - Daily SOAP Journal"
	body := fmt.Sprintf(`
<html>
<body>
	<h1>Password Reset Request</h1>
	<p>We received a request to reset your password for your Daily SOAP Journal account.</p>
	<p>Click the link below to reset your password:</p>
	<p><a href="%s">Reset Password</a></p>
	<p>Or copy and paste this link into your browser:</p>
	<p>%s</p>
	<p>This link will expire in 1 hour.</p>
	<p>If you didn't request this, you can safely ignore this email.</p>
</body>
</html>
`, resetURL, resetURL)

	return c.send(recipientEmail, subject, body)
}
