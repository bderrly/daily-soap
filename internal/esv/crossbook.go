package esv

import (
	"fmt"
	"log/slog"
	"regexp"
	"strings"
)

// SplitCrossBookPassages detects passages that span multiple books and splits
// them into separate entries with their own headings. The ESV API merges
// cross-book ranges (e.g. "Leviticus 27:16–Numbers 1:16") into a single
// passage string, omitting the second book's heading. This function restores
// the heading by detecting where the book number changes in the verse
// data-ref attributes and inserting a new <h2 class="extra_text"> element.
func SplitCrossBookPassages(resp *Response) {
	if len(resp.Passages) != len(resp.PassageMeta) {
		return
	}

	var newPassages []string
	var newMeta []PassageMeta

	for i, meta := range resp.PassageMeta {
		if i >= len(resp.Passages) {
			break
		}

		startBook := bookFromChapterRef(meta.ChapterStart)
		endBook := bookFromChapterRef(meta.ChapterEnd)

		if startBook == 0 || endBook == 0 || startBook == endBook {
			// Same book or unable to determine — keep as-is.
			newPassages = append(newPassages, resp.Passages[i])
			newMeta = append(newMeta, meta)
			continue
		}

		// Cross-book passage detected. Find the boundary in the HTML.
		first, second, secondBook, ok := splitAtBookBoundary(resp.Passages[i], startBook)
		if !ok {
			slog.Debug("cross-book passage detected but could not find boundary",
				"canonical", meta.Canonical)
			newPassages = append(newPassages, resp.Passages[i])
			newMeta = append(newMeta, meta)
			continue
		}

		// Derive heading text for the second book from the metadata.
		secondHeading := secondBookHeading(meta, secondBook)

		// Inject the h2 heading into the second fragment.
		second = fmt.Sprintf("<h2 class=\"extra_text\">%s</h2>\n", secondHeading) + second

		newPassages = append(newPassages, first, second)
		// Use the original meta for the first part, and a synthetic one for the second.
		newMeta = append(newMeta, meta, PassageMeta{Canonical: secondHeading})

		slog.Debug("split cross-book passage",
			"canonical", meta.Canonical,
			"secondHeading", secondHeading)
	}

	resp.Passages = newPassages
	resp.PassageMeta = newMeta
}

// bookFromChapterRef extracts the book number from a chapter reference array.
// The ESV API chapter_start/chapter_end arrays contain verse IDs like [3027001, 3027034]
// where the first two digits encode the book number.
func bookFromChapterRef(refs []int) int {
	if len(refs) == 0 {
		return 0
	}
	return refs[0] / 1000000
}

// dataRefRegex matches data-ref="BBCCCVVV" attribute values.
var dataRefRegex = regexp.MustCompile(`data-ref="(\d{8})"`)

// splitAtBookBoundary splits processed passage HTML at the point where the
// book number in data-ref attributes changes. It returns the two halves and
// the new book number.
func splitAtBookBoundary(html string, startBook int) (first, second string, newBook int, ok bool) {
	matches := dataRefRegex.FindAllStringSubmatchIndex(html, -1)
	if len(matches) == 0 {
		return "", "", 0, false
	}

	startBookPrefix := fmt.Sprintf("%02d", startBook)

	for _, m := range matches {
		// m[2], m[3] are the submatch (the 8-digit ref) start/end.
		ref := html[m[2]:m[3]]
		if !strings.HasPrefix(ref, startBookPrefix) {
			// Found a verse from a different book.
			book := 0
			if len(ref) >= 2 {
				if _, err := fmt.Sscanf(ref[:2], "%d", &book); err != nil {
					// book will remain 0 if parsing fails, which is handled downstream.
					slog.Debug("failed to parse book number from ref", "ref", ref, "err", err)
				}
			}

			// Find a clean split point before this verse reference.
			splitIdx := findSplitPoint(html, m[0])
			if splitIdx <= 0 {
				return "", "", 0, false
			}

			return html[:splitIdx], html[splitIdx:], book, true
		}
	}

	return "", "", 0, false
}

// findSplitPoint finds the best point to split HTML before a given position.
// It looks backward for a block-level element boundary (</p>, </h3>, etc.)
// or a copyright paragraph end.
func findSplitPoint(html string, beforePos int) int {
	// Look for the nearest block-level closing tag before the target position.
	// We want to split between passages, so we look for </p> or </section> tags.
	searchRegion := html[:beforePos]

	// Find the last </p> before the position.
	lastP := strings.LastIndex(searchRegion, "</p>")
	if lastP >= 0 {
		return lastP + len("</p>") + 1 // +1 for newline
	}

	// Fall back to the last newline.
	lastNL := strings.LastIndex(searchRegion, "\n")
	if lastNL >= 0 {
		return lastNL + 1
	}

	return -1
}

// secondBookHeading constructs the heading text for the second book portion
// of a cross-book passage. It uses the passage metadata to determine the
// chapter and verse range. For example, given canonical "Leviticus 27:16–Numbers 1:16"
// with chapter_end=[4001001, 4001054], it constructs "Numbers 1:1–16".
func secondBookHeading(meta PassageMeta, bookNum int) string {
	name := bookNames[bookNum]
	if name == "" {
		return meta.Canonical
	}

	// Extract the end verse reference from the canonical string.
	// Canonical is like "Leviticus 27:16–Numbers 1:16"
	idx := strings.Index(meta.Canonical, name)
	if idx < 0 {
		return meta.Canonical
	}
	endRef := strings.TrimSpace(meta.Canonical[idx:])

	// Parse chapter_end to get the starting verse of the second book's chapter.
	// chapter_end[0] is the first verse of the chapter, e.g. 4001001 = Numbers 1:1
	if len(meta.ChapterEnd) >= 1 {
		chapterFirstVerse := meta.ChapterEnd[0]
		startChapter := (chapterFirstVerse / 1000) % 1000
		startVerse := chapterFirstVerse % 1000

		// Extract the end chapter:verse from the canonical reference.
		// endRef is like "Numbers 1:16"
		// We need to get the chapter and verse from it.
		afterName := strings.TrimPrefix(endRef, name)
		afterName = strings.TrimSpace(afterName)

		// Parse "1:16" → chapter=1, verse=16
		parts := strings.SplitN(afterName, ":", 2)
		if len(parts) == 2 {
			// If the start verse is 1, construct "Book Chapter:StartVerse–EndVerse"
			if startVerse == 1 {
				return fmt.Sprintf("%s %d:%d–%s", name, startChapter, startVerse, parts[1])
			}
			// Otherwise use the full end reference
			return fmt.Sprintf("%s %s", name, afterName)
		}
	}

	// Fallback to extracting from canonical.
	return endRef
}
