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
