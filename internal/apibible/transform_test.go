package apibible_test

import (
	"strings"
	"testing"

	"github.com/bderrly/daily-soap/internal/apibible"
)

func TestProcessPassageHTML(t *testing.T) {
	input := `<p class="s1">God So Loved the World</p><p class="p"><span data-number="16" class="v">16</span>For God so loved the world, that he gave his only begotten Son. <span data-number="17" class="v">17</span>For God sent not his Son into the world to condemn the world.</p>`

	// John = book 43, chapter 3
	got, err := apibible.ProcessPassageHTML(input, 43, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(got, `data-ref="43003016"`) {
		t.Errorf("expected data-ref 43003016 in output, got %s", got)
	}
	if !strings.Contains(got, `data-ref="43003017"`) {
		t.Errorf("expected data-ref 43003017 in output, got %s", got)
	}
	if !strings.Contains(got, `<b class="verse-num">16</b>`) {
		t.Errorf("expected formatted verse num 16, got %s", got)
	}
	if !strings.Contains(got, `<b class="verse-num">17</b>`) {
		t.Errorf("expected formatted verse num 17, got %s", got)
	}
	if !strings.Contains(got, `class="verse"`) {
		t.Errorf("expected class verse, got %s", got)
	}
}
