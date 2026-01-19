package web_test

import (
	"bytes"
	"html/template"
	"strings"
	"testing"
)

type PassageMeta struct { // minimalistic mock
}

type EsvResponse struct {
	Passages  []string
	Copyright string
	// other fields ignored for this test
}

func TestVersesTemplate(t *testing.T) {
	// Mock data
	data := map[string]any{
		"esvData": EsvResponse{
			Passages:  []string{"<p>Verse 1</p>", "<p>Verse 2</p>"},
			Copyright: "ESV Copyright",
		},
	}

	// Parse template
	funcMap := template.FuncMap{
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	// Read the actual template file
	// Note: In a real test we'd use embed or read the file.
	// Here I will assume I can read it or I should just paste the content if I want to be self-contained?
	// But to test the *actual file*, I should parse the file.
	tmpl, err := template.New("verses.gotmpl").Funcs(funcMap).ParseFiles("verses.gotmpl")
	if err != nil {
		t.Fatalf("Failed to parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("Failed to execute template: %v", err)
	}

	output := buf.String()

	// Verify assertions
	if !strings.Contains(output, "Verse 1") {
		t.Errorf("Expected output to contain 'Verse 1'")
	}
	if !strings.Contains(output, "Verse 2") {
		t.Errorf("Expected output to contain 'Verse 2'")
	}
	if !strings.Contains(output, "ESV Copyright") {
		t.Errorf("Expected output to contain 'ESV Copyright'")
	}
	if !strings.Contains(output, "class=\"passage-content\"") {
		t.Errorf("Expected output to contain class 'passage-content'")
	}
}
