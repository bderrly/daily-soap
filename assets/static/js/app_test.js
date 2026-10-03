import { parseHTML } from "npm:linkedom";
import { assertEquals, assertExists } from "https://deno.land/std@0.224.0/assert/mod.ts";

import * as logic from "./logic.js";

// Helper to load app.js in a specific window context
async function loadApp(window) {
  let code = await Deno.readTextFile(new URL("./app.js", import.meta.url));
  // Strip import statements which new Function() doesn't support
  code = code.replace(/import\s+{[^}]+}\s+from\s+['"]\.\/logic\.js['"];?/g, "");

  // Wrap in a function to pass window as global
  const fn = new Function(
    "window", "document", "Intl", "fetch", "Node", "setTimeout", "clearTimeout",
    "formatVerseReference", "parseVerseId",
    code
  );
  fn(
    window,
    window.document,
    window.Intl,
    window.fetch,
    window.Node,
    window.setTimeout,
    window.clearTimeout,
    logic.formatVerseReference,
    logic.parseVerseId
  );
}

// Helper to create document from generated template fixture
async function loadFixtureDocument() {
  const fixtureUrl = new URL("./fixtures/home_content.html", import.meta.url);
  const fixtureHtml = await Deno.readTextFile(fixtureUrl);
  const html = `<!DOCTYPE html><html><body>${fixtureHtml}</body></html>`;
  return parseHTML(html);
}

Deno.test("verse highlighting - valid highlighting", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="verses-section">
          <div class="daily-reading">
            <div class="passages">
              <div class="verse-content">
                <p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
              </div>
            </div>
          </div>
        </div>
        <div id="selectedVersesReference"></div>
        <textarea id="observation"></textarea>
        <textarea id="application"></textarea>
        <textarea id="prayer"></textarea>
        <div id="saveStatus"></div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  // Mock globals
  window.Node = Node;
  window.SOAP_DATA = {
    date: "2026-03-07",
    selectedVerses: [],
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ json: () => Promise.resolve({}) });

  // Load app.js
  await loadApp(window);

  // Find the verse and click it
  const verseSpan = document.querySelector('[data-ref="01002017"]');
  assertExists(verseSpan, "Verse span should exist");

  const event = new window.Event("click", {
    bubbles: true,
    cancelable: true
  });

  verseSpan.dispatchEvent(event);

  // Check if it's highlighted
  const isHighlighted = verseSpan.classList.contains("verse-selected");
  assertEquals(isHighlighted, true, "Verse should be highlighted");
});

Deno.test("verse highlighting - clicking verse number highlights verse", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="verses-section">
          <div class="daily-reading">
            <div class="passages">
              <div class="verse-content">
                <p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
              </div>
            </div>
          </div>
        </div>
        <div id="selectedVersesReference"></div>
        <textarea id="observation"></textarea>
        <textarea id="application"></textarea>
        <textarea id="prayer"></textarea>
        <div id="saveStatus"></div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  window.Node = Node;
  window.SOAP_DATA = {
    date: "2026-03-07",
    selectedVerses: [],
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ json: () => Promise.resolve({}) });

  await loadApp(window);

  const verseNum = document.querySelector('.verse-num');
  assertExists(verseNum, "Verse number should exist");

  verseNum.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));

  const verseSpan = document.querySelector('[data-ref="01002017"]');
  assertEquals(verseSpan.classList.contains("verse-selected"), true, "Verse should be highlighted when clicking verse number");
});

Deno.test("verse highlighting - clicking non-verse areas does not highlight", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="verses-section">
          <div class="daily-reading">
            <h2>Genesis 2:17-18</h2>
            <div class="passages">
              <div class="verse-content">
                <p id="para"><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
              </div>
              <div class="copyright" id="copyright">ESV</div>
            </div>
          </div>
        </div>
        <div id="selectedVersesReference"></div>
        <textarea id="observation"></textarea>
        <textarea id="application"></textarea>
        <textarea id="prayer"></textarea>
        <div id="saveStatus"></div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  window.Node = Node;
  window.SOAP_DATA = {
    date: "2026-03-07",
    selectedVerses: [],
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ json: () => Promise.resolve({}) });

  await loadApp(window);

  const verseSpan = document.querySelector('[data-ref="01002017"]');

  // Click on paragraph (empty space outside the verse span)
  const para = document.getElementById('para');
  para.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));
  assertEquals(verseSpan.classList.contains("verse-selected"), false, "Verse should NOT be highlighted when clicking paragraph space");

  // Click on header
  const h2 = document.querySelector('h2');
  h2.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));
  assertEquals(verseSpan.classList.contains("verse-selected"), false, "Verse should NOT be highlighted when clicking header");

  // Click on copyright
  const copyright = document.getElementById('copyright');
  copyright.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));
  assertEquals(verseSpan.classList.contains("verse-selected"), false, "Verse should NOT be highlighted when clicking copyright");
});

