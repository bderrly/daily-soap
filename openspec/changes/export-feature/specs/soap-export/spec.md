## ADDED Requirements

### Requirement: Export to HTML
The system SHALL generate a well-formatted HTML document of a user's SOAP journal entry for a specified date, including associated Bible verse content.

#### Scenario: Successful HTML Generation
- **WHEN** a user requests an HTML export for a date where they have a journal entry
- **THEN** the system generates and returns an HTML document containing the Scripture, Observation, Application, and Prayer sections, along with the full text of the referenced Bible verses.

### Requirement: Export to Markdown
The system SHALL generate a structured Markdown document of a user's SOAP journal entry for a specified date.

#### Scenario: Successful Markdown Generation
- **WHEN** a user requests a Markdown export for a date where they have a journal entry
- **THEN** the system generates and returns a Markdown document with clear headers for Scripture, Observation, Application, and Prayer.

### Requirement: Direct Download
The system SHALL allow users to download the exported file directly to their device.

#### Scenario: Download Action
- **WHEN** a user selects the "Download" method for an export request
- **THEN** the system serves the file with the `Content-Disposition: attachment` header and the appropriate MIME type for the selected format.

### Requirement: Email Export
The system SHALL allow users to send the exported journal entry to a specified email address.

#### Scenario: Email Action
- **WHEN** a user selects the "Email" method, provides a valid email address, and submits the request
- **THEN** the system sends an email to the provided address containing the journal entry content formatted for the selected export type.

### Requirement: User Data Isolation
The system SHALL ensure that users can only export journal entries that belong to their own account.

#### Scenario: Unauthorized Export Attempt
- **WHEN** an authenticated user attempts to export a journal entry for a date where they do not have an entry, or attempts to access another user's data
- **THEN** the system returns an error (401 Unauthorized or 404 Not Found) and does not provide the exported data.
