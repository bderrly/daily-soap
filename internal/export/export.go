// Package export provides tools for exporting SOAP journal entries in various formats.
package export

import (
	"context"
	"io"

	"derrclan.com/moravian-soap/internal/store"
)

// Exporter defines the interface for exporting SOAP entries in different formats
// (e.g., HTML, Markdown).
type Exporter interface {
	// Export writes the formatted SOAP entry to the provided writer.
	// The scripture parameter should contain the pre-fetched scripture content.
	Export(ctx context.Context, w io.Writer, entry *store.SOAPData, scripture string) error
	// ContentType returns the MIME type of the exported content (e.g., "text/html").
	ContentType() string
}
