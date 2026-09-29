package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bderrly/daily-soap/internal/store"
)

type mockStore struct {
	store.Store
	savedSOAPData    *store.SOAPData
	soapDataToReturn *store.SOAPData
}

func (m *mockStore) GetSOAPData(_ context.Context, _ int64, dateStr string) (*store.SOAPData, error) {
	if m.soapDataToReturn != nil {
		return m.soapDataToReturn, nil
	}
	return &store.SOAPData{Date: dateStr, Translation: ""}, nil
}

func (m *mockStore) SaveSOAPData(_ context.Context, _ int64, data *store.SOAPData) error {
	m.savedSOAPData = data
	return nil
}

func (m *mockStore) GetCachedESV(_ context.Context, _ string) (string, error) {
	return `{"query": "test", "passages": ["<p>Mocked Verse</p>"]}`, nil
}

func (m *mockStore) SaveCachedESV(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockStore) GetCachedScripture(_ context.Context, _ string) (string, error) {
	return `{"query": "test", "passages": ["<p>Mocked Verse</p>"]}`, nil
}

func (m *mockStore) SaveCachedScripture(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockStore) UpdateUserTranslation(_ context.Context, _ int64, _ string) error {
	return nil
}

func (m *mockStore) UpdateJournalTranslation(_ context.Context, _ int64, _, trans string) error {
	if m.soapDataToReturn != nil {
		m.soapDataToReturn.Translation = trans
	}
	return nil
}

func (m *mockStore) GetSOAPDatesWithEntries(_ context.Context, _ int64, startDate string, endDate string) ([]string, error) {
	if startDate == "2026-02-01" && endDate == "2026-08-31" {
		return []string{"2026-05-01", "2026-05-02"}, nil
	}
	return []string{}, nil
}

