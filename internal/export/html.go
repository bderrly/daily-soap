// Package export provides tools for exporting SOAP journal entries in various formats.
package export

import (
	"context"
	"fmt"
	"html/template"
	"io"

	"derrclan.com/moravian-soap/internal/store"
)

// HTMLExporter implements the Exporter interface for HTML format.
type HTMLExporter struct {
	tmpl *template.Template
}

const htmlTemplate = `
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>SOAP Entry - {{.Date}}</title>
    <style>
        body { font-family: sans-serif; line-height: 1.6; max-width: 800px; margin: 40px auto; padding: 20px; }
        h1 { border-bottom: 2px solid #333; padding-bottom: 10px; }
        h2 { color: #555; margin-top: 30px; }
        .section { margin-bottom: 20px; }
        .scripture { font-style: italic; background: #f9f9f9; padding: 15px; border-left: 5px solid #ccc; }
        @media print {
            body { margin: 0; padding: 0; }
            .no-print { display: none; }
        }
    </style>
</head>
<body>
    <h1>SOAP Journal Entry</h1>
    <p><strong>Date:</strong> {{.Date}}</p>

    <div class="section">
        <h2>Scripture</h2>
        <div class="scripture">
            {{.Scripture}}
        </div>
    </div>

    <div class="section">
        <h2>Observation</h2>
        <p>{{.Observation}}</p>
    </div>

    <div class="section">
        <h2>Application</h2>
        <p>{{.Application}}</p>
    </div>

    <div class="section">
        <h2>Prayer</h2>
        <p>{{.Prayer}}</p>
    </div>
</body>
</html>
`

// NewHTMLExporter creates a new HTMLExporter.
func NewHTMLExporter() (*HTMLExporter, error) {
	tmpl, err := template.New("soap-html").Parse(htmlTemplate)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML template: %w", err)
	}
	return &HTMLExporter{tmpl: tmpl}, nil
}

// Export writes the SOAP entry as HTML to the writer.
func (e *HTMLExporter) Export(_ context.Context, w io.Writer, entry *store.SOAPData, scripture string) error {
	data := struct {
		Date        string
		Scripture   template.HTML
		Observation string
		Application string
		Prayer      string
	}{
		Date:        entry.Date,
		Scripture:   template.HTML(scripture),
		Observation: entry.Observation,
		Application: entry.Application,
		Prayer:      entry.Prayer,
	}
	if err := e.tmpl.Execute(w, data); err != nil {
		return fmt.Errorf("failed to execute HTML template: %w", err)
	}
	return nil
}

// ContentType returns "text/html".
func (e *HTMLExporter) ContentType() string {
	return "text/html"
}
