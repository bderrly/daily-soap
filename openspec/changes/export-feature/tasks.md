## 1. Backend Export Logic

- [x] 1.1 Create `internal/export` package and define `Exporter` interface.
- [x] 1.2 Implement `HTMLExporter` using `html/template`.
- [x] 1.3 Implement `MarkdownExporter`.
- [x] 1.4 Add unit tests for both exporters in `internal/export/export_test.go`.

## 2. Email Integration & Reliability

- [ ] 2.1 Create a database migration for `queued_emails` table to support persistent retries.
- [ ] 2.2 Implement `internal/store` methods for queuing, retrieving, and updating email tasks.
- [ ] 2.3 Create an inline-CSS version of the SOAP export HTML template for email delivery.
- [ ] 2.4 Add `QueueExportEmail` method to `Client` in `internal/email/email.go`.
- [ ] 2.5 Implement a background worker in `internal/email` (or a new package) to process the email queue with exponential backoff.

## 3. Server Integration

- [ ] 3.1 Implement the `/export` handler in `internal/server/server.go`.
- [ ] 3.2 Add data isolation checks to ensure users only export their own data.
- [ ] 3.3 Register the `/export` route (GET/POST) in the router.
- [ ] 3.4 Add integration tests for the export endpoint.

## 4. Frontend UI

- [ ] 4.1 Add the "Share" button and the "Export Modal" to `internal/server/web/index.html`.
- [ ] 4.2 Define styles for the Share modal in `internal/server/web/style.css`.
- [ ] 4.3 Implement logic for the Share modal and export actions in `internal/server/web/app.js`.
- [ ] 4.4 Perform a full end-to-end verification of the export flow.