func TestHandleIndex_DateQueryParam_Verification(t *testing.T) {
	app := newTestApplication(t, &mockStore{})

	user := &store.User{
		ID:       1,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")

	// Test Case 1: Specific date in the past.
	req, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
	rr := httptest.NewRecorder()
	app.home(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	// Check if the response contains the requested date.
	if !strings.Contains(rr.Body.String(), "2026-05-07") {
		t.Errorf("expected response to contain '2026-05-07', but it didn't")
	}

	// Test Case 2: No date parameter (should redirect to today).
	req2, _ := http.NewRequestWithContext(ctx, "GET", "/", nil)
	rr2 := httptest.NewRecorder()
	app.home(rr2, req2)

	if rr2.Code != http.StatusFound {
		t.Fatalf("expected status Found, got %d", rr2.Code)
	}

	locHeader := rr2.Header().Get("Location")
	expectedLoc := "/?date=" + time.Now().In(time.UTC).Format(time.DateOnly)
	if locHeader != expectedLoc {
		t.Errorf("expected location header '%s', got '%s'", expectedLoc, locHeader)
	}

	// Test Case 3: HTMX request (should return home:content partial).
	req3, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
	req3.Header.Set("HX-Request", "true")
	rr3 := httptest.NewRecorder()
	app.home(rr3, req3)

	if rr3.Code != http.StatusOK {
		t.Fatalf("expected status OK, got %d", rr3.Code)
	}

	body := rr3.Body.String()
	// Ensure it does not render the full HTML layout boilerplate.
	if strings.Contains(body, "<!doctype html>") || strings.Contains(body, "<head>") {
		t.Errorf("expected HTMX response NOT to contain HTML layout boilerplate, but it did")
	}
	// Ensure it renders the content partial container.
	if !strings.Contains(body, `id="content-container"`) {
		t.Errorf("expected HTMX response to contain content-container, but it didn't")
	}
}

func TestHandleIndex_SelectedVersesReferenceStyle(t *testing.T) {
	user := &store.User{
		ID:       1,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")

	t.Run("with selected verses", func(t *testing.T) {
		mock := &mockStore{
			soapDataToReturn: &store.SOAPData{
				Date:           "2026-05-07",
				SelectedVerses: []string{"19001001"},
			},
		}
		app := newTestApplication(t, mock)

		req, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
		req.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		app.home(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d", rr.Code)
		}
		body := rr.Body.String()
		expected := `<div class="selected-verses-reference" id="selectedVersesReference" style="display: block;"></div>`
		if !strings.Contains(body, expected) {
			t.Errorf("expected body to contain %q, but got %s", expected, body)
		}
	})

	t.Run("without selected verses", func(t *testing.T) {
		mock := &mockStore{
			soapDataToReturn: &store.SOAPData{
				Date:           "2026-05-07",
				SelectedVerses: []string{},
			},
		}
		app := newTestApplication(t, mock)

		req, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
		req.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		app.home(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d", rr.Code)
		}
		body := rr.Body.String()
		expected := `<div class="selected-verses-reference" id="selectedVersesReference"></div>`
		if !strings.Contains(body, expected) {
			t.Errorf("expected body to contain %q, but got %s", expected, body)
		}
	})
}

func (m *mockStore) GetSOAPDataRange(_ context.Context, _ int64, _, _ string) ([]*store.SOAPData, error) {
	return []*store.SOAPData{}, nil
}

func TestGetSoapDates(t *testing.T) {
	app := newTestApplication(t, &mockStore{})

	user := &store.User{
		ID:       1,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	t.Run("by month query param", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "/soap/dates?month=2026-05", nil)
		rr := httptest.NewRecorder()
		app.getSoapDates(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		expected := `{"dates":["2026-05-01","2026-05-02"]}`
		if strings.TrimSpace(rr.Body.String()) != expected {
			t.Errorf("expected %s, got %s", expected, strings.TrimSpace(rr.Body.String()))
		}
	})

	t.Run("by date query param", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "/soap/dates?date=2026-05-15", nil)
		rr := httptest.NewRecorder()
		app.getSoapDates(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d. Body: %s", rr.Code, rr.Body.String())
		}
		expected := `{"dates":["2026-05-01","2026-05-02"]}`
		if strings.TrimSpace(rr.Body.String()) != expected {
			t.Errorf("expected %s, got %s", expected, strings.TrimSpace(rr.Body.String()))
		}
	})
}

func TestHeaderNavigationAndTitleLinks(t *testing.T) {
	app := newTestApplication(t, &mockStore{})

	user := &store.User{
		ID:       1,
		Email:    "test@example.com",
		Timezone: "UTC",
	}
	ctx := context.WithValue(context.Background(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")

	t.Run("home page header navigation and title link", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "/?date=2026-05-07", nil)
		rr := httptest.NewRecorder()
		app.home(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d", rr.Code)
		}

		body := rr.Body.String()
		expectedTitleLink := `<h1 class="header-title"><a href="/">Daily Reading + SOAP</a></h1>`
		if !strings.Contains(body, expectedTitleLink) {
			t.Errorf("expected body to contain title link %q, but got: %s", expectedTitleLink, body)
		}

		expectedNav := `<a href="/history" class="nav-link history-btn">History</a>`
		if !strings.Contains(body, expectedNav) {
			t.Errorf("expected home page to contain nav link %q, but got: %s", expectedNav, body)
		}
	})

	t.Run("history page header navigation and title link", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "GET", "/history", nil)
		rr := httptest.NewRecorder()
		app.history(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status OK, got %d", rr.Code)
		}

		body := rr.Body.String()
		expectedTitleLink := `<h1 class="header-title"><a href="/">Daily Reading + SOAP</a></h1>`
		if !strings.Contains(body, expectedTitleLink) {
			t.Errorf("expected body to contain title link %q, but got: %s", expectedTitleLink, body)
		}

		expectedNav := `<a href="/" class="nav-link history-btn">Back to Today</a>`
		if !strings.Contains(body, expectedNav) {
			t.Errorf("expected history page to contain nav link %q, but got: %s", expectedNav, body)
		}

		// Verify that "Previous" and "Next" navigation buttons appear twice (top and bottom).
		prevCount := strings.Count(body, "Previous 7 Days")
		if prevCount != 2 {
			t.Errorf("expected 2 'Previous 7 Days' occurrences (top and bottom), got %d", prevCount)
		}

		nextCount := strings.Count(body, "Next 7 Days")
		if nextCount != 2 {
			t.Errorf("expected 2 'Next 7 Days' occurrences (top and bottom), got %d", nextCount)
		}
	})
}

