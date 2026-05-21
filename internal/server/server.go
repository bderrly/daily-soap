// Package server provides the core HTTP server and application logic.
package server

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/bderrly/daily-soap/internal/email"
	"github.com/bderrly/daily-soap/internal/expunger"
	"github.com/bderrly/daily-soap/internal/migrations"
	"github.com/bderrly/daily-soap/internal/store"
	"github.com/bderrly/daily-soap/internal/store/sqlite"
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
	mux.HandleFunc("/export", authMiddleware(handleExport))
	mux.HandleFunc("/history", authMiddleware(handleHistory))

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
				Secure:   true,
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
				Secure:   true,
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

	// Start email background worker
	emailClient, err := email.GetClient()
	if err == nil {
		go email.StartWorker(ctx, appStore, emailClient)
	} else {
		slog.Warn("email worker not started due to missing configuration", "error", err)
	}

	return nil
}
