package apibible_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/bderrly/daily-soap/internal/apibible"
)

func TestClient_FetchPassages_Success(t *testing.T) {
	apiKey := "test-api-token"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("api-key"); got != apiKey {
			t.Errorf("expected api-key %q, got %q", apiKey, got)
		}
		if r.URL.Query().Get("fums-version") != "3" {
			t.Errorf("expected fums-version 3, got %q", r.URL.Query().Get("fums-version"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "JHN.3.16",
				"reference": "John 3:16",
				"content": "<p class=\"p\"><span class=\"v\" data-number=\"16\">16</span>For God so loved the world...</p>",
				"copyright": "ABS 2026"
			},
			"meta": {
				"fumsToken": "test-fums-token-123"
			}
		}`))
	}))
	defer server.Close()

	client := &apibible.Client{
		APIKey:     apiKey,
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	resp, err := client.FetchPassages(context.Background(), apibible.TranslationNLT, []string{"John 3:16"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Passages) != 1 {
		t.Fatalf("expected 1 passage, got %d", len(resp.Passages))
	}
	if !strings.Contains(resp.Passages[0], `<h2 class="extra_text">John 3:16 (NLT)</h2>`) {
		t.Errorf("expected passage to contain heading with short-hand (NLT), got %q", resp.Passages[0])
	}
	if resp.Copyright != "ABS 2026" {
		t.Errorf("expected copyright 'ABS 2026', got %q", resp.Copyright)
	}
	if len(resp.FUMSTokens) != 1 || resp.FUMSTokens[0] != "test-fums-token-123" {
		t.Errorf("expected fums token 'test-fums-token-123', got %v", resp.FUMSTokens)
	}
}

func TestClient_FetchPassages_EndpointRouting(t *testing.T) {
	requestedPaths := make(map[string]bool)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths[r.URL.Path] = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "test",
				"reference": "Test",
				"content": "<p class=\"p\">Scripture text</p>",
				"copyright": "ABS 2026"
			}
		}`))
	}))
	defer server.Close()

	client := &apibible.Client{
		APIKey:     "token",
		BaseURL:    server.URL,
		HTTPClient: server.Client(),
	}

	// 1 Samuel 10 (chapter), Psalm 107:33-43 (passage range), John 3:16 (verse)
	refs := []string{"1 Samuel 10", "Psalm 107:33-43", "John 3:16"}
	_, err := client.FetchPassages(context.Background(), apibible.TranslationMSG, refs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPaths := []string{
		"/bibles/" + apibible.TranslationMSG + "/chapters/1SA.10",
		"/bibles/" + apibible.TranslationMSG + "/passages/PSA.107.33-PSA.107.43",
		"/bibles/" + apibible.TranslationMSG + "/verses/JHN.3.16",
	}

	for _, ep := range expectedPaths {
		if !requestedPaths[ep] {
			t.Errorf("expected request to path %q, but it was not requested (got: %v)", ep, requestedPaths)
		}
	}
}

func TestClient_FetchPassages_Errors(t *testing.T) {
	tests := []struct {
		status   int
		expected error
	}{
		{http.StatusBadRequest, apibible.ErrBadRequest},
		{http.StatusUnauthorized, apibible.ErrUnauthorized},
		{http.StatusForbidden, apibible.ErrForbidden},
		{http.StatusNotFound, apibible.ErrNotFound},
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer server.Close()

			client := &apibible.Client{
				APIKey:     "tok",
				BaseURL:    server.URL,
				HTTPClient: server.Client(),
			}

			_, err := client.FetchPassages(context.Background(), apibible.TranslationMSG, []string{"Psalm 1"})
			if !errors.Is(err, tt.expected) {
				t.Errorf("expected error %v, got %v", tt.expected, err)
			}
		})
	}
}
