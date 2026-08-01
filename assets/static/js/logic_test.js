import { assertEquals } from "https://deno.land/std@0.224.0/assert/mod.ts";
import { formatVerseReference, parseVerseId } from "./logic.js";

Deno.test("parseVerseId - correctly parses a valid ID", () => {
    const result = parseVerseId("23063008");
    assertEquals(result.book, 23);
    assertEquals(result.chapter, 63);
    assertEquals(result.verse, 8);
});

Deno.test("formatVerseReference - handles contiguous ranges", () => {
    const input = ["23063008", "23063009", "23063010"];
    const result = formatVerseReference(input);
    assertEquals(result, "Isaiah 63:8-10");
});

Deno.test("formatVerseReference - handles non-contiguous verses", () => {
    const input = ["23063008", "23063010"];
    const result = formatVerseReference(input);
    assertEquals(result, "Isaiah 63:8,10");
});

Deno.test("formatVerseReference - handles multiple chapters", () => {
    const input = ["23063008", "23064001"];
    const result = formatVerseReference(input);
    assertEquals(result, "Isaiah 63:8; Isaiah 64:1");
});
