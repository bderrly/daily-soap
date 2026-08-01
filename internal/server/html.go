package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"time"
)

type htmlRenderer struct {
	templateFS      fs.FS
	sharedTemplates *template.Template
}

// newHTMLRenderer creates a new htmlRenderer containing a shared set of parsed
// templates with support for custom template functions.
func newHTMLRenderer(templateFS fs.FS, sharedTemplateFiles ...string) (*htmlRenderer, error) {
	funcs := template.FuncMap{
		"now": time.Now,
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

	sharedTemplates, err := template.New("").Funcs(funcs).ParseFS(templateFS, sharedTemplateFiles...)
	if err != nil {
		return nil, fmt.Errorf("parsing shared templates: %w", err)
	}

	r := &htmlRenderer{
		templateFS:      templateFS,
		sharedTemplates: sharedTemplates,
	}

	return r, nil
}

// render clones the shared template set, optionally parses additional templates,
// executes the named template with the supplied data, and writes the response.
//
//nolint:unparam // status is currently always http.StatusOK but will vary for error responses.
func (h *htmlRenderer) render(w http.ResponseWriter, status int, data any, templateName string, additionalTemplateFiles ...string) error {
	ts, err := h.sharedTemplates.Clone()
	if err != nil {
		return fmt.Errorf("cloning shared templates: %w", err)
	}

	if len(additionalTemplateFiles) > 0 {
		ts, err = ts.ParseFS(h.templateFS, additionalTemplateFiles...)
		if err != nil {
			return fmt.Errorf("parsing additional templates: %w", err)
		}
	}

	buf := new(bytes.Buffer)

	err = ts.ExecuteTemplate(buf, templateName, data)
	if err != nil {
		return fmt.Errorf("executing template %q: %w", templateName, err)
	}

	w.Header().Add("Vary", "HX-Request")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)

	return nil
}

// isHTMXRequest checks if the request was made using HTMX.
func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// redirect sends an appropriate redirect response. For HTMX requests, it uses
// the HX-Redirect header to trigger a client-side redirect. For regular
// requests, it uses a standard HTTP redirect.
func redirect(w http.ResponseWriter, r *http.Request, url string, code int) {
	if isHTMXRequest(r) {
		w.Header().Set("HX-Redirect", url)
		w.WriteHeader(http.StatusNoContent)
		return
	}

	http.Redirect(w, r, url, code)
}
