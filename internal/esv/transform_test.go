package esv

import (
	"strings"
	"testing"
)

func TestProcessPassageHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string // Check for presence of these strings
	}{
		{
			name: "Basic verse wrapping",
			input: `<p>
<b class="verse-num" id="v01001001">1</b> In the beginning.
<b class="verse-num" id="v01001002">2</b> The earth was void.
</p>`,
			expected: []string{
				`<span id="v01001001" class="verse-span"><b class="verse-num">1</b> In the beginning.`,
				`<span id="v01001002" class="verse-span"><b class="verse-num">2</b> The earth was void.`,
			},
		},
		{
			name: "Multi-block verse (Poetry)",
			input: `<div class="poetry">
<b id="v23063008">8</b> He said, "Surely they are my people"
</div>
<div class="poetry">
children who will not deal falsely.
</div>`,
			expected: []string{
				// First block wrapped
				`<span id="v23063008" class="verse-span"><b>8</b> He said, &#34;Surely they are my people&#34;`,
				// Second block wrapped with SAME ID
				// We just check that the text is inside the span with the correct ID
				`id="v23063008"`,
				`children who will not deal falsely.`,
				`</span>`,
			},
		},
		{
			name: "Verse crossing Paragraphs",
			input: `<p><b id="v1">1</b> Start of verse 1.</p>
<p>End of verse 1. <b id="v2">2</b> Start of verse 2.</p>`,
			expected: []string{
				`<span id="v1" class="verse-span"><b>1</b> Start of verse 1.</span>`,
				`<span id="v1" class="verse-span">End of verse 1. </span>`,
				`<span id="v2" class="verse-span"><b>2</b> Start of verse 2.</span>`,
			},
		},
		{
			name: "Deeply nested verse markers (ESV Poetry)",
			input: `<p class="block-indent">
<span class="line"><b class="verse-num" id="v19148007-1">7</b> Praise the LORD</span><br>
<span class="line">you great sea creatures</span>
</p>`,
			expected: []string{
				// Marker inside span should be found and wrapped.
				// Note: The wrapper is INSIDE the span.line.
				// We check for the sequence: wrapper start -> b tag with class -> text
				`<span id="v19148007-1" class="verse-span"><b class="verse-num">7</b> Praise the LORD</span>`,
				// Subsequent line should also be wrapped
				`<span id="v19148007-1" class="verse-span">you great sea creatures</span>`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := processPassageHTML(tt.input)
			if err != nil {
				t.Fatalf("processPassageHTML() error = %v", err)
			}

			for _, exp := range tt.expected {
				// Simplify whitespace for comparison
				// This is a naive check; ideally we'd parse and compare DOMs or use a robust normalizer
				// For now, let's just check if the KEY parts are there.
				if !strings.Contains(normalize(got), normalize(exp)) {
					t.Errorf("Result missing expected substring.\nExpected chunk:\n%s\n\nGot full output:\n%s", exp, got)
				}
			}
		})
	}
}

// normalize removes newlines and extra spaces for loose comparison
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\n", "")
	return strings.Join(strings.Fields(s), " ")
}
