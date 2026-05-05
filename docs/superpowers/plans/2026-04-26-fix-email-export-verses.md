# Fix Email Export Verses Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use beads (`bd`) for task tracking.

**Goal:** Ensure that the email export only contains the verses selected by the user, if any. If no verses are selected, fall back to the full daily scripture.

**Architecture:**
- Implement `FormatReferences(verseIDs []string) string` in `internal/esv/reference.go` to convert 8-digit verse IDs to a single reference string.
- Update `internal/server/server.go:handleExport` to use the selected verses if available.

**Tech Stack:** Go, standard library.

---

## File Structure

- Create: `internal/esv/reference.go` (and `internal/esv/reference_test.go`)
- Modify: `internal/server/server.go`

---

## Tasks

### 1. Implement Reference Formatting Logic

- [ ] **Step 1.1: Create `internal/esv/reference.go` with book name mapping.**

```go
package esv

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

var bookNames = map[int]string{
	1: "Genesis", 2: "Exodus", 3: "Leviticus", 4: "Numbers", 5: "Deuteronomy",
	6: "Joshua", 7: "Judges", 8: "Ruth", 9: "1 Samuel", 10: "2 Samuel",
	11: "1 Kings", 12: "2 Kings", 13: "1 Chronicles", 14: "2 Chronicles", 15: "Ezra",
	16: "Nehemiah", 17: "Esther", 18: "Job", 19: "Psalm", 20: "Proverbs",
	21: "Ecclesiastes", 22: "Song of Solomon", 23: "Isaiah", 24: "Jeremiah", 25: "Lamentations",
	26: "Ezekiel", 27: "Daniel", 28: "Hosea", 29: "Joel", 30: "Amos",
	31: "Obadiah", 32: "Jonah", 33: "Micah", 34: "Nahum", 35: "Habakkuk",
	36: "Zephaniah", 37: "Haggai", 38: "Zechariah", 39: "Malachi", 40: "Matthew",
	41: "Mark", 42: "Luke", 43: "John", 44: "Acts", 45: "Romans",
	46: "1 Corinthians", 47: "2 Corinthians", 48: "Galatians", 49: "Ephesians", 50: "Philippians",
	51: "Colossians", 52: "1 Thessalonians", 53: "2 Thessalonians", 54: "1 Timothy", 55: "2 Timothy",
	56: "Titus", 57: "Philemon", 58: "Hebrews", 59: "James", 60: "1 Peter",
	61: "2 Peter", 62: "1 John", 63: "2 John", 64: "3 John", 65: "Jude", 66: "Revelation",
}

type verseInfo struct {
	book    int
	chapter int
	verse   int
}

func parseVerseID(id string) (*verseInfo, error) {
	if len(id) != 8 {
		return nil, fmt.Errorf("invalid verse ID: %s", id)
	}
	book, err := strconv.Atoi(id[0:2])
	if err != nil {
		return nil, err
	}
	chapter, err := strconv.Atoi(id[2:5])
	if err != nil {
		return nil, err
	}
	verse, err := strconv.Atoi(id[5:8])
	if err != nil {
		return nil, err
	}
	return &verseInfo{book: book, chapter: chapter, verse: verse}, nil
}

// FormatReferences converts a list of 8-digit verse IDs to a single ESV-compatible reference string.
func FormatReferences(verseIDs []string) string {
	if len(verseIDs) == 0 {
		return ""
	}

	type chapterGroup struct {
		book    int
		chapter int
		verses  []int
	}

	groups := make(map[string]*chapterGroup)
	var keys []string

	for _, id := range verseIDs {
		info, err := parseVerseID(id)
		if err != nil {
			continue
		}
		key := fmt.Sprintf("%02d-%03d", info.book, info.chapter)
		if _, ok := groups[key]; !ok {
			groups[key] = &chapterGroup{book: info.book, chapter: info.chapter}
			keys = append(keys, key)
		}
		groups[key].verses = append(groups[key].verses, info.verse)
	}

	sort.Strings(keys)

	var results []string
	for _, key := range keys {
		g := groups[key]
		sort.Ints(g.verses)

		var ranges []string
		if len(g.verses) > 0 {
			start := g.verses[0]
			end := g.verses[0]

			for i := 1; i < len(g.verses); i++ {
				if g.verses[i] == end+1 {
					end = g.verses[i]
				} else {
					if start == end {
						ranges = append(ranges, strconv.Itoa(start))
					} else {
						ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
					}
					start = g.verses[i]
					end = g.verses[i]
				}
			}
			if start == end {
				ranges = append(ranges, strconv.Itoa(start))
			} else {
				ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
			}
		}

		bookName := bookNames[g.book]
		if bookName == "" {
			bookName = fmt.Sprintf("Book %d", g.book)
		}
		results = append(results, fmt.Sprintf("%s %d:%s", bookName, g.chapter, strings.Join(ranges, ",")))
	}

	return strings.Join(results, "; ")
}
```

- [ ] **Step 1.2: Write tests for `FormatReferences` in `internal/esv/reference_test.go`.**

```go
package esv_test

import (
	"testing"
	"derrclan.com/moravian-soap/internal/esv"
)

func TestFormatReferences(t *testing.T) {
	tests := []struct {
		name     string
		verseIDs []string
		want     string
	}{
		{
			name:     "single verse",
			verseIDs: []string{"01001001"},
			want:     "Genesis 1:1",
		},
		{
			name:     "multiple verses in same chapter",
			verseIDs: []string{"01001001", "01001002", "01001003"},
			want:     "Genesis 1:1-3",
		},
		{
			name:     "non-contiguous verses",
			verseIDs: []string{"01001001", "01001003", "01001005"},
			want:     "Genesis 1:1,3,5",
		},
		{
			name:     "multiple chapters",
			verseIDs: []string{"01001001", "01002001"},
			want:     "Genesis 1:1; Genesis 2:1",
		},
		{
			name:     "mixed contiguous and non-contiguous",
			verseIDs: []string{"01001001", "01001002", "01001005", "01001006", "01001007"},
			want:     "Genesis 1:1-2,5-7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := esv.FormatReferences(tt.verseIDs); got != tt.want {
				t.Errorf("FormatReferences() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 1.3: Run tests and verify they pass.**

Run: `go test ./internal/esv/...`
Expected: PASS

### 2. Update `handleExport` to use Selected Verses

- [ ] **Step 2.1: Modify `internal/server/server.go` to use `FormatReferences` if `soapData.SelectedVerses` is present.**

```go
	// Fetch verse content from ESV API (using cache)
	references := dailyText.Verses
	if len(soapData.SelectedVerses) > 0 {
		references = []string{esv.FormatReferences(soapData.SelectedVerses)}
	}
	verseContents, err := fetchPassagesWithCache(r.Context(), references)
```

- [ ] **Step 2.2: Verify the change (manual or integration test if possible).**

Since it's hard to integration test without real API, verify by running `go build ./...` to ensure no compilation errors.

### 3. Final Verification and Cleanup

- [ ] **Step 3.1: Run all tests.**

Run: `go test ./...`
Expected: PASS

- [ ] **Step 3.2: Commit the changes.**

```bash
git add internal/esv/reference.go internal/esv/reference_test.go internal/server/server.go
git commit -m "fix: export only selected verses in email"
```
