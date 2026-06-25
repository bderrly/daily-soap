# Resizable Input Sections Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Resize input sections proportionally to the height of the desktop display to eliminate double scrollbars on default page load, while maintaining min-height safety and ignoring mobile views.

**Architecture:** Flexbox column scaling within `@media (min-width: 901px)` to constrain layout height to `100vh`, while letting Observation, Application, and Prayer fields grow/shrink proportionally and scroll natively when min-height limits are hit.

**Tech Stack:** Go Templates, CSS, JS

---

### Task 1: Update HTML Template to Add Helper Class

**Files:**
- Modify: `internal/server/web/content.gotmpl`

- [ ] **Step 1: Add soap-field-expandable class to input field wrappers**
Modify `internal/server/web/content.gotmpl` to add the `soap-field-expandable` class to the Observation, Application, and Prayer `.soap-field` divs.
```html
        <div class="soap-field soap-field-expandable">
            <label for="observation">Observation</label>
            <textarea id="observation" name="observation" rows="6"
                placeholder="What do you observe in these verses?">{{.observation}}</textarea>
        </div>
        <div class="soap-field soap-field-expandable">
            <label for="application">Application</label>
            <textarea id="application" name="application" rows="6"
                placeholder="How can you apply this to your life?">{{.application}}</textarea>
        </div>
        <div class="soap-field soap-field-expandable">
            <label for="prayer">Prayer</label>
            <textarea id="prayer" name="prayer" rows="6"
                placeholder="What is your prayer?">{{.prayer}}</textarea>
        </div>
```

- [ ] **Step 2: Commit template changes**
Run:
```bash
git add internal/server/web/content.gotmpl
git commit -m "feat: add soap-field-expandable class to content template"
```


### Task 2: Implement Responsive Styling in CSS

**Files:**
- Modify: `internal/server/web/style.css`

- [ ] **Step 1: Add CSS rules for desktop layout**
Add the layout rules targeting `@media (min-width: 901px)` to the end of `internal/server/web/style.css`.
```css
@media (min-width: 901px) {
    body {
        height: 100vh;
        overflow: hidden;
        display: flex;
        flex-direction: column;
    }

    .container {
        height: calc(100vh - 4rem);
        display: flex;
        flex-direction: column;
        overflow: hidden;
    }

    .content-wrapper {
        flex: 1;
        min-height: 0;
        overflow: hidden;
    }

    .verses-section {
        height: 100%;
        overflow-y: auto;
    }

    .soap-section {
        height: 100%;
        max-height: 100%;
        position: static;
        overflow-y: auto;
        display: flex;
        flex-direction: column;
        gap: 1rem;
    }

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

    .site-footer {
        flex-shrink: 0;
    }
}
```

- [ ] **Step 2: Commit CSS changes**
Run:
```bash
git add internal/server/web/style.css
git commit -m "style: implement responsive fit-viewport style for desktop"
```


### Task 3: Verify the Changes

- [ ] **Step 1: Run linter and formatter**
Run linter and formatter checks using mise:
```bash
mise exec -- check
```

- [ ] **Step 2: Start the application server**
Run:
```bash
go run cmd/server/main.go
```

- [ ] **Step 3: Manually test UI behaviors**
1. Open page in browser on desktop screen size (> 900px wide). Verify that the page fits exactly within the viewport without body scrollbars.
2. Select verses and check that the scripture selection box is displayed and the textareas shrink proportionally to accommodate it.
3. Shrink the browser height very small and check that textareas stop shrinking at 100px and a scrollbar appears on the `.soap-section` container.
4. Resize browser to mobile size (<= 900px wide) and check that layout stacks vertically and scrolls normally.
