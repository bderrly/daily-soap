package server

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bderrly/daily-soap/internal/auth"
	"github.com/bderrly/daily-soap/internal/email"
	"github.com/bderrly/daily-soap/internal/store"
)

func (app *application) login(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		data := map[string]any{
			"IsLogin":   true,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			slog.Error("failed to render login page", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		emailStr := r.FormValue("email")
		password := r.FormValue("password")
		timezone := r.FormValue("timezone")

		user, err := app.authenticateUser(r.Context(), emailStr, password)
		if err != nil {
			slog.Error("authenticating user", "email", emailStr, "error", err)
			data := map[string]any{
				"IsLogin":   true,
				"Error":     "Invalid email or password",
				"Email":     emailStr,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render login page", "error", err)
			}
			return
		}

		// Update timezone if provided.
		if timezone != "" {
			if err := app.store.UpdateUserTimezone(r.Context(), user.ID, timezone); err != nil {
				slog.Error("failed to update user timezone", "error", err, "user_id", user.ID)
			}
		}

		sessionToken, err := app.createSession(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour * 30), // 30 days
		})

		redirect(w, r, "/", http.StatusSeeOther)
	}
}

func (app *application) register(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		data := map[string]any{
			"IsLogin":   false,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			slog.Error("failed to render register page", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		emailStr := r.FormValue("email")
		password := r.FormValue("password")
		timezone := r.FormValue("timezone")

		if emailStr == "" || password == "" {
			data := map[string]any{
				"IsLogin":   false,
				"Error":     "Email and password are required",
				"Email":     emailStr,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render register page", "error", err)
			}
			return
		}

		// Generate verification token.
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("failed to generate verification token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		token := base64.URLEncoding.EncodeToString(tokenBytes)

		if err := app.createUser(r.Context(), emailStr, password, token, timezone); err != nil {
			slog.Error("failed to create user", "error", err)
			data := map[string]any{
				"IsLogin":   false,
				"Error":     "Failed to create user. Email may already be in use.",
				"Email":     emailStr,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render register page", "error", err)
			}
			return
		}

		// Send welcome email.
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		confirmationURL := fmt.Sprintf("%s/confirm?token=%s", baseURL, token)

		client, err := email.GetClient()
		if err == nil {
			err = client.SendWelcomeEmail(r.Context(), emailStr, confirmationURL)
		}
		if err != nil {
			slog.Error("failed to send welcome email", "error", err)
			// User created but email failed. They can't login.
			// Ideally we'd rollback or have a "resend" option.
			// For now, show error.
			data := map[string]any{
				"IsLogin":   false,
				"Error":     "User created but failed to send verification email. Please contact support.",
				"Email":     emailStr,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render register page", "error", err)
			}
			return
		}

		// Show success message.
		data := map[string]any{
			"IsLogin":   true, // Switch to login view.
			"Success":   "Registration successful! Please check your email to confirm your account.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			slog.Error("failed to render login page", "error", err)
		}
	}
}

func (app *application) confirm(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Verification token missing from URL", http.StatusBadRequest)
		return
	}

	userID, emailStr, err := app.store.ConfirmUser(r.Context(), token)
	if err != nil {
		slog.Error("failed to verify user", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if userID == 0 {
		data := map[string]any{
			"IsLogin":   true,
			"Error":     "Invalid or expired verification token.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			slog.Error("failed to render login page", "error", err)
		}
		return
	}

	// Notify admin.
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail != "" {
		notification := &store.QueuedEmail{
			UserID:        userID,
			Recipient:     adminEmail,
			Subject:       "New User Registration: " + emailStr,
			BodyHTML:      "A new user has registered and verified their account: " + emailStr,
			Status:        "pending",
			Attempts:      0,
			NextAttemptAt: time.Now(),
		}
		if err := app.store.QueueEmail(r.Context(), notification); err != nil {
			slog.Error("failed to queue admin notification email", "error", err, "admin_email", adminEmail, "user_email", emailStr)
		}
	}

	data := map[string]any{
		"IsLogin":   true,
		"Success":   "Email verified! You can now log in.",
		"CSRFToken": csrfToken,
		"Nonce":     nonce,
	}
	if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
		slog.Error("failed to render login page", "error", err)
	}
}

func (app *application) forgotPassword(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		data := map[string]any{"CSRFToken": csrfToken, "Nonce": nonce}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/forgot_password.tmpl"); err != nil {
			slog.Error("failed to render forgot_password page", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		emailStr := r.FormValue("email")
		if emailStr == "" {
			data := map[string]any{
				"Error":     "Email is required",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/forgot_password.tmpl"); err != nil {
				slog.Error("failed to render forgot_password page", "error", err)
			}
			return
		}

		// Check if user exists (generic success message regardless).
		user, err := app.store.GetUserByEmail(r.Context(), emailStr)
		if errors.Is(err, sql.ErrNoRows) {
			// User not found - pretend we sent it.
			data := map[string]any{
				"Success":   "If an account exists for that email, a password reset link has been sent.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/forgot_password.tmpl"); err != nil {
				slog.Error("failed to render forgot_password page", "error", err)
			}
			return
		} else if err != nil {
			slog.Error("failed to query user for password reset", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Generate reset token.
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("failed to generate reset token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		token := base64.URLEncoding.EncodeToString(tokenBytes)
		expiresAt := time.Now().Add(1 * time.Hour)

		// Save token.
		err = app.store.CreatePasswordResetToken(r.Context(), token, user.ID, expiresAt)
		if err != nil {
			slog.Error("failed to save reset token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Send email.
		baseURL := os.Getenv("BASE_URL")
		if baseURL == "" {
			baseURL = "http://localhost:8080"
		}
		resetURL := fmt.Sprintf("%s/reset-password?token=%s", baseURL, token)

		client, err := email.GetClient()
		if err == nil {
			err = client.SendPasswordResetEmail(r.Context(), emailStr, resetURL)
		}
		if err != nil {
			slog.Error("failed to send password reset email", "error", err)
			// Log the link for dev/debug if email fails.
			slog.Debug("Password reset link", "url", resetURL, "email", emailStr)
			data := map[string]any{
				"Error":     "Failed to send email. Please try again later.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/forgot_password.tmpl"); err != nil {
				slog.Error("failed to render forgot_password page", "error", err)
			}
			return
		}

		data := map[string]any{
			"Success":   "If an account exists for that email, a password reset link has been sent.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/forgot_password.tmpl"); err != nil {
			slog.Error("failed to render forgot_password page", "error", err)
		}
	}
}

func (app *application) resetPassword(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Invalid token", http.StatusBadRequest)
			return
		}

		// Validate token.
		_, expiresAt, err := app.store.GetPasswordResetToken(r.Context(), token)
		if err != nil {
			data := map[string]any{
				"Error":     "Invalid or expired password reset link.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			// Just render login with error if token invalid.
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render login page", "error", err)
			}
			return
		}

		if time.Now().After(expiresAt) {
			data := map[string]any{
				"Error":     "Password reset link has expired.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render login page", "error", err)
			}
			return
		}

		data := map[string]any{
			"Token":     token,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/reset_password.tmpl"); err != nil {
			slog.Error("failed to render reset_password page", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		token := r.FormValue("token")
		password := r.FormValue("password")

		if token == "" || password == "" {
			http.Error(w, "Missing token or password", http.StatusBadRequest)
			return
		}

		// Validate token again.
		userID, expiresAt, err := app.store.GetPasswordResetToken(r.Context(), token)
		if err != nil || time.Now().After(expiresAt) {
			data := map[string]any{
				"Error":     "Invalid or expired password reset link.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
				slog.Error("failed to render login page", "error", err)
			}
			return
		}

		// Update password.
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		err = app.store.UpdateUserPassword(r.Context(), userID, hashedPassword)
		if err != nil {
			slog.Error("failed to update password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Delete used token.
		err = app.store.DeletePasswordResetToken(r.Context(), token)
		if err != nil {
			slog.Error("failed to delete password reset token", "error", err)
		}

		data := map[string]any{
			"IsLogin":   true,
			"Success":   "Password reset successfully! You can now log in.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := app.html.render(w, http.StatusOK, data, "base", "pages/login.tmpl"); err != nil {
			slog.Error("failed to render login page", "error", err)
		}
	}
}

func (app *application) logout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})
	redirect(w, r, "/login", http.StatusFound)
}
