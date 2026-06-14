# Spec: Unify Date Change Action using HTMX

## 1. Background & Goals

When a user changes the date via the date picker in the Daily Soaps application, the current implementation splits responsibility between:
1. **HTMX**: Fetches and renders the new scripture verses by calling `/reading` (returning HTML).
2. **Vanilla JS (`app.js`)**: Listens to the `change` event on the datepicker, saves the current SOAP data, fetches `/soap?date=...` (returning JSON), manually updates the textareas, manages loading states, and updates selection references.

This split introduces potential race conditions between the two concurrent network requests, forces the application to maintain both HTMX-based HTML swapping and manual AJAX JSON parsing, and creates brittle DOM manipulation logic in `app.js`.

The goal of this design is to unify the date change action. Instead of split-responsibility, HTMX will request the single unified endpoint `/` (root) with a `date` query parameter. The server will return the combined content area (both the verses and the textareas pre-populated), and HTMX will swap the entire content wrapper.

---

## 2. Architecture & Data Flow

### Current Flow
```
User changes Date Picker
  ├── [HTMX GET] /reading?date=...  ──> Update .verses-section (HTML)
  └── [JS listener]
        ├── [JS POST] /soap (Save old SOAP)
        └── [JS GET]  /soap?date=... ──> Manual update of textareas & selections (JSON)
```

### Proposed Flow
```
User changes Date Picker
  └── [JS listener] intercept 'change'
        ├── [JS POST] /soap (Save old SOAP)
        └── [Awaits POST complete]
              └── Dispatch 'change-date' event
                    └── [HTMX GET] /?date=... ──> Update #content-container (HTML)
                          └── [HTMX Swap Settle]
                                └── [JS listener 'htmx:afterSwap']
                                      └── Read window.SOAP_DATA & refresh highlights
```

---

## 3. Proposed Changes

### Backend

#### 1. Define a Content Partial Template: `internal/server/web/content.gotmpl`
Create a new partial template containing the `.content-wrapper` (including both the scripture reading section and the SOAP input fields).

This partial will also contain an inline `<script>` tag to update the global `window.SOAP_DATA` with the newly loaded date and selected verses so that the frontend code can highlight selected verses.

#### 2. Modify `internal/server/web/index.html`
Replace the inline `.content-wrapper` block with a template execution of the new `content.gotmpl` partial.
Wrap the template execution in `<div id="content-container">`.

#### 3. Update `internal/server/handlers_app.go`
* Modify `handleIndex` to check for the `HX-Request` header.
* If `HX-Request` is `"true"`, execute the `content.gotmpl` partial template instead of `index.html`.
* Ensure `/` handles the `date` query parameter (this is already supported).

---

### Frontend

#### 1. Configure the Date Picker (`content.gotmpl`)
* Update the date-picker element's attributes:
  * `hx-get="/"`
  * `hx-target="#content-container"`
  * `hx-swap="outerHTML"`
  * `hx-trigger="change-date"` (a custom event triggered after saving finishes)
  * `hx-push-url="true"` (to update the address bar)

#### 2. Simplify JavaScript Logic (`internal/server/web/app.js`)
* Listen to the native `change` event on the date picker.
* When a change occurs:
  1. Immediately trigger the autosave process (`saveData(true)`).
  2. Await the response of the save operation.
  3. Update the `currentDate` locally to the new date.
  4. Dispatch the `change-date` custom event on the date picker to let HTMX fetch the new content.
* Remove the `loadDataForDate()` function entirely, eliminating manual loading indicators and textarea value manipulation.
* Listen to `htmx:afterSwap` on `#content-container` (or document body target check) to read the new date and selected verses from `window.SOAP_DATA` and refresh verse highlights.

---

## 4. Verification Plan

### Automated Tests
* Run `go test ./...` to verify all backend handlers remain correct.
* Run Deno tests `deno test --allow-read internal/server/web/` to verify frontend logic.

### Manual Verification
1. Open the application.
2. Make edits to the textareas for the current date.
3. Select a new date via the date picker.
4. Verify:
   * The edits on the original date are successfully saved.
   * The new scripture readings and SOAP fields load correctly.
   * Bookmarked URLs for specific dates load the correct content.
