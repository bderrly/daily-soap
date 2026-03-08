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
