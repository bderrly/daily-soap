## Context

The SOAP journal provides users with a platform for scripture study and reflection. While currently functional for entry management, the system lacks a direct way for users to archive their work outside the platform or share specific entries via standard channels like email. This design introduces a robust export system to address these needs.

## Goals / Non-Goals

**Goals:**
- Implement a flexible export engine supporting multiple formats (HTML, Markdown).
- Create a user-friendly UI for selecting export options and delivering content.
- Integrate export delivery with the existing email system.
- Ensure strict data isolation for export requests.

**Non-Goals:**
- Bulk export of entire journals (single-date export only).
- Support for complex file formats like PDF or DOCX.
- Social media "direct sharing" (users can copy/paste generated content or send via email).

## Decisions

- **Package: `internal/export`**: A new package will house the `Exporter` interface and its implementations (`HTMLExporter`, `MarkdownExporter`). This separation ensures that the export logic is decoupled from HTTP handlers, making it easier to test and maintain.
- **Go Templates for HTML**: The `HTMLExporter` will use Go's `html/template` package to generate clean, printable layouts. This approach provides built-in protection against XSS and allows for consistent styling.
- **API Endpoint `/export`**: A POST endpoint that handles export requests.
    - **Payload**: `{"date": "YYYY-MM-DD", "format": "html|markdown", "method": "email|download", "recipients": ["email1", "email2"]}`.
- **Delivery Methods**:
    - **Download**: Uses standard HTTP headers (`Content-Disposition: attachment`) to trigger a file save in the browser. Supports HTML and Markdown.
    - **Email**: Leverages the `internal/email` package.
        - **Format**: Only HTML format is supported for email sharing.
        - **Content**: The exported SOAP entry will be placed directly in the email body (not as an attachment).
        - **Styling**: To ensure compatibility across email clients, the HTML template will use **inline CSS** rather than a `<style>` block.
        - **Recipients**: Users can send exports to multiple email addresses simultaneously.
        - **Reliability**: To handle transient delivery failures, export email requests will be persisted to a `queued_emails` database table.
            - **Schema**: `id`, `user_id`, `recipient` (one row per recipient), `subject`, `body_html`, `status`, `attempts`, `last_attempt_at`, `next_attempt_at`.
            - **Worker**: A background worker will handle sending with exponential backoff and a maximum of five retries.
- **UI: Share Modal**: Instead of adding multiple buttons to the main view, a single "Share" button will open a modal. It will allow selecting the format (for download) or entering multiple recipient emails.

## Risks / Trade-offs

- **[Risk] Resource Exhaustion / Spam** → [Mitigation] Limit export requests to authenticated users and single-date entries only. While any recipient is allowed, the content is restricted to the user's own journal entries.
- **[Risk] Email Delivery Failures** → [Mitigation] Persistent storage of email tasks allows for retries even if the server restarts. The UI will indicate that the email has been "queued" for delivery.
- **[Trade-off] Client-side vs. Server-side Generation** → [Decision] Server-side generation is chosen to ensure consistency and leverage the existing server-side templates and data access logic, despite the additional server load.