Deno.test("verse highlighting - text selection does not trigger verse toggle", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="verses-section">
          <div class="daily-reading">
            <div class="passages">
              <div class="verse-content">
                <p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
              </div>
            </div>
          </div>
        </div>
        <div id="selectedVersesReference"></div>
        <textarea id="observation"></textarea>
        <textarea id="application"></textarea>
        <textarea id="prayer"></textarea>
        <div id="saveStatus"></div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  window.Node = Node;
  window.SOAP_DATA = {
    date: "2026-03-07",
    selectedVerses: [],
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ json: () => Promise.resolve({}) });

  try {
    // Mock active selection
    window.getSelection = () => ({
      toString: () => "but of the tree"
    });

    await loadApp(window);

    const verseSpan = document.querySelector('[data-ref="01002017"]');
    verseSpan.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));

    assertEquals(verseSpan.classList.contains("verse-selected"), false, "Verse should NOT be highlighted when text was selected");
  } finally {
    delete window.getSelection;
  }
});

Deno.test("export modal - method change logic", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div id="share-btn"></div>
        <dialog id="export-modal">
          <form id="export-form">
            <input type="hidden" id="export-method" value="download">
            <input type="hidden" id="export-format" value="html">

            <div class="option-grid">
              <div class="option-card selected" data-value="download" data-target="export-method">Download</div>
              <div class="option-card" id="email-card" data-value="email" data-target="export-method">Email</div>
            </div>

            <div class="option-grid">
              <div class="option-card selected" data-value="html" data-target="export-format">HTML</div>
              <div id="format-markdown" class="option-card" data-value="markdown" data-target="export-format">Markdown</div>
            </div>

            <div id="recipients-group" style="display: none;">
              <input id="export-recipients">
            </div>
            <button type="submit">Export</button>
          </form>
        </dialog>
        <textarea id="observation"></textarea>
        <textarea id="application"></textarea>
        <textarea id="prayer"></textarea>
        <div id="saveStatus"></div>
        <div id="selectedVersesReference"></div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  // Mock globals
  window.Node = Node;
  window.SOAP_DATA = {
    date: "2026-03-07",
    selectedVerses: [],
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ json: () => Promise.resolve({}) });

  // Load app.js
  await loadApp(window);

  const emailCard = document.getElementById('email-card');
  const methodInput = document.getElementById('export-method');
  const recipientsGroup = document.getElementById('recipients-group');
  const markdownCard = document.getElementById('format-markdown');

  // Initial state
  assertEquals(methodInput.value, 'download');
  assertEquals(recipientsGroup.style.display, 'none');
  // Linkedom might return undefined or empty string for unassigned style property
  const initialDisplay = markdownCard.style.display;
  if (initialDisplay !== undefined) {
    assertEquals(initialDisplay, '');
  }

  // Click Email card
  emailCard.dispatchEvent(new window.Event("click", { bubbles: true }));

  assertEquals(methodInput.value, 'email');
  assertEquals(recipientsGroup.style.display, 'block');
  assertEquals(markdownCard.style.display, 'none');

  // Click Download card
  const downloadCard = document.querySelector('.option-card[data-value="download"]');
  downloadCard.dispatchEvent(new window.Event("click", { bubbles: true }));

  assertEquals(methodInput.value, 'download');
  assertEquals(recipientsGroup.style.display, 'none');
  assertEquals(markdownCard.style.display, 'flex');
});

