package server_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/bderrly/daily-soap/internal/esv"
	"github.com/bderrly/daily-soap/internal/server"
	"github.com/bderrly/daily-soap/internal/store"
)

func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

func renderHomeContentFixture() (string, error) {
	data := map[string]any{
		"esvData": esv.Response{
			Passages: []string{
				`<p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree of the knowledge of good and evil you shall not eat, for in the day that you eat of it you shall surely die.</span></p>`,
			},
			Copyright: "Scripture quotations are from the ESV® Bible (The Holy Bible, English Standard Version®), copyright © 2001 by Crossway, a publishing ministry of Good News Publishers. Used by permission. All rights reserved.",
		},
		"fumsTokens":     []string{},
		"date":           "2026-07-01",
		"observation":    "",
		"application":    "",
		"prayer":         "",
		"selectedVerses": []string{"01002017"},
		"translation":    "NLT",
		"user": &store.User{
			ID:          1,
			Email:       "test@example.com",
			Translation: "NLT",
		},
		"CSRFToken": "test-token",
		"Nonce":     "test-nonce",
	}

	res, err := server.RenderTemplateForTest("home:content", data, "pages/home.tmpl")
	if err != nil {
		return "", fmt.Errorf("rendering home:content template: %w", err)
	}
	return res, nil
}

func TestFixturesUpToDate(t *testing.T) {
	rendered, err := renderHomeContentFixture()
	if err != nil {
		t.Fatalf("failed to render home content fixture: %v", err)
	}

	root, err := findRepoRoot()
	if err != nil {
		t.Fatalf("failed to determine repository root: %v", err)
	}

	fixturePath := filepath.Clean(filepath.Join(root, "assets", "static", "js", "fixtures", "home_content.html"))

	if os.Getenv("UPDATE_FIXTURES") == "1" {
		if err := os.MkdirAll(filepath.Dir(fixturePath), 0o750); err != nil {
			t.Fatalf("failed to create fixture directory: %v", err)
		}
		if err := os.WriteFile(fixturePath, []byte(rendered), 0o600); err != nil {
			t.Fatalf("failed to write fixture file: %v", err)
		}
		t.Log("Wrote fixture file because UPDATE_FIXTURES=1 was set")
		return
	}

	existing, err := os.ReadFile(fixturePath) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			t.Fatalf("fixture file %s does not exist; run 'mise run fixtures' to generate it", fixturePath)
		}
		t.Fatalf("failed to read fixture file: %v", err)
	}

	if string(existing) != rendered {
		t.Fatalf("fixture file %s is out of date with current templates; run 'mise run fixtures' to regenerate it", fixturePath)
	}
}
