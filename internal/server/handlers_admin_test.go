package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bderrly/daily-soap/internal/store"
)

type mockAdminStore struct {
	store.Store
}

func (m *mockAdminStore) GetAdminStats(_ context.Context) (*store.AdminStats, error) {
	return &store.AdminStats{
		TotalUsers: 5,
	}, nil
}

func (m *mockAdminStore) GetAdminUserDirectory(_ context.Context) ([]*store.AdminUserDirEntry, error) {
	return []*store.AdminUserDirEntry{}, nil
}

func TestAdminMiddleware(t *testing.T) {
	tests := []struct {
		name       string
		user       *store.User
		wantStatus int
	}{
		{
			name:       "no user in context",
			user:       nil,
			wantStatus: http.StatusForbidden,
		},
		{
			name: "standard user",
			user: &store.User{
				ID:      1,
				IsAdmin: false,
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name: "admin user",
			user: &store.User{
				ID:      2,
				IsAdmin: true,
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})

			handlerToTest := adminMiddleware(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/admin", nil)
			if tt.user != nil {
				ctx := context.WithValue(req.Context(), userContextKey, tt.user)
				req = req.WithContext(ctx)
			}

			rr := httptest.NewRecorder()
			handlerToTest.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatus {
				t.Errorf("adminMiddleware() status = %v, want %v", rr.Code, tt.wantStatus)
			}
		})
	}
}

func TestHandleAdmin(t *testing.T) {
	// Setup mock store
	origStore := appStore
	appStore = &mockAdminStore{}
	defer func() { appStore = origStore }()

	req := httptest.NewRequest(http.MethodGet, "/admin", nil)
	user := &store.User{ID: 1, IsAdmin: true}
	ctx := context.WithValue(req.Context(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()

	handleAdmin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("handleAdmin() returned unexpected status: %v", rr.Code)
	}
}