Deno.test("HTMX swap updates currentDate and selectedVerseIds correctly", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="content-wrapper" id="content-container" data-date="2026-07-01" data-selected-verses="[]">
          <div class="verses-section">
            <div class="daily-reading">
              <div class="passages">
                <div class="verse-content">
                  <p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
                </div>
              </div>
            </div>
          </div>
          <div id="selectedVersesReference"></div>
          <textarea id="observation"></textarea>
          <textarea id="application"></textarea>
          <textarea id="prayer"></textarea>
          <div id="saveStatus"></div>
          <input type="date" id="date-picker" value="2026-07-01">
        </div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  // Mock globals
  window.Node = Node;
  window.SOAP_DATA = {
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };

  let lastPayload = null;
  window.fetch = (url, options) => {
    if (url === '/soap' && options.method === 'POST') {
      lastPayload = JSON.parse(options.body);
    }
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ status: "success" })
    });
  };

  // Load app.js
  await loadApp(window);

  // Simulate HTMX swap: update attributes on #content-container and dispatch htmx:afterSwap
  const container = document.getElementById('content-container');
  assertExists(container, "Container should exist");

  container.setAttribute('data-date', '2026-06-26');
  container.setAttribute('data-selected-verses', '[]');

  const afterSwapEvent = new window.Event("htmx:afterSwap", {
    bubbles: true,
    cancelable: true
  });
  container.dispatchEvent(afterSwapEvent);

  // Click the verse to trigger a save
  const verseSpan = document.querySelector('[data-ref="01002017"]');
  assertExists(verseSpan, "Verse span should exist");

  const clickEvent = new window.Event("click", {
    bubbles: true,
    cancelable: true
  });
  verseSpan.dispatchEvent(clickEvent);

  // Wait for the autosave timeout (1000ms delay + buffer)
  await new Promise(resolve => setTimeout(resolve, 1100));

  // Verify that the save payload was sent with the swapped date 2026-06-26
  assertExists(lastPayload, "A save request should have been sent");
  assertEquals(lastPayload.date, '2026-06-26', "The date in the payload should be 2026-06-26");
  assertEquals(lastPayload.selectedVerses, ['01002017'], "The selected verses should include the clicked verse");
});

Deno.test("initial load - date parameter in URL", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="content-wrapper" id="content-container" data-date="2026-07-01" data-selected-verses="[]">
          <div id="selectedVersesReference"></div>
          <textarea id="observation"></textarea>
          <textarea id="application"></textarea>
          <textarea id="prayer"></textarea>
          <div id="saveStatus"></div>
          <input type="date" id="date-picker" value="2026-07-01">
        </div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  // Mock globals
  window.Node = Node;
  window.SOAP_DATA = {
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ ok: true, json: () => Promise.resolve({ status: "success" }) });

  // Mock location to simulate ?date=2026-06-20
  window.location = {
    search: "?date=2026-06-20"
  };

  // Load app.js
  await loadApp(window);

  // Verify that date picker value is updated to match URL parameter in init()
  const datePicker = document.getElementById('date-picker');
  assertExists(datePicker, "Date picker should exist");
  assertEquals(datePicker.value, "2026-06-20", "Date picker value should be updated to match URL query parameter");
});

Deno.test("flatpickr initialization and entry encircling", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="content-wrapper" id="content-container" data-date="2026-06-20" data-selected-verses="[]">
          <div id="selectedVersesReference"></div>
          <textarea id="observation"></textarea>
          <textarea id="application"></textarea>
          <textarea id="prayer"></textarea>
          <div id="saveStatus"></div>
          <input type="text" id="date-picker" value="2026-06-20">
        </div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  window.Node = Node;
  window.SOAP_DATA = {
    csrfToken: "test-token"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };

  // Mock fetch to return dates with entries
  window.fetch = (url) => {
    if (typeof url === 'string' && url.includes('/soap/dates')) {
      return Promise.resolve({
        ok: true,
        json: () => Promise.resolve({ dates: ["2026-06-15", "2026-06-20"] })
      });
    }
    return Promise.resolve({ ok: true, json: () => Promise.resolve({ status: "success" }) });
  };

  let capturedConfig = null;
  window.flatpickr = (element, config) => {
    capturedConfig = config;
    return {
      redraw: () => {},
      destroy: () => {}
    };
  };

  await loadApp(window);

  assertExists(capturedConfig, "Flatpickr should have been initialized with config");
  assertEquals(capturedConfig.dateFormat, "Y-m-d");
  assertEquals(capturedConfig.minDate, "2026-01-01");

  // Wait a tick for loadEntryDatesForMonth fetch to resolve
  await new Promise((resolve) => setTimeout(resolve, 50));

  // Test onDayCreate with a date that has an entry
  const dayElemWithEntry = document.createElement("span");
  dayElemWithEntry.dateObj = new Date(2026, 5, 15); // June 15, 2026
  capturedConfig.onDayCreate(null, null, null, dayElemWithEntry);
  assertEquals(dayElemWithEntry.classList.contains("has-journal-entry"), true, "Should have has-journal-entry class");

  // Test onDayCreate with a date without an entry
  const dayElemNoEntry = document.createElement("span");
  dayElemNoEntry.dateObj = new Date(2026, 5, 16); // June 16, 2026
  capturedConfig.onDayCreate(null, null, null, dayElemNoEntry);
  assertEquals(dayElemNoEntry.classList.contains("has-journal-entry"), false, "Should not have has-journal-entry class");
});

