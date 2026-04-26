package export_test

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"derrclan.com/moravian-soap/internal/export"
	"derrclan.com/moravian-soap/internal/store"
)

func TestHTMLExporter(t *testing.T) {
	exporter, err := export.NewHTMLExporter()
	if err != nil {
		t.Fatalf("failed to create HTMLExporter: %v", err)
	}

	entry := &store.SOAPData{
		Date:        "2026-04-23",
		Observation: "Good observation",
		Application: "Practical application",
		Prayer:      "Sincere prayer",
	}
	scripture := "<p>John 3:16 - For God so loved the world...</p>"

	var buf bytes.Buffer
	err = exporter.Export(context.Background(), &buf, entry, scripture)
	if err != nil {
		t.Fatalf("failed to export HTML: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "2026-04-23") {
		t.Errorf("output missing date")
	}
	if !strings.Contains(output, scripture) {
		t.Errorf("output missing scripture")
	}
	if !strings.Contains(output, "Good observation") {
		t.Errorf("output missing observation")
	}
	if exporter.ContentType() != "text/html" {
		t.Errorf("incorrect content type: %s", exporter.ContentType())
	}
}

func TestHTMLExporter_Escaping(t *testing.T) {
	exporter, err := export.NewHTMLExporter()
	if err != nil {
		t.Fatalf("failed to create HTMLExporter: %v", err)
	}

	entry := &store.SOAPData{
		Observation: "Rock & Roll <script>alert(1)</script>",
	}
	scripture := "<b>John 3:16</b>"

	var buf bytes.Buffer
	err = exporter.Export(context.Background(), &buf, entry, scripture)
	if err != nil {
		t.Fatalf("failed to export HTML: %v", err)
	}

	output := buf.String()
	// Observation should be escaped
	if strings.Contains(output, "Rock & Roll") {
		t.Errorf("observation not escaped: &")
	}
	if strings.Contains(output, "<script>") {
		t.Errorf("observation not escaped: <script>")
	}
	if !strings.Contains(output, "Rock &amp; Roll") {
		t.Errorf("observation missing escaped &: %s", output)
	}
	// Scripture should NOT be escaped as it's passed as template.HTML
	if !strings.Contains(output, "<b>John 3:16</b>") {
		t.Errorf("scripture should not be escaped: %s", output)
	}
}

func TestMarkdownExporter(t *testing.T) {
	exporter, err := export.NewMarkdownExporter()
	if err != nil {
		t.Fatalf("failed to create MarkdownExporter: %v", err)
	}

	entry := &store.SOAPData{
		Date:        "2026-04-23",
		Observation: "Good observation",
		Application: "Practical application",
		Prayer:      "Sincere prayer",
	}
	scripture := "John 3:16 - For God so loved the world..."

	var buf bytes.Buffer
	err = exporter.Export(context.Background(), &buf, entry, scripture)
	if err != nil {
		t.Fatalf("failed to export Markdown: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "# SOAP Journal Entry - 2026-04-23") {
		t.Errorf("output missing header/date")
	}
	if !strings.Contains(output, scripture) {
		t.Errorf("output missing scripture")
	}
	if !strings.Contains(output, "## Observation\nGood observation") {
		t.Errorf("output missing observation")
	}
	if exporter.ContentType() != "text/markdown" {
		t.Errorf("incorrect content type: %s", exporter.ContentType())
	}
}
