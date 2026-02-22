// Package server provides the core HTTP server and application logic.
package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // Initialize timezone data

	"derrclan.com/moravian-soap/internal/auth"
	"derrclan.com/moravian-soap/internal/dailytexts"
	"derrclan.com/moravian-soap/internal/email"
	"derrclan.com/moravian-soap/internal/esv"
	"derrclan.com/moravian-soap/internal/expunger"
	"derrclan.com/moravian-soap/internal/migrations"
	"derrclan.com/moravian-soap/internal/store"
	"derrclan.com/moravian-soap/internal/store/sqlite"

	_ "github.com/mattn/go-sqlite3" // SQLite driver
)

var (
	tmpl     *template.Template
	db       *sql.DB
	appStore store.Store
)

//go:embed web
var web embed.FS

type contextKey string

const (
	userContextKey  contextKey = "user"
	csrfContextKey  contextKey = "csrf_token"
	nonceContextKey contextKey = "nonce"
)

func init() {
	// Parse templates with function map for safe HTML rendering
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s) // #nosec G203
		},
		"toJSON": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", fmt.Errorf("marshaling JSON: %w", err)
			}
			return template.JS(b), nil // #nosec G203
		},
	}
	var err error
	tmpl, err = template.New("").Funcs(funcMap).ParseFS(web, "web/*.html", "web/*.gotmpl")
	if err != nil {
		slog.Error("failed to parse template", "error", err)
		// Create a minimal template to prevent nil pointer errors
		tmpl = template.Must(template.New("error").Parse("<html><body><h1>Template Error</h1></body></html>"))
	}
}

// Muxer returns the HTTP handler for the application.
func Muxer() http.Handler {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/login", handleLogin)
	mux.HandleFunc("/register", handleRegister)
	mux.HandleFunc("/confirm", handleConfirm)
	mux.HandleFunc("/forgot-password", handleForgotPassword)
	mux.HandleFunc("/reset-password", handleResetPassword)
	mux.HandleFunc("/logout", handleLogout)

	// Protected routes
	mux.HandleFunc("/", authMiddleware(handleIndex))
	mux.HandleFunc("/reading", authMiddleware(handleReading))
	mux.HandleFunc("/soap", authMiddleware(handleSOAP))

	// Create a subdirectory filesystem for the web directory
	webFS, err := fs.Sub(web, "web")
	if err != nil {
		slog.Error("failed to create web subdirectory filesystem", "error", err)
	} else {
		mux.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.FS(webFS))))
	}

	return securityMiddleware(csrfMiddleware(mux))
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateRandomString(16)
		ctx := context.WithValue(r.Context(), nonceContextKey, nonce)

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy with Nonce
		csp := fmt.Sprintf("default-src 'self'; script-src 'self' 'nonce-%s'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; frame-ancestors 'none'; upgrade-insecure-requests;", nonce)
		w.Header().Set("Content-Security-Policy", csp)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var token string
		cookie, err := r.Cookie("csrf_token")
		if err != nil {
			token = generateRandomString(32)
			http.SetCookie(w, &http.Cookie{
				Name:     "csrf_token",
				Value:    token,
				Path:     "/",
				HttpOnly: true,
				Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
				SameSite: http.SameSiteLaxMode,
			})
		} else {
			token = cookie.Value
		}

		if r.Method == http.MethodPost {
			requestToken := r.Header.Get("X-CSRF-Token")
			if requestToken == "" {
				requestToken = r.FormValue("csrf_token")
			}

			if requestToken == "" || requestToken != token {
				slog.Warn("invalid CSRF token", "method", r.Method, "path", r.URL.Path)
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
		}

		ctx := context.WithValue(r.Context(), csrfContextKey, token)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func generateRandomString(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		slog.Error("failed to generate random string", "error", err)
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// authMiddleware checks for a valid session cookie and sets the user in the context.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := appStore.GetUserFromSession(r.Context(), cookie.Value)
		if err != nil {
			// Invalid session
			http.SetCookie(w, &http.Cookie{
				Name:     "session_token",
				Value:    "",
				Path:     "/",
				MaxAge:   -1,
				HttpOnly: true,
				Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
				SameSite: http.SameSiteLaxMode,
			})
			if r.URL.Path == "/" {
				http.Redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		data := map[string]any{
			"IsLogin":   true,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("failed to execute login template", "error", err)
		}
		return
	}

	if r.Method == http.MethodPost {
		email := r.FormValue("email")
		password := r.FormValue("password")
		timezone := r.FormValue("timezone")

		user, err := authenticateUser(r.Context(), email, password)
		if err != nil {
			slog.Error("authenticating user", "email", email, "error", err)
			data := map[string]any{
				"IsLogin":   true,
				"Error":     "Invalid email or password",
				"Email":     email,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute login template", "error", err)
			}
			return
		}

		// Update timezone if provided
		if timezone != "" {
			if err := appStore.UpdateUserTimezone(r.Context(), user.ID, timezone); err != nil {
				slog.Error("failed to update user timezone", "error", err, "user_id", user.ID)
			}
		}

		sessionToken, err := createSession(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Path:     "/",
			HttpOnly: true,
			Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().Add(24 * time.Hour * 30), // 30 days
		})

		http.Redirect(w, r, "/", http.StatusFound)
	}
}

func handleRegister(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		data := map[string]any{
			"IsLogin":   false,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("failed to execute register template", "error", err)
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
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute register template", "error", err)
			}
			return
		}

		// Generate verification token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("failed to generate verification token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		token := base64.URLEncoding.EncodeToString(tokenBytes)

		if err := createUser(r.Context(), emailStr, password, token, timezone); err != nil {
			slog.Error("failed to create user", "error", err)
			data := map[string]any{
				"IsLogin":   false,
				"Error":     "Failed to create user. Email may already be in use.",
				"Email":     emailStr,
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute register template", "error", err)
			}
			return
		}

		// Send welcome email
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
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute register template", "error", err)
			}
			return
		}

		// Show success message
		data := map[string]any{
			"IsLogin":   true, // Switch to login view
			"Success":   "Registration successful! Please check your email to confirm your account.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("failed to execute login template", "error", err)
		}
	}
}

