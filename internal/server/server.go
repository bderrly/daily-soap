// Package server provides the core HTTP server and application logic.
package server

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/bderrly/daily-soap/assets"
	"github.com/bderrly/daily-soap/internal/email"
	"github.com/bderrly/daily-soap/internal/expunger"
	"github.com/bderrly/daily-soap/internal/migrations"
	"github.com/bderrly/daily-soap/internal/store"
	"github.com/bderrly/daily-soap/internal/store/sqlite"
)

type contextKey string

const (
	userContextKey  contextKey = "user"
	csrfContextKey  contextKey = "csrf_token"
	nonceContextKey contextKey = "nonce"
)

// application holds the dependencies needed for HTTP handlers.
type application struct {
	html  *htmlRenderer
	store store.Store
}

// NewApplication creates a new application with an initialized HTML renderer.
func NewApplication(s store.Store) (*application, error) {
	hr, err := newHTMLRenderer(assets.HTMLFiles, "base.tmpl", "partials/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("creating HTML renderer: %w", err)
	}

	return &application{
		html:  hr,
		store: s,
	}, nil
}

// Routes returns the HTTP handler for the application with all routes registered.
func (app *application) Routes() http.Handler {
	mux := http.NewServeMux()

	// Public routes
	mux.HandleFunc("/login", app.login)
	mux.HandleFunc("/register", app.register)
	mux.HandleFunc("/confirm", app.confirm)
	mux.HandleFunc("/forgot-password", app.forgotPassword)
	mux.HandleFunc("/reset-password", app.resetPassword)
	mux.HandleFunc("/logout", app.logout)

	// Protected routes
	mux.HandleFunc("/", app.authMiddleware(app.home))
	mux.HandleFunc("GET /soap", app.authMiddleware(app.getSoap))
	mux.HandleFunc("POST /soap", app.authMiddleware(app.postSoap))
	mux.HandleFunc("GET /export", app.authMiddleware(app.export))
	mux.HandleFunc("/history", app.authMiddleware(app.history))
	mux.HandleFunc("/admin", app.authMiddleware(adminMiddleware(app.admin)))

	// Static files served from the assets package.
	fileserver := http.FileServerFS(assets.StaticFiles)
	mux.Handle("/static/", http.StripPrefix("/static", fileserver))

	return securityMiddleware(csrfMiddleware(mux))
}

func securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nonce := generateRandomString(16)
		ctx := context.WithValue(r.Context(), nonceContextKey, nonce)

		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		// Content Security Policy with Nonce.
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

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
			// Safe method: Skip token verification.
		default:
			// Unsafe method (POST, PUT, DELETE, PATCH): Verify CSRF token.
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
func (app *application) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil {
			if r.URL.Path == "/" {
				redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		user, err := app.store.GetUserFromSession(r.Context(), cookie.Value)
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
				redirect(w, r, "/login", http.StatusFound)
				return
			}
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, user)
		next(w, r.WithContext(ctx))
	}
}

// InitDB initializes the SQLite database, applies migrations, and returns the
// initialized store.
func InitDB(ctx context.Context) (store.Store, error) {
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "/data/app.db"
	}

	// Parse the DSN to safely append query parameters.
	u, err := url.Parse(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database path: %w", err)
	}

	q := u.Query()
	q.Set("_foreign_keys", "on")
	u.RawQuery = q.Encode()

	db, err := sql.Open("sqlite3", u.String())
	if err != nil {
		return nil, fmt.Errorf("failed to open database at %s: %w", dbPath, err)
	}

	// Run migrations.
	if err := migrations.Run(ctx, db); err != nil {
		return nil, fmt.Errorf("failed to run migrations: %w", err)
	}

	slog.Info("database initialized successfully")

	// Initialize the store.
	s := sqlite.New(db)

	// Promote existing user matching ADMIN_EMAIL to admin if configured.
	adminEmail := strings.TrimSpace(os.Getenv("ADMIN_EMAIL"))
	if adminEmail != "" {
		rows, err := s.PromoteUserToAdmin(ctx, adminEmail)
		if err != nil {
			return nil, fmt.Errorf("failed to promote admin user %q: %w", adminEmail, err)
		}
		if rows > 0 {
			slog.Info("successfully promoted existing user to admin", "admin_email", adminEmail)
		} else {
			slog.Info("checked admin user; no promotion needed (user not found or already admin)", "admin_email", adminEmail)
		}
	}

	// Start the cache expunger service.
	expunger.Start(ctx, s)

	// Start email background worker.
	emailClient, err := email.GetClient()
	if err == nil {
		go email.StartWorker(ctx, s, emailClient)
	} else {
		slog.Warn("email worker not started due to missing configuration", "error", err)
	}

	return s, nil
}
