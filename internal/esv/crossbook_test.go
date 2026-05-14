package esv

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSplitCrossBookPassages(t *testing.T) {
	// Simulate what the ESV API returns for "Psalm 59:10–17;Leviticus 27:16–Numbers 1:16;Mark 9:11–29"
	// The API merges Leviticus+Numbers into one passage.
	resp := &Response{
		PassageMeta: []PassageMeta{
			{
				Canonical:    "Psalm 59:10–17",
				ChapterStart: []int{19059001, 19059017},
				ChapterEnd:   []int{19059001, 19059017},
			},
			{
				Canonical:    "Leviticus 27:16–Numbers 1:16",
				ChapterStart: []int{3027001, 3027034},
				ChapterEnd:   []int{4001001, 4001054},
			},
			{
				Canonical:    "Mark 9:11–29",
				ChapterStart: []int{41009001, 41009050},
				ChapterEnd:   []int{41009001, 41009050},
			},
		},
		Passages: []string{
			// Psalm passage
			`<h2 class="extra_text">Psalm 59:10–17</h2>
<p><span class="verse" data-ref="19059010"><b class="verse-num">10</b>My God in his steadfast love.</span></p>
<p>(<a href="http://www.esv.org" class="copyright">ESV</a>)</p>`,

			// Leviticus+Numbers merged passage (no Numbers h2)
			`<h2 class="extra_text">Leviticus 27:16–34</h2>
<p><span class="verse" data-ref="03027016"><b class="verse-num">16</b>"If a man dedicates to the LORD.</span><span class="verse" data-ref="03027034"><b class="verse-num">34</b>These are the commandments.</span></p>
<h3>A Census of Israel's Warriors</h3>
<p class="starts-chapter"><span class="verse" data-ref="04001001"><b class="chapter-num">1 </b>The LORD spoke to Moses.</span><span class="verse" data-ref="04001016"><b class="verse-num">16</b>These were the ones chosen.</span></p>
<p>(<a href="http://www.esv.org" class="copyright">ESV</a>)</p>`,

			// Mark passage
			`<h2 class="extra_text">Mark 9:11–29</h2>
<p><span class="verse" data-ref="41009011"><b class="verse-num">11</b>And they asked him.</span></p>
<p>(<a href="http://www.esv.org" class="copyright">ESV</a>)</p>`,
		},
	}

	SplitCrossBookPassages(resp)

	if len(resp.Passages) != 4 {
		t.Fatalf("Expected 4 passages after split, got %d", len(resp.Passages))
	}

	// Check passage order
	expectations := []struct {
		index          int
		mustContain    string
		mustNotContain string
	}{
		{0, "Psalm 59:10", ""},
		{1, "Leviticus 27:16", "04001001"},
		{2, "Numbers 1:1", ""},
		{3, "Mark 9:11", ""},
	}

	for _, exp := range expectations {
		p := resp.Passages[exp.index]
		if !strings.Contains(p, exp.mustContain) {
			t.Errorf("Passage %d: expected to contain %q\nGot: %.200s", exp.index, exp.mustContain, p)
		}
		if exp.mustNotContain != "" && strings.Contains(p, exp.mustNotContain) {
			t.Errorf("Passage %d: should NOT contain %q", exp.index, exp.mustNotContain)
		}
	}

	// Check that the Numbers passage has the h2 heading
	if !strings.Contains(resp.Passages[2], `<h2 class="extra_text">Numbers 1:1`) {
		t.Errorf("Numbers passage missing h2 heading.\nGot: %.300s", resp.Passages[2])
	}

	// Check that verse references in the Numbers passage start with 04
	if !strings.Contains(resp.Passages[2], `data-ref="04001001"`) {
		t.Errorf("Numbers passage should contain data-ref for Numbers verses")
	}

	// Check passage_meta length matches
	if len(resp.PassageMeta) != 4 {
		t.Fatalf("Expected 4 passage_meta entries, got %d", len(resp.PassageMeta))
	}

	t.Logf("Passage 2 (Numbers): %.300s", resp.Passages[2])
}

func TestSplitCrossBookPassages_NoCrossBook(t *testing.T) {
	resp := &Response{
		PassageMeta: []PassageMeta{
			{
				Canonical:    "Psalm 59:10–17",
				ChapterStart: []int{19059001, 19059017},
				ChapterEnd:   []int{19059001, 19059017},
			},
		},
		Passages: []string{
			`<h2 class="extra_text">Psalm 59:10–17</h2><p>Content</p>`,
		},
	}

	SplitCrossBookPassages(resp)

	if len(resp.Passages) != 1 {
		t.Fatalf("Expected 1 passage (no split needed), got %d", len(resp.Passages))
	}
}

func TestSplitCrossBookPassages_RealCachedData(t *testing.T) {
	// This is the actual cached data structure from the SQLite database
	cachedJSON := `{
		"passage_meta": [
			{"canonical": "Psalm 59:10–17", "chapter_start": [19059001, 19059017], "chapter_end": [19059001, 19059017]},
			{"canonical": "Leviticus 27:16–Numbers 1:16", "chapter_start": [3027001, 3027034], "chapter_end": [4001001, 4001054]},
			{"canonical": "Mark 9:11–29", "chapter_start": [41009001, 41009050], "chapter_end": [41009001, 41009050]}
		],
		"passages": [
			"<h2 class=\"extra_text\">Psalm 59:10–17</h2>\n<p><span class=\"verse\" data-ref=\"19059010\">psalm text</span></p>\n<p>(<a href=\"http://www.esv.org\" class=\"copyright\">ESV</a>)</p>",
			"<h2 class=\"extra_text\">Leviticus 27:16–34</h2>\n<p><span class=\"verse\" data-ref=\"03027016\">lev text</span><span class=\"verse\" data-ref=\"03027034\">lev end</span></p>\n<h3><span class=\"verse\" data-ref=\"03027034\">A Census of Israel's Warriors</span></h3>\n<p class=\"starts-chapter\"><span class=\"verse\" data-ref=\"04001001\"><b class=\"chapter-num\">1 </b>The LORD spoke to Moses.</span><span class=\"verse\" data-ref=\"04001016\">These were the ones chosen.</span></p>\n<p>(<a href=\"http://www.esv.org\" class=\"copyright\">ESV</a>)</p>",
			"<h2 class=\"extra_text\">Mark 9:11–29</h2>\n<p><span class=\"verse\" data-ref=\"41009011\">mark text</span></p>\n<p>(<a href=\"http://www.esv.org\" class=\"copyright\">ESV</a>)</p>"
		]
	}`

	var resp Response
	if err := json.Unmarshal([]byte(cachedJSON), &resp); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	SplitCrossBookPassages(&resp)

	if len(resp.Passages) != 4 {
		t.Fatalf("Expected 4 passages, got %d", len(resp.Passages))
	}

	// Verify the Numbers passage has the correct heading
	numbersPassage := resp.Passages[2]
	if !strings.Contains(numbersPassage, `<h2 class="extra_text">Numbers 1:1–16</h2>`) {
		// Also accept just "Numbers 1:1" in case of different formatting
		if !strings.Contains(numbersPassage, `Numbers 1:`) {
			t.Errorf("Numbers passage missing heading.\nGot: %s", numbersPassage)
		}
	}

	t.Logf("Numbers passage: %s", numbersPassage)
}
