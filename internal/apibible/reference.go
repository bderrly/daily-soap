// Package apibible provides a client and utilities for accessing scripture from API.Bible.
package apibible

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// BookInfo stores metadata about a Bible book.
type BookInfo struct {
	Number int
	USFM   string
	Name   string
}

var booksList = []BookInfo{
	{Number: 1, USFM: "GEN", Name: "Genesis"},
	{Number: 2, USFM: "EXO", Name: "Exodus"},
	{Number: 3, USFM: "LEV", Name: "Leviticus"},
	{Number: 4, USFM: "NUM", Name: "Numbers"},
	{Number: 5, USFM: "DEU", Name: "Deuteronomy"},
	{Number: 6, USFM: "JOS", Name: "Joshua"},
	{Number: 7, USFM: "JDG", Name: "Judges"},
	{Number: 8, USFM: "RUT", Name: "Ruth"},
	{Number: 9, USFM: "1SA", Name: "1 Samuel"},
	{Number: 10, USFM: "2SA", Name: "2 Samuel"},
	{Number: 11, USFM: "1KI", Name: "1 Kings"},
	{Number: 12, USFM: "2KI", Name: "2 Kings"},
	{Number: 13, USFM: "1CH", Name: "1 Chronicles"},
	{Number: 14, USFM: "2CH", Name: "2 Chronicles"},
	{Number: 15, USFM: "EZR", Name: "Ezra"},
	{Number: 16, USFM: "NEH", Name: "Nehemiah"},
	{Number: 17, USFM: "EST", Name: "Esther"},
	{Number: 18, USFM: "JOB", Name: "Job"},
	{Number: 19, USFM: "PSA", Name: "Psalm"},
	{Number: 20, USFM: "PRO", Name: "Proverbs"},
	{Number: 21, USFM: "ECC", Name: "Ecclesiastes"},
	{Number: 22, USFM: "SNG", Name: "Song of Solomon"},
	{Number: 23, USFM: "ISA", Name: "Isaiah"},
	{Number: 24, USFM: "JER", Name: "Jeremiah"},
	{Number: 25, USFM: "LAM", Name: "Lamentations"},
	{Number: 26, USFM: "EZK", Name: "Ezekiel"},
	{Number: 27, USFM: "DAN", Name: "Daniel"},
	{Number: 28, USFM: "HOS", Name: "Hosea"},
	{Number: 29, USFM: "JOL", Name: "Joel"},
	{Number: 30, USFM: "AMO", Name: "Amos"},
	{Number: 31, USFM: "OBA", Name: "Obadiah"},
	{Number: 32, USFM: "JON", Name: "Jonah"},
	{Number: 33, USFM: "MIC", Name: "Micah"},
	{Number: 34, USFM: "NAM", Name: "Nahum"},
	{Number: 35, USFM: "HAB", Name: "Habakkuk"},
	{Number: 36, USFM: "ZEP", Name: "Zephaniah"},
	{Number: 37, USFM: "HAG", Name: "Haggai"},
	{Number: 38, USFM: "ZEC", Name: "Zechariah"},
	{Number: 39, USFM: "MAL", Name: "Malachi"},
	{Number: 40, USFM: "MAT", Name: "Matthew"},
	{Number: 41, USFM: "MRK", Name: "Mark"},
	{Number: 42, USFM: "LUK", Name: "Luke"},
	{Number: 43, USFM: "JHN", Name: "John"},
	{Number: 44, USFM: "ACT", Name: "Acts"},
	{Number: 45, USFM: "ROM", Name: "Romans"},
	{Number: 46, USFM: "1CO", Name: "1 Corinthians"},
	{Number: 47, USFM: "2CO", Name: "2 Corinthians"},
	{Number: 48, USFM: "GAL", Name: "Galatians"},
	{Number: 49, USFM: "EPH", Name: "Ephesians"},
	{Number: 50, USFM: "PHP", Name: "Philippians"},
	{Number: 51, USFM: "COL", Name: "Colossians"},
	{Number: 52, USFM: "1TH", Name: "1 Thessalonians"},
	{Number: 53, USFM: "2TH", Name: "2 Thessalonians"},
	{Number: 54, USFM: "1TI", Name: "1 Timothy"},
	{Number: 55, USFM: "2TI", Name: "2 Timothy"},
	{Number: 56, USFM: "TIT", Name: "Titus"},
	{Number: 57, USFM: "PHM", Name: "Philemon"},
	{Number: 58, USFM: "HEB", Name: "Hebrews"},
	{Number: 59, USFM: "JAS", Name: "James"},
	{Number: 60, USFM: "1PE", Name: "1 Peter"},
	{Number: 61, USFM: "2PE", Name: "2 Peter"},
	{Number: 62, USFM: "1JN", Name: "1 John"},
	{Number: 63, USFM: "2JN", Name: "2 John"},
	{Number: 64, USFM: "3JN", Name: "3 John"},
	{Number: 65, USFM: "JUD", Name: "Jude"},
	{Number: 66, USFM: "REV", Name: "Revelation"},
}