func handleConfirm(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Verification token missing from URL", http.StatusBadRequest)
		return
	}

	rowsAffected, err := appStore.ConfirmUser(r.Context(), token)
	if err != nil {
		slog.Error("failed to verify user", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if rowsAffected == 0 {
		data := map[string]any{
			"IsLogin":   true,
			"Error":     "Invalid or expired verification token.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("failed to execute login template", "error", err)
		}
		return
	}

	data := map[string]any{
		"IsLogin":   true,
		"Success":   "Email verified! You can now log in.",
		"CSRFToken": csrfToken,
		"Nonce":     nonce,
	}
	if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
		slog.Error("failed to execute login template", "error", err)
	}
}

func handleForgotPassword(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		if err := tmpl.ExecuteTemplate(w, "forgot_password.html", map[string]any{"CSRFToken": csrfToken, "Nonce": nonce}); err != nil {
			slog.Error("failed to execute forgot_password template", "error", err)
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
			if err := tmpl.ExecuteTemplate(w, "forgot_password.html", data); err != nil {
				slog.Error("failed to execute forgot_password template", "error", err)
			}
			return
		}

		// Check if user exists (generic success message regardless)
		user, err := appStore.GetUserByEmail(r.Context(), emailStr)
		if errors.Is(err, sql.ErrNoRows) {
			// User not found - pretend we sent it
			data := map[string]any{
				"Success":   "If an account exists for that email, a password reset link has been sent.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "forgot_password.html", data); err != nil {
				slog.Error("failed to execute forgot_password template", "error", err)
			}
			return
		} else if err != nil {
			slog.Error("failed to query user for password reset", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Generate reset token
		tokenBytes := make([]byte, 32)
		if _, err := rand.Read(tokenBytes); err != nil {
			slog.Error("failed to generate reset token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		token := base64.URLEncoding.EncodeToString(tokenBytes)
		expiresAt := time.Now().Add(1 * time.Hour)

		// Save token
		err = appStore.CreatePasswordResetToken(r.Context(), token, user.ID, expiresAt)
		if err != nil {
			slog.Error("failed to save reset token", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Send email
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
			// Log the link for dev/debug if email fails
			slog.Debug("Password reset link", "url", resetURL, "email", emailStr)
			data := map[string]any{
				"Error":     "Failed to send email. Please try again later.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "forgot_password.html", data); err != nil {
				slog.Error("failed to execute forgot_password template", "error", err)
			}
			return
		}

		data := map[string]any{
			"Success":   "If an account exists for that email, a password reset link has been sent.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "forgot_password.html", data); err != nil {
			slog.Error("failed to execute forgot_password template", "error", err)
		}
	}
}

func handleResetPassword(w http.ResponseWriter, r *http.Request) {
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)
	if r.Method == http.MethodGet {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "Invalid token", http.StatusBadRequest)
			return
		}

		// Validate token
		_, expiresAt, err := appStore.GetPasswordResetToken(r.Context(), token)
		if err != nil {
			data := map[string]any{
				"Error":     "Invalid or expired password reset link.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			// Just render login with error if token invalid
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute login template", "error", err)
			}
			return
		}

		if time.Now().After(expiresAt) {
			data := map[string]any{
				"Error":     "Password reset link has expired.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute login template", "error", err)
			}
			return
		}

		data := map[string]any{
			"Token":     token,
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "reset_password.html", data); err != nil {
			slog.Error("failed to execute reset_password template", "error", err)
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

		// Validate token again
		userID, expiresAt, err := appStore.GetPasswordResetToken(r.Context(), token)
		if err != nil || time.Now().After(expiresAt) {
			data := map[string]any{
				"Error":     "Invalid or expired password reset link.",
				"CSRFToken": csrfToken,
				"Nonce":     nonce,
			}
			if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
				slog.Error("failed to execute login template", "error", err)
			}
			return
		}

		// Update password
		hashedPassword, err := auth.HashPassword(password)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		err = appStore.UpdateUserPassword(r.Context(), userID, hashedPassword)
		if err != nil {
			slog.Error("failed to update password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Delete used token
		err = appStore.DeletePasswordResetToken(r.Context(), token)
		if err != nil {
			slog.Error("failed to delete password reset token", "error", err)
		}

		data := map[string]any{
			"IsLogin":   true,
			"Success":   "Password reset successfully! You can now log in.",
			"CSRFToken": csrfToken,
			"Nonce":     nonce,
		}
		if err := tmpl.ExecuteTemplate(w, "login.html", data); err != nil {
			slog.Error("failed to execute login template", "error", err)
		}
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https",
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, "/login", http.StatusFound)
}

func handleIndex(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	// Only handle root path
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Get current date in YYYY-MM-DD format based on user location
	loc, err := time.LoadLocation(user.Timezone)
	if err != nil {
		slog.Error("failed to load user location", "timezone", user.Timezone, "error", err)
		loc = time.UTC
	}
	today := time.Now().In(loc).Format(time.DateOnly)

	// Get today's data (will load year file if needed)
	dailyText, err := dailytexts.GetDailyText(today)
	if err != nil {
		slog.Error("failed to get daily text", "date", today, "error", err)
		http.Error(w, fmt.Sprintf("Error loading data for date: %s", today), http.StatusInternalServerError)
		return
	}

	if dailyText == nil {
		slog.Warn("no data found for date", "date", today)
		http.Error(w, fmt.Sprintf("No data found for date: %s", today), http.StatusNotFound)
		return
	}

	// Fetch verse content from ESV API (using cache)
	verseContents, err := fetchPassagesWithCache(r.Context(), dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading verses for %s", today), http.StatusInternalServerError)
		return
	}

	// Load existing SOAP data from database
	soapData, err := appStore.GetSOAPData(r.Context(), user.ID, today)
	if err != nil {
		slog.Warn("failed to load SOAP data", "date", today, "error", err)
		// Continue with empty values if there's an error
		soapData = &store.SOAPData{
			Date:           today,
			Observation:    "",
			Application:    "",
			Prayer:         "",
			SelectedVerses: []string{},
		}
	}

	// Prepare template data
	data := map[string]any{
		"esvData":        verseContents,
		"date":           today,
		"observation":    soapData.Observation,
		"application":    soapData.Application,
		"prayer":         soapData.Prayer,
		"selectedVerses": soapData.SelectedVerses,
		"user":           user,
		"CSRFToken":      r.Context().Value(csrfContextKey).(string),
		"Nonce":          r.Context().Value(nonceContextKey).(string),
	}

	// Execute template
	if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		slog.Error("failed to execute template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleReading handles requests for the verses partial template (for HTMX).
// Accepts a "date" query parameter (YYYY-MM-DD format). Defaults to today if not provided.
func handleReading(w http.ResponseWriter, r *http.Request) {
	// Get date from query parameter, default to today
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		// Use user's timezone for default date
		loc := time.UTC
		if user, ok := r.Context().Value(userContextKey).(*store.User); ok {
			if l, err := time.LoadLocation(user.Timezone); err == nil {
				loc = l
			}
		}
		dateStr = time.Now().In(loc).Format(time.DateOnly)
	}

	// Get daily text for the requested date
	dailyText, err := dailytexts.GetDailyText(dateStr)
	if err != nil {
		slog.Error("failed to get daily text", "date", dateStr, "error", err)
		http.Error(w, fmt.Sprintf("Error loading data for date: %s", dateStr), http.StatusInternalServerError)
		return
	}

	if dailyText == nil {
		slog.Warn("no data found for date", "date", dateStr)
		http.Error(w, fmt.Sprintf("No data found for date: %s", dateStr), http.StatusNotFound)
		return
	}

	// Fetch verse content from ESV API (using cache)
	verseContents, err := fetchPassagesWithCache(r.Context(), dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error fetching verses for %s", dateStr), http.StatusInternalServerError)
		return
	}

	// Prepare template data
	data := map[string]any{
		"esvData": verseContents,
		"date":    dateStr,
	}

	// Execute only the verses template
	if err := tmpl.ExecuteTemplate(w, "verses.gotmpl", data); err != nil {
		slog.Error("failed to execute verses template", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// InitDB initializes the SQLite database and applies migrations.
func InitDB(ctx context.Context) error {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/app.db"
	}

	// Parse the DSN to safely append query parameters
	u, err := url.Parse(dbPath)
	if err != nil {
		return fmt.Errorf("failed to parse database path: %w", err)
	}

	q := u.Query()
	q.Set("_foreign_keys", "on")
	u.RawQuery = q.Encode()

	db, err = sql.Open("sqlite3", u.String())
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	// Run migrations
	if err := migrations.Run(ctx, db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database initialized successfully")

	// Initialize the store
	appStore = sqlite.New(db)

	// Start the cache expunger service
	expunger.Start(ctx, appStore)

	return nil
}

// handleSOAP handles GET and POST requests for SOAP data.
func handleSOAP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		handleGetSOAP(w, r)
	case http.MethodPost:
		handlePostSOAP(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetSOAP retrieves SOAP data for a given date.
func handleGetSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(time.DateOnly)
	}

	soapData, err := appStore.GetSOAPData(r.Context(), user.ID, dateStr)
	if err != nil {
		slog.Error("failed to get SOAP data", "date", dateStr, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(soapData); err != nil {
		slog.Error("failed to encode SOAP data", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handlePostSOAP saves SOAP data.
func handlePostSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)

	var soapData store.SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := appStore.SaveSOAPData(r.Context(), user.ID, &soapData); err != nil {
		slog.Error("failed to save SOAP data", "error", err)
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save data"}); err != nil {
			slog.Error("failed to encode error response", "error", err)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "success"}); err != nil {
		slog.Error("failed to encode success response", "error", err)
	}
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
				slog.Error("failed to migrate password hash", "user_id", id, "error", err)
			}
		} else {
			slog.Error("failed to generate new hash for migration", "user_id", id, "error", err)
		}
	}

	if !isVerified {
		return nil, fmt.Errorf("email not verified")
	}

	return &store.User{ID: id, Email: email, IsVerified: isVerified, Timezone: timezone}, nil
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

// fetchPassagesWithCache fetches verses from the cache or the ESV API.
func fetchPassagesWithCache(ctx context.Context, references []string) (esv.Response, error) {
	key := strings.Join(references, ";")
	var response esv.Response

	// 1. Check cache
	content, err := appStore.GetCachedESV(ctx, key)
	if err == nil {
		// Cache hit
		if err := json.Unmarshal([]byte(content), &response); err != nil {
			// If unmarshal fails, log it and fall back to fetch
			slog.Error("failed to unmarshal cached ESV response", "error", err)
		} else {
			slog.Debug("cache hit for verses", "reference", key)
			return response, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
		// Log DB error but proceed to fetch
		slog.Error("failed to query esv_cache", "error", err)
	}

	// 2. Fetch from API
	response, err = esv.FetchPassages(ctx, references)
	if err != nil {
		return response, fmt.Errorf("fetching passages %v from ESV: %w", references, err)
	}

	// 3. Save to cache
	responseBytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("failed to marshal ESV response for cache", "error", err)
		return response, nil // Return successful fetch even if cache save fails
	}

	err = appStore.SaveCachedESV(ctx, key, string(responseBytes))
	if err != nil {
		slog.Error("failed to save to esv_cache", "error", err)
	} else {
		slog.Debug("saved verses to cache", "reference", key)
	}

	return response, nil
}