Deno.test("verse reference and save payload include translation shorthand", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const html = `
    <!DOCTYPE html>
    <html>
      <body>
        <div class="content-wrapper" id="content-container" data-date="2026-07-01" data-selected-verses="[]" data-translation="NLT">
          <select id="translation-select">
            <option value="ESV">ESV</option>
            <option value="NLT" selected>NLT</option>
            <option value="MSG">MSG</option>
          </select>
          <div class="verses-section">
            <div class="daily-reading">
              <div class="passages">
                <div class="verse-content">
                  <p><span class="verse" data-ref="01002017"><b class="verse-num">17</b>but of the tree...</span></p>
                </div>
              </div>
            </div>
          </div>
          <div id="selectedVersesReference"></div>
          <textarea id="observation"></textarea>
          <textarea id="application"></textarea>
          <textarea id="prayer"></textarea>
          <div id="saveStatus"></div>
        </div>
      </body>
    </html>
  `;

  const { window, document, Node } = parseHTML(html);

  window.Node = Node;
  window.SOAP_DATA = {
    csrfToken: "test-token",
    translation: "NLT"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };

  let lastPayload = null;
  window.fetch = (url, options) => {
    if (url === '/soap' && options.method === 'POST') {
      lastPayload = JSON.parse(options.body);
    }
    return Promise.resolve({
      ok: true,
      json: () => Promise.resolve({ status: "success" })
    });
  };

  await loadApp(window);

  const verseSpan = document.querySelector('[data-ref="01002017"]');
  assertExists(verseSpan, "Verse span should exist");

  // Click verse to select it
  verseSpan.dispatchEvent(new window.Event("click", { bubbles: true, cancelable: true }));

  const referenceDiv = document.getElementById("selectedVersesReference");
  assertEquals(referenceDiv.textContent, "Genesis 2:17 (NLT)", "Reference should include (NLT) translation");

  // Wait for autosave
  await new Promise(resolve => setTimeout(resolve, 1100));

  assertExists(lastPayload, "Payload should be sent");
  assertEquals(lastPayload.translation, "NLT", "Payload should contain translation NLT");

  // Change translation select to MSG
  const select = document.getElementById("translation-select");
  const nltOpt = select.querySelector('option[value="NLT"]');
  const msgOpt = select.querySelector('option[value="MSG"]');
  if (nltOpt) nltOpt.removeAttribute("selected");
  if (msgOpt) msgOpt.setAttribute("selected", "selected");
  select.dispatchEvent(new window.Event("change", { bubbles: true, cancelable: true }));

  assertEquals(referenceDiv.textContent, "Genesis 2:17 (MSG)", "Reference should update to (MSG) translation");
});

Deno.test("HTMX afterSettle preserves selectedVersesReference display", { sanitizeOps: false, sanitizeResources: false }, async () => {
  const { window, document, Node } = await loadFixtureDocument();

  window.Node = Node;
  window.SOAP_DATA = {
    csrfToken: "test-token",
    translation: "NLT"
  };
  window.Intl = {
    DateTimeFormat: () => ({
      resolvedOptions: () => ({ timeZone: "UTC" })
    })
  };
  window.fetch = () => Promise.resolve({ ok: true, json: () => Promise.resolve({}) });

  await loadApp(window);

  const referenceDiv = document.getElementById("selectedVersesReference");
  assertExists(referenceDiv, "Reference div should exist");

  // Verify that the template rendered style="display: block;" because selectedVerses has items
  assertEquals(referenceDiv.style.display, "block", "Template should render display: block when selectedVerses is populated");

  // Simulate HTMX settle wiping the inline style (as happened during attribute settling)
  referenceDiv.style.display = "";

  // Dispatch htmx:afterSettle event
  const container = document.getElementById("content-container");
  const settleEvent = new window.Event("htmx:afterSettle", { bubbles: true, cancelable: true });
  container.dispatchEvent(settleEvent);

  assertEquals(referenceDiv.textContent, "Genesis 2:17 (NLT)", "Reference text should be updated");
  assertEquals(referenceDiv.style.display, "block", "Reference display should be block after settle");
});


