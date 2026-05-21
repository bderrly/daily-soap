package server

import (
	"log/slog"
	"net/http"

	"github.com/bderrly/daily-soap/internal/store"
)

// adminMiddleware checks if the user in the context is an administrator.
func adminMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := r.Context().Value(userContextKey).(*store.User)
		if !ok || user == nil || !user.IsAdmin {
			http.Error(w, "403 Forbidden", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// handleAdmin handles the admin directory and stats view.
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value(userContextKey).(*store.User)
	csrfToken := r.Context().Value(csrfContextKey).(string)
	nonce := r.Context().Value(nonceContextKey).(string)

	stats, err := appStore.GetAdminStats(r.Context())
	if err != nil {
		slog.Error("failed to get admin stats", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	directory, err := appStore.GetAdminUserDirectory(r.Context())
	if err != nil {
		slog.Error("failed to get admin user directory", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	data := map[string]any{
		"user":       user,
		"User":       user,
		"stats":      stats,
		"Stats":      stats,
		"directory":  directory,
		"Directory":  directory,
		"csrf_token": csrfToken,
		"CSRFToken":  csrfToken,
		"nonce":      nonce,
		"Nonce":      nonce,
	}

	if err := tmpl.ExecuteTemplate(w, "admin.html", data); err != nil {
		slog.Error("failed to execute admin template", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
