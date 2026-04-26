## Why

Users currently cannot easily share or download their SOAP journal entries. Adding an export feature allows users to archive their reflections or share them with others (e.g., via email), increasing the utility of the journal for personal record-keeping and community sharing.

## What Changes

- **New Export Backend**: A new `internal/export` package to handle the transformation of SOAP data into HTML and Markdown formats.
- **New API Endpoint**: A `/export` endpoint in the server to process export requests, supporting format selection (HTML, Markdown) and delivery method (Download, Email).
- **Email Integration**: Enhanced `internal/email` client to support sending exported journal entries as formatted emails.
- **Frontend UI Enhancements**:
    - A "Share" button on the main journal page.
    - An interactive "Export Modal" allowing users to choose their preferred format and action.
    - Client-side logic to handle downloads and email submission via the new API.

## Capabilities

### New Capabilities
- `soap-export`: Ability to export SOAP journal entries for a specific date in HTML or Markdown format, with options to download the file directly or send it via email.

### Modified Capabilities
<!-- Existing capabilities whose REQUIREMENTS are changing (not just implementation).
     Only list here if spec-level behavior changes. Each needs a delta spec file.
     Use existing spec names from openspec/specs/. Leave empty if no requirement changes. -->

## Impact

- **Backend**: New `internal/export` package; updates to `internal/server` for the new endpoint; updates to `internal/email` for new email types.
- **Frontend**: Modifications to `index.html`, `style.css`, and `app.js` to include the export UI and logic.
- **Dependencies**: Uses existing `HTMX` for some interactions but may require standard `fetch` for email submissions and direct links for downloads.
