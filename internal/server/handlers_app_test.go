package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bderrly/daily-soap/internal/store"
)

type mockStore struct {
	store.Store
}

func (m *mockStore) GetSOAPData(_ context.Context, _ int64, dateStr string) (*store.SOAPData, error) {
	return &store.SOAPData{Date: dateStr}, nil
}

func (m *mockStore) GetCachedESV(_ context.Context, _ string) (string, error) {
	return `{"query": "test", "passages": ["<p>Mocked Verse</p>"]}`, nil
}

func (m *mockStore) SaveCachedESV(_ context.Context, _ string, _ string) error {
	return nil
}

func TestHandleIndex_DateQueryParam_Verification(t *testing.T) {
	// Setup global state
	oldAppStore := appStore
	appStore = &mockStore{}
	defer func() { appStore = oldAppStore }()

	user := &store.User{
		ID:       1,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")

	// Test Case 1: Specific date in the past
	req, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
	rr := httptest.NewRecorder()
	handleIndex(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Check if the response contains the requested date
	// The template renders date in various places, e.g., <input type="date" ... value="{{.date}}">
	if !strings.Contains(rr.Body.String(), "2026-05-07") {
		t.Errorf("expected response to contain '2026-05-07', but it didn't")
	}

	// Test Case 2: No date parameter (should be today)
	req2, _ := http.NewRequestWithContext(ctx, "GET", "/", nil)
	rr2 := httptest.NewRecorder()
	handleIndex(rr2, req2)

	if rr2.Code != http.StatusOK {
		t.Fatalf("expected status OK for today, got %d", rr2.Code)
	}

	if strings.Contains(rr2.Body.String(), "2026-05-07") {
		t.Errorf("expected response NOT to contain '2026-05-07' when no date provided")
	}

	// Test Case 3: HTMX request (should return content.gotmpl partial)
	req3, _ := http.NewRequestWithContext(ctx, "GET", "/", nil)
	req3.Header.Set("HX-Request", "true")
	rr3 := httptest.NewRecorder()
	handleIndex(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rr3.Code)
	}

	body := rr3.Body.String()
	// Ensure it does not render the full HTML layout boilerplate
	if strings.Contains(body, "<!DOCTYPE html>") || strings.Contains(body, "<head>") {
		t.Errorf("expected HTMX response NOT to contain HTML layout boilerplate, but it did")
	}
	// Ensure it renders the content partial container
	if !strings.Contains(body, `class="content-wrapper" id="content-container"`) {
		t.Errorf("expected HTMX response to contain content-container wrapper, but it didn't")
	}
}
