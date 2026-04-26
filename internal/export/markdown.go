package export

import (
	"context"
	"fmt"
	"io"
	"text/template"

	"derrclan.com/moravian-soap/internal/store"
)

// MarkdownExporter implements the Exporter interface for Markdown format.
type MarkdownExporter struct {
	tmpl *template.Template
}

const markdownTemplate = `# SOAP Journal Entry - {{.Date}}

## Scripture
{{.Scripture}}

## Observation
{{.Observation}}

## Application
{{.Application}}

## Prayer
{{.Prayer}}
`

// NewMarkdownExporter creates a new MarkdownExporter instance.
func NewMarkdownExporter() (*MarkdownExporter, error) {
	tmpl, err := template.New("soap-md").Parse(markdownTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown template: %w", err)
	}
	return &MarkdownExporter{tmpl: tmpl}, nil
}

// Export writes the SOAP entry formatted as Markdown to the provided writer.
// It assumes the scripture content is already in a format suitable for Markdown.
func (e *MarkdownExporter) Export(_ context.Context, w io.Writer, entry *store.SOAPData, scripture string) error {
	data := struct {
		Date        string
		Scripture   string
		Observation string
		Application string
		Prayer      string
	}{
		Date:        entry.Date,
		Scripture:   scripture,
		Observation: entry.Observation,
		Application: entry.Application,
		Prayer:      entry.Prayer,
	}
	if err := e.tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute markdown template: %w", err)
	}
	return nil
}

// ContentType returns "text/markdown".
func (e *MarkdownExporter) ContentType() string {
	return "text/markdown"
}