var (
	nameToBook     = make(map[string]BookInfo)
	usfmToBook     = make(map[string]BookInfo)
	numberToBook   = make(map[int]BookInfo)
	sortedBookKeys []string
)

func init() {
	for _, b := range booksList {
		nameToBook[strings.ToLower(b.Name)] = b
		usfmToBook[b.USFM] = b
		numberToBook[b.Number] = b
	}
	// Aliases
	nameToBook["psalms"] = nameToBook["psalm"]
	nameToBook["song of songs"] = nameToBook["song of solomon"]
	nameToBook["corinthians"] = nameToBook["1 corinthians"]
	nameToBook["thessalonians"] = nameToBook["1 thessalonians"]

	for k := range nameToBook {
		sortedBookKeys = append(sortedBookKeys, k)
	}
	// Sort by length descending so longer book names match first (e.g. "1 Corinthians" before "Corinthians")
	sort.Slice(sortedBookKeys, func(i, j int) bool {
		return len(sortedBookKeys[i]) > len(sortedBookKeys[j])
	})
}

// FindBook resolves a book name or alias to BookInfo.
func FindBook(name string) (BookInfo, bool) {
	b, ok := nameToBook[strings.ToLower(strings.TrimSpace(name))]
	return b, ok
}

// USFMToBookInfo resolves a 3-letter USFM code to BookInfo.
func USFMToBookInfo(usfm string) (BookInfo, bool) {
	b, ok := usfmToBook[strings.ToUpper(strings.TrimSpace(usfm))]
	return b, ok
}

// NumberToBookInfo resolves a 1-66 book index to BookInfo.
func NumberToBookInfo(num int) (BookInfo, bool) {
	b, ok := numberToBook[num]
	return b, ok
}

type bookMatch struct {
	start int
	end   int
	book  BookInfo
}

func findBooksInString(s string) []bookMatch {
	lower := strings.ToLower(s)
	var matches []bookMatch

	for _, k := range sortedBookKeys {
		idx := 0
		for {
			pos := strings.Index(lower[idx:], k)
			if pos == -1 {
				break
			}
			actualPos := idx + pos
			endPos := actualPos + len(k)
			idx = endPos

			// Check word boundary before and after
			if actualPos > 0 {
				prev := lower[actualPos-1]
				if (prev >= 'a' && prev <= 'z') || (prev >= '0' && prev <= '9') {
					continue
				}
			}
			if endPos < len(lower) {
				next := lower[endPos]
				if (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
					continue
				}
			}

			// Ensure not overlapping an earlier matched longer key
			overlap := false
			for _, m := range matches {
				if actualPos >= m.start && actualPos < m.end {
					overlap = true
					break
				}
			}
			if !overlap {
				matches = append(matches, bookMatch{
					start: actualPos,
					end:   endPos,
					book:  nameToBook[k],
				})
			}
		}
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].start < matches[j].start
	})
	return matches
}