func TestPostTranslation(t *testing.T) {
	app := newTestApplication(t, &mockStore{})

	user := &store.User{
		ID:          1,
		Email:       "user@example.com",
		Timezone:    "UTC",
		Translation: "ESV",
	}

	ctx := context.WithValue(context.Background(), userContextKey, user)
	ctx = context.WithValue(ctx, csrfContextKey, "test-csrf")
	ctx = context.WithValue(ctx, nonceContextKey, "test-nonce")

	t.Run("valid translation update", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "POST", "/translation?date=2026-01-01&translation=MSG", nil)
		rr := httptest.NewRecorder()
		app.postTranslation(rr, req)

		if rr.Code != http.StatusFound {
			t.Errorf("expected status 302 redirect, got %d", rr.Code)
		}
		if user.Translation != "MSG" {
			t.Errorf("expected user translation to be updated to MSG, got %s", user.Translation)
		}

		reqNLT, _ := http.NewRequestWithContext(ctx, "POST", "/translation?date=2026-01-01&translation=d6e14a625393b4da-01", nil)
		rrNLT := httptest.NewRecorder()
		app.postTranslation(rrNLT, reqNLT)

		if user.Translation != "NLT" {
			t.Errorf("expected user translation to be updated to NLT, got %s", user.Translation)
		}
	})

	t.Run("htmx translation update", func(t *testing.T) {
		fakeRef := "NLT:Psalm 1;Genesis 1:1–2:3;Matthew 1:1–17"
		_ = app.store.SaveCachedScripture(ctx, fakeRef, `{"passages":["<p>Test NLT</p>"]}`)

		req, _ := http.NewRequestWithContext(ctx, "POST", "/translation?date=2026-01-01&translation=NLT", nil)
		req.Header.Set("HX-Request", "true")
		rr := httptest.NewRecorder()
		app.postTranslation(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200 OK for HTMX translation update, got %d", rr.Code)
		}
		body := rr.Body.String()
		if !strings.Contains(body, `id="content-container"`) {
			t.Errorf("expected rendered content container in HTMX response, got: %s", body)
		}
		if !strings.Contains(body, `data-translation="NLT"`) {
			t.Errorf("expected data-translation='NLT' in HTMX response, got: %s", body)
		}
		if !strings.Contains(body, `<p>Mocked Verse</p>`) {
			t.Errorf("expected rendered passage in HTMX response, got: %s", body)
		}
	})

	t.Run("invalid translation", func(t *testing.T) {
		req, _ := http.NewRequestWithContext(ctx, "POST", "/translation?translation=KJV", nil)
		rr := httptest.NewRecorder()
		app.postTranslation(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 bad request, got %d", rr.Code)
		}
	})
}

func TestPostSoap_Translation(t *testing.T) {
	ms := &mockStore{}
	app := newTestApplication(t, ms)
	user := &store.User{ID: 1, Email: "test@example.com", Translation: "ESV"}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	t.Run("explicit translation saved", func(t *testing.T) {
		body := `{"date":"2026-05-07","observation":"obs","application":"app","prayer":"pry","translation":"NLT"}`
		req, _ := http.NewRequestWithContext(ctx, "POST", "/soap", strings.NewReader(body))
		rr := httptest.NewRecorder()
		app.postSoap(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if ms.savedSOAPData == nil {
			t.Fatal("expected savedSOAPData not to be nil")
		}
		if ms.savedSOAPData.Translation != "NLT" {
			t.Errorf("expected saved translation NLT, got %s", ms.savedSOAPData.Translation)
		}
	})

	t.Run("fallback to user translation if empty", func(t *testing.T) {
		body := `{"date":"2026-05-07","observation":"obs","application":"app","prayer":"pry","translation":""}`
		req, _ := http.NewRequestWithContext(ctx, "POST", "/soap", strings.NewReader(body))
		rr := httptest.NewRecorder()
		app.postSoap(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}
		if ms.savedSOAPData.Translation != "ESV" {
			t.Errorf("expected fallback translation ESV, got %s", ms.savedSOAPData.Translation)
		}
	})
}

func TestExport_TranslationShortHand(t *testing.T) {
	ms := &mockStore{
		soapDataToReturn: &store.SOAPData{
			Date:           "2026-05-07",
			Observation:    "my observation",
			Application:    "my application",
			Prayer:         "my prayer",
			SelectedVerses: []string{"01001001"}, // Genesis 1:1
			Translation:    "NLT",
		},
	}
	app := newTestApplication(t, ms)
	user := &store.User{ID: 1, Email: "test@example.com", Translation: "ESV"}
	ctx := context.WithValue(context.Background(), userContextKey, user)

	// Save cached scripture for NLT:Genesis 1:1
	fakeScripture := `{"query":"Genesis 1:1","passages":["<h2 class=\"extra_text\">Genesis 1:1</h2><p>In the beginning God created the heavens and the earth.</p>"]}`
	_ = app.store.SaveCachedScripture(ctx, "NLT:Genesis 1:1", fakeScripture)

	body := `{"date":"2026-05-07","format":"html","method":"download"}`
	req, _ := http.NewRequestWithContext(ctx, "POST", "/export", strings.NewReader(body))
	rr := httptest.NewRecorder()
	app.export(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK for export, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	output := rr.Body.String()
	if !strings.Contains(output, `<h2 class="extra_text">Genesis 1:1 (NLT)</h2>`) {
		t.Errorf("expected exported HTML to contain scripture reference with translation short-hand, got: %s", output)
	}
}
