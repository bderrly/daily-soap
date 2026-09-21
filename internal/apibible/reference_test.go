package apibible_test

import (
	"reflect"
	"testing"

	"github.com/bderrly/daily-soap/internal/apibible"
)

func TestParseReference(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{
			input:    "Psalm 1",
			expected: []string{"PSA.1"},
		},
		{
			input:    "John 3:16",
			expected: []string{"JHN.3.16"},
		},
		{
			input:    "Genesis 1:1–2:3",
			expected: []string{"GEN.1.1-GEN.2.3"},
		},
		{
			input:    "Genesis 3,4",
			expected: []string{"GEN.3", "GEN.4"},
		},
		{
			input:    "1 Corinthians 12:12-31a",
			expected: []string{"1CO.12.12-1CO.12.31"},
		},
		{
			input:    "John 1:(1-9),10-18",
			expected: []string{"JHN.1.1-JHN.1.18"},
		},
		{
			input:    "Exodus 40:24–Leviticus 1:17",
			expected: []string{"EXO.40.24-LEV.1.17"},
		},
		{
			input:    "2 Peter 3:8-15a, Mark 1:1-8",
			expected: []string{"2PE.3.8-2PE.3.15", "MRK.1.1-MRK.1.8"},
		},
		{
			input:    "3 John 5",
			expected: []string{"3JN.1.5"},
		},
		{
			input:    "2 John",
			expected: []string{"2JN.1"},
		},
		{
			input:    "Corinthians 1:1–9",
			expected: []string{"1CO.1.1-1CO.1.9"},
		},
		{
			input:    "Thessalonians 1:5a",
			expected: []string{"1TH.1.5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := apibible.ParseReference(tt.input)
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.expected) {
				t.Errorf("ParseReference(%q) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestParseVerseRef(t *testing.T) {
	b, chap, verse, err := apibible.ParseVerseRef("43003016")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if b.USFM != "JHN" || chap != 3 || verse != 16 {
		t.Errorf("got %s %d:%d; want JHN 3:16", b.USFM, chap, verse)
	}
}
