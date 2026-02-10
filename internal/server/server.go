package server

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	_ "time/tzdata"

	"derrclan.com/moravian-soap/internal/cache_expunger"
	"derrclan.com/moravian-soap/internal/dailytexts"
	"derrclan.com/moravian-soap/internal/email"
	"derrclan.com/moravian-soap/internal/esv"
	"derrclan.com/moravian-soap/internal/migrations"
	"golang.org/x/crypto/bcrypt"

	_ "github.com/mattn/go-sqlite3"
)

var (
	tmpl *template.Template
	db   *sql.DB
)

//go:embed web
var web embed.FS

type contextKey string

const (
	userContextKey  contextKey = "user"
	csrfContextKey  contextKey = "csrf_token"
	nonceContextKey contextKey = "nonce"
)

type User struct {
	ID         int64
	Email      string
	IsVerified bool
	Timezone   string
}

func init() {
	// Parse templates with function map for safe HTML rendering
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
		"toJSON": func(v any) (template.JS, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return "", err
			}
			return template.JS(b), nil
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

// authMiddleware checks for a valid session cookie and sets the user in the context
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

		user, err := getUserFromSession(cookie.Value)
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

		user, err := authenticateUser(email, password)
		if err != nil {
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
			if _, err := db.Exec("UPDATE users SET timezone = ? WHERE id = ?", timezone, user.ID); err != nil {
				slog.Error("failed to update user timezone", "error", err, "user_id", user.ID)
			}
		}

		sessionToken, err := createSession(user.ID)
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

		if err := createUser(emailStr, password, token, timezone); err != nil {
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
			err = client.SendWelcomeEmail(emailStr, confirmationURL)
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

	result, err := db.Exec("UPDATE users SET is_verified = 1, verification_token = NULL WHERE verification_token = ?", token)
	if err != nil {
		slog.Error("failed to verify user", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
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
		var id int64
		err := db.QueryRow("SELECT id FROM users WHERE email = ?", emailStr).Scan(&id)
		if err == sql.ErrNoRows {
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
		_, err = db.Exec("INSERT INTO password_reset_tokens (token, user_id, expires_at) VALUES (?, ?, ?)", token, id, expiresAt)
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
			err = client.SendPasswordResetEmail(emailStr, resetURL)
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
		var userID int64
		var expiresAt time.Time
		err := db.QueryRow("SELECT user_id, expires_at FROM password_reset_tokens WHERE token = ?", token).Scan(&userID, &expiresAt)
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
		var userID int64
		var expiresAt time.Time
		err := db.QueryRow("SELECT user_id, expires_at FROM password_reset_tokens WHERE token = ?", token).Scan(&userID, &expiresAt)
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
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("failed to hash password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		_, err = db.Exec("UPDATE users SET password_hash = ? WHERE id = ?", string(hashedPassword), userID)
		if err != nil {
			slog.Error("failed to update password", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Delete used token
		db.Exec("DELETE FROM password_reset_tokens WHERE token = ?", token)

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
	user := r.Context().Value(userContextKey).(*User)

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
	verseContents, err := fetchPassagesWithCache(dailyText.Verses)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error loading verses for %s", today), http.StatusInternalServerError)
		return
	}

	// Load existing SOAP data from database
	soapData, err := getSOAPData(user.ID, today)
	if err != nil {
		slog.Warn("failed to load SOAP data", "date", today, "error", err)
		// Continue with empty values if there's an error
		soapData = &SOAPData{
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
		var loc *time.Location = time.UTC
		if user, ok := r.Context().Value(userContextKey).(*User); ok {
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
	verseContents, err := fetchPassagesWithCache(dailyText.Verses)
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
func InitDB() error {
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
	if err := migrations.Run(db); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database initialized successfully")

	// Start the cache expunger service
	cache_expunger.Start(db)

	return nil
}

// SOAPData represents the SOAP journal entry
type SOAPData struct {
	Date           string   `json:"date"`
	Observation    string   `json:"observation"`
	Application    string   `json:"application"`
	Prayer         string   `json:"prayer"`
	SelectedVerses []string `json:"selectedVerses"`
}

// handleSOAP handles GET and POST requests for SOAP data
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

// handleGetSOAP retrieves SOAP data for a given date
func handleGetSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*User)
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		dateStr = time.Now().Format(time.DateOnly)
	}

	soapData, err := getSOAPData(user.ID, dateStr)
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

// handlePostSOAP saves SOAP data
func handlePostSOAP(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*User)

	var soapData SOAPData
	if err := json.NewDecoder(r.Body).Decode(&soapData); err != nil {
		slog.Error("failed to decode SOAP data", "error", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	if err := saveSOAPData(user.ID, &soapData); err != nil {
		slog.Error("failed to save SOAP data", "error", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to save data"})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// getSOAPData retrieves SOAP data from the database for a given user and date
func getSOAPData(userID int64, dateStr string) (*SOAPData, error) {
	var soapData SOAPData
	var selectedVersesJSON sql.NullString
	soapData.Date = dateStr

	query := `SELECT observation, application, prayer, selected_verses FROM journal WHERE user_id = ? AND date = ?`
	err := db.QueryRow(query, userID, dateStr).Scan(&soapData.Observation, &soapData.Application, &soapData.Prayer, &selectedVersesJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			// No data found, return empty values
			soapData.SelectedVerses = []string{}
			return &soapData, nil
		}
		return nil, fmt.Errorf("failed to query SOAP data: %w", err)
	}

	// Parse selected verses from JSON
	if selectedVersesJSON.Valid && selectedVersesJSON.String != "" {
		if err := json.Unmarshal([]byte(selectedVersesJSON.String), &soapData.SelectedVerses); err != nil {
			slog.Warn("failed to unmarshal selected verses", "error", err)
			soapData.SelectedVerses = []string{}
		}
	} else {
		soapData.SelectedVerses = []string{}
	}

	return &soapData, nil
}

// saveSOAPData saves SOAP data to the database
func saveSOAPData(userID int64, soapData *SOAPData) error {
	// Encode selected verses as JSON
	selectedVersesJSON, err := json.Marshal(soapData.SelectedVerses)
	if err != nil {
		return fmt.Errorf("failed to marshal selected verses: %w", err)
	}

	query := `
		INSERT INTO journal (user_id, date, observation, application, prayer, selected_verses)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id, date) DO UPDATE SET
			observation = excluded.observation,
			application = excluded.application,
			prayer = excluded.prayer,
			selected_verses = excluded.selected_verses,
			timestamp = CURRENT_TIMESTAMP
	`
	_, err = db.Exec(query, userID, soapData.Date, soapData.Observation, soapData.Application, soapData.Prayer, selectedVersesJSON)
	if err != nil {
		return fmt.Errorf("failed to save SOAP data: %w", err)
	}
	return nil
}

// User registration and authentication helpers

func createUser(email, password, token, timezone string) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	if timezone == "" {
		timezone = "UTC"
	}

	_, err = db.Exec("INSERT INTO users (email, password_hash, is_verified, verification_token, timezone) VALUES (?, ?, 0, ?, ?)", email, string(hashedPassword), token, timezone)
	return err
}

func authenticateUser(email, password string) (*User, error) {
	var id int64
	var passwordHash string
	var isVerified bool
	var timezone string
	err := db.QueryRow("SELECT id, password_hash, is_verified, timezone FROM users WHERE email = ?", email).Scan(&id, &passwordHash, &isVerified, &timezone)
	if err != nil {
		return nil, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)); err != nil {
		return nil, err
	}

	if !isVerified {
		return nil, fmt.Errorf("email not verified")
	}

	return &User{ID: id, Email: email, IsVerified: isVerified, Timezone: timezone}, nil
}

func createSession(userID int64) (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := base64.URLEncoding.EncodeToString(b)

	// Clean up expired sessions first (optional, but good practice)
	// In a production app, do this in a background job
	db.Exec("DELETE FROM sessions WHERE expires_at < ?", time.Now())

	expiresAt := time.Now().Add(24 * time.Hour * 30) // 30 days
	_, err := db.Exec("INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)", token, userID, expiresAt)
	return token, err
}

func getUserFromSession(token string) (*User, error) {
	var user User
	var expiresAt time.Time

	query := `
		SELECT u.id, u.email, u.is_verified, u.timezone, s.expires_at 
		FROM sessions s 
		JOIN users u ON s.user_id = u.id 
		WHERE s.token = ?`

	err := db.QueryRow(query, token).Scan(&user.ID, &user.Email, &user.IsVerified, &user.Timezone, &expiresAt)
	if err != nil {
		return nil, err
	}

	if time.Now().After(expiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &user, nil
}

// fetchPassagesWithCache fetches verses from the cache or the ESV API
func fetchPassagesWithCache(references []string) (esv.EsvResponse, error) {
	key := strings.Join(references, ";")
	var response esv.EsvResponse
	var content string

	// 1. Check cache
	err := db.QueryRow("SELECT content FROM esv_cache WHERE reference = ?", key).Scan(&content)
	if err == nil {
		// Cache hit
		if err := json.Unmarshal([]byte(content), &response); err != nil {
			// If unmarshal fails, log it and fall back to fetch
			slog.Error("failed to unmarshal cached ESV response", "error", err)
		} else {
			slog.Debug("cache hit for verses", "reference", key)
			return response, nil
		}
	} else if err != sql.ErrNoRows {
		// Log DB error but proceed to fetch
		slog.Error("failed to query esv_cache", "error", err)
	}

	// 2. Fetch from API
	response, err = esv.FetchPassages(references)
	if err != nil {
		return response, err
	}

	// 3. Save to cache
	responseBytes, err := json.Marshal(response)
	if err != nil {
		slog.Error("failed to marshal ESV response for cache", "error", err)
		return response, nil // Return successful fetch even if cache save fails
	}

	// Use INSERT OR REPLACE to update if somehow exists (though we checked)
	_, err = db.Exec("INSERT OR REPLACE INTO esv_cache (reference, content) VALUES (?, ?)", key, string(responseBytes))
	if err != nil {
		slog.Error("failed to save to esv_cache", "error", err)
	} else {
		slog.Debug("saved verses to cache", "reference", key)
	}

	return response, nil
}
