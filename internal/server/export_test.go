package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/bderrly/daily-soap/assets"
)

// RenderTemplateForTest renders a template using the server's HTML renderer for testing.
// This function is in an _test.go file, so it is only compiled during tests and is never
// included in the production build.
func RenderTemplateForTest(templateName string, data any, additionalTemplateFiles ...string) (string, error) {
	hr, err := newHTMLRenderer(assets.HTMLFiles, "base.tmpl", "partials/*.tmpl")
	if err != nil {
		return "", fmt.Errorf("creating HTML renderer: %w", err)
	}

	rec := httptest.NewRecorder()
	if err := hr.render(rec, http.StatusOK, data, templateName, additionalTemplateFiles...); err != nil {
		return "", fmt.Errorf("rendering template %s: %w", templateName, err)
	}

	return rec.Body.String(), nil
}