// ParseReference converts human-readable references (such as "John 3:16-18", "Psalm 1",
// "Genesis 1:1–2:3", "Genesis 3,4") into API.Bible Passage IDs (e.g. "JHN.3.16-JHN.3.18").
func ParseReference(ref string) ([]string, error) {
	// Normalize dashes and spaces
	s := strings.ReplaceAll(ref, "\u2013", "-")
	s = strings.ReplaceAll(s, "\u2014", "-")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, nil
	}

	matches := findBooksInString(s)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no recognized Bible book in reference: %q", ref)
	}

	// Handle multiple references with distinct books
	if len(matches) > 1 {
		between := strings.TrimSpace(s[matches[0].end:matches[1].start])
		isCrossBook := strings.HasSuffix(between, "-")

		if !isCrossBook {
			parts := strings.Split(s, ", ")
			if len(parts) > 1 {
				var all []string
				for _, p := range parts {
					res, err := ParseReference(p)
					if err != nil {
						return nil, err
					}
					all = append(all, res...)
				}
				return all, nil
			}
		} else {
			// Cross-book range like "Exodus 40:24-Leviticus 1:17" or "Isaiah 66-Jeremiah 1:7"
			b1 := matches[0].book
			b2 := matches[1].book
			dashIdx := strings.LastIndex(s[:matches[1].start], "-")
			p1 := strings.TrimSpace(s[matches[0].end:dashIdx])
			p2 := strings.TrimSpace(s[matches[1].end:])

			v1 := formatPoint(b1, p1, true)
			v2 := formatPoint(b2, p2, false)
			return []string{fmt.Sprintf("%s.%s-%s.%s", b1.USFM, v1, b2.USFM, v2)}, nil
		}
	}

	// Single book reference
	b := matches[0].book
	rest := strings.TrimSpace(s[matches[0].end:])
	if rest == "" {
		// e.g. "2 John"
		return []string{fmt.Sprintf("%s.1", b.USFM)}, nil
	}

	// Handle parenthesized compound ranges e.g. 1:(1-9),10-18 -> 1:1-18
	fullParenRegex := regexp.MustCompile(`(\d+):\((\d+)-\d+\),\d+-(\d+)`)
	rest = fullParenRegex.ReplaceAllString(rest, "$1:$2-$3")

	endParenRegex := regexp.MustCompile(`(\d+):(\d+)-\d+,\((?:\d+[a-z]?-)?(\d+)[a-z]?\)`)
	rest = endParenRegex.ReplaceAllString(rest, "$1:$2-$3")

	// Strip trailing letter suffixes from verses (e.g. 31a -> 31)
	letterSuffixRegex := regexp.MustCompile(`(\d+)[a-z]\b`)
	rest = letterSuffixRegex.ReplaceAllString(rest, "$1")

	// Clean any remaining parentheses
	otherParenRegex := regexp.MustCompile(`[()]`)
	rest = otherParenRegex.ReplaceAllString(rest, "")
	rest = strings.TrimSpace(rest)

	// Multi-chapter comma: e.g. "3,4" -> ["GEN.3", "GEN.4"]
	multiChapterRegex := regexp.MustCompile(`^(\d+),\s*(\d+)$`)
	if m := multiChapterRegex.FindStringSubmatch(rest); m != nil {
		return []string{fmt.Sprintf("%s.%s", b.USFM, m[1]), fmt.Sprintf("%s.%s", b.USFM, m[2])}, nil
	}

	// Just a chapter number e.g. "12"
	justNumRegex := regexp.MustCompile(`^\d+$`)
	if justNumRegex.MatchString(rest) {
		// Single-chapter books like Obadiah, Philemon, 2 John, 3 John, Jude
		if isSingleChapterBook(b.USFM) {
			return []string{fmt.Sprintf("%s.1.%s", b.USFM, rest)}, nil
		}
		return []string{fmt.Sprintf("%s.%s", b.USFM, rest)}, nil
	}

	// Single-chapter books with verse range like "3 John 5" -> "3JN.1.5"
	if isSingleChapterBook(b.USFM) && !strings.Contains(rest, ":") {
		if strings.Contains(rest, "-") {
			parts := strings.Split(rest, "-")
			return []string{fmt.Sprintf("%s.1.%s-%s.1.%s", b.USFM, strings.TrimSpace(parts[0]), b.USFM, strings.TrimSpace(parts[1]))}, nil
		}
		return []string{fmt.Sprintf("%s.1.%s", b.USFM, rest)}, nil
	}

	// Range with dash
	if strings.Contains(rest, "-") {
		parts := strings.Split(rest, "-")
		startPart := strings.TrimSpace(parts[0])
		endPart := strings.TrimSpace(parts[1])

		if strings.Contains(startPart, ":") {
			sp := strings.Split(startPart, ":")
			c1, v1 := sp[0], sp[1]
			if strings.Contains(endPart, ":") {
				ep := strings.Split(endPart, ":")
				c2, v2 := ep[0], ep[1]
				return []string{fmt.Sprintf("%s.%s.%s-%s.%s.%s", b.USFM, c1, v1, b.USFM, c2, v2)}, nil
			}
			return []string{fmt.Sprintf("%s.%s.%s-%s.%s.%s", b.USFM, c1, v1, b.USFM, c1, endPart)}, nil
		}
		// Chapter range e.g. "1-3"
		return []string{fmt.Sprintf("%s.%s-%s.%s", b.USFM, startPart, b.USFM, endPart)}, nil
	}

	// Single verse: "3:16"
	if strings.Contains(rest, ":") {
		sp := strings.Split(rest, ":")
		return []string{fmt.Sprintf("%s.%s.%s", b.USFM, sp[0], sp[1])}, nil
	}

	return []string{fmt.Sprintf("%s.%s", b.USFM, rest)}, nil
}

func isSingleChapterBook(usfm string) bool {
	switch usfm {
	case "OBA", "PHM", "2JN", "3JN", "JUD":
		return true
	default:
		return false
	}
}

func formatPoint(_ BookInfo, p string, isStart bool) string {
	p = strings.TrimSpace(p)
	if strings.Contains(p, ":") {
		return strings.ReplaceAll(p, ":", ".")
	}
	if isStart {
		return p + ".1"
	}
	return p
}

// ParseVerseRef converts an 8-digit verse ID ("BBCCCVVV") to BookInfo, chapter, and verse.
func ParseVerseRef(ref string) (BookInfo, int, int, error) {
	if len(ref) != 8 {
		return BookInfo{}, 0, 0, fmt.Errorf("invalid verse reference length: %q", ref)
	}
	bNum, err := strconv.Atoi(ref[:2])
	if err != nil {
		return BookInfo{}, 0, 0, fmt.Errorf("parsing book number: %w", err)
	}
	b, ok := numberToBook[bNum]
	if !ok {
		return BookInfo{}, 0, 0, fmt.Errorf("unknown book number: %d", bNum)
	}
	chap, err := strconv.Atoi(ref[2:5])
	if err != nil {
		return BookInfo{}, 0, 0, fmt.Errorf("parsing chapter number: %w", err)
	}
	verse, err := strconv.Atoi(ref[5:8])
	if err != nil {
		return BookInfo{}, 0, 0, fmt.Errorf("parsing verse number: %w", err)
	}
	return b, chap, verse, nil
}
