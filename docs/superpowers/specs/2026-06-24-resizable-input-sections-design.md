# Design Spec: Resizable Input Sections for SOAP Page

## Background & Goal
Currently, on desktop viewports, the SOAP entry page displays two vertical scrollbars (one on the page body and another on the `.soap-section` panel). The input sections (Observation, Application, Prayer) have fixed heights, which causes the layout to exceed standard laptop viewport heights.

The goal is to fit the page structure perfectly within the browser viewport on desktop screens (no page-level scrollbars) while making the input textareas dynamically and proportionally scale to the height of the display. The textareas should shrink to accommodate the scripture reference box when it is displayed, and hit a minimum height safeguard of `100px` for readability on very short viewports. Mobile layouts should remain unaffected.

## Design Details

### 1. HTML Markup Updates
In `internal/server/web/content.gotmpl`:
- Add a new helper class `soap-field-expandable` to the Observation, Application, and Prayer `.soap-field` containers:
  ```html
  <div class="soap-field soap-field-expandable">
      <label for="observation">Observation</label>
      <textarea id="observation" ...></textarea>
  </div>
  ```

### 2. CSS Stylesheets Updates
In `internal/server/web/style.css`, target desktop-only screens (`@media (min-width: 901px)`):
- **Body & Container**:
  - Restrict `body` to `height: 100vh; overflow: hidden;` to eliminate window scrollbars.
  - Constrain `.container` to `height: calc(100vh - 4rem);` (accounting for body padding) and use `display: flex; flex-direction: column; overflow: hidden;`.
- **Content Layout**:
  - Set `.content-wrapper` to `flex: 1; min-height: 0; overflow: hidden;` to allow it to fill the remaining container space.
  - Settle `.verses-section` to `height: 100%; overflow-y: auto;` to allow independent scripture reading.
  - Convert `.soap-section` to a flex column layout:
    ```css
    .soap-section {
        height: 100%;
        max-height: 100%;
        position: static;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }
    ```
- **Proportional Textareas**:
  - Remove fixed margins and define:
    ```css
    .soap-section .soap-field {
        margin-bottom: 0;
    }
    .soap-section .soap-field.soap-field-expandable {
        flex: 1;
        display: flex;
        flex-direction: column;
        min-height: 100px;
    }
    .soap-section .soap-field.soap-field-expandable textarea {
        flex: 1;
        min-height: 0;
        resize: none;
    }
    ```
- **Footer**:
  - Ensure the footer is kept at the bottom of the container without shrinking: `.site-footer { flex-shrink: 0; }`.

## Verification Plan

### Manual Verification
1. **Desktop Chrome/Firefox (Viewport > 900px)**:
   - Check that the page loads with zero window scrollbars.
   - Resize window height and verify that textareas resize proportionally.
   - Click a verse to create/display the scripture reference box. Verify that the reference box is shown and the three textareas shrink proportionally to accommodate it.
   - Reduce viewport height to extremely small values (e.g., < 500px) and verify that textareas stop shrinking at `100px` height, and a scrollbar appears on `.soap-section` to maintain readability.
2. **Mobile Simulation (Viewport <= 900px)**:
   - Verify that the layout stacks vertically and scrolls normally, preserving the existing mobile experience.
