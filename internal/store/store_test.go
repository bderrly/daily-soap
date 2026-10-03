package store_test

import (
	"testing"

	"github.com/bderrly/daily-soap/internal/store"
)

func TestSOAPData_IsEmpty(t *testing.T) {
	tests := []struct {
		name     string
		data     *store.SOAPData
		expected bool
	}{
		{
			name:     "nil receiver",
			data:     nil,
			expected: true,
		},
		{
			name:     "empty struct",
			data:     &store.SOAPData{},
			expected: true,
		},
		{
			name: "whitespace only fields",
			data: &store.SOAPData{
				Observation:    "   \n\t  ",
				Application:    "  ",
				Prayer:         "   ",
				SelectedVerses: []string{},
				Translation:    "ESV",
			},
			expected: true,
		},
		{
			name: "translation set but no content",
			data: &store.SOAPData{
				Date:        "2026-10-02",
				Translation: "NLT",
			},
			expected: true,
		},
		{
			name: "has observation",
			data: &store.SOAPData{
				Observation: "Some thought",
			},
			expected: false,
		},
		{
			name: "has application",
			data: &store.SOAPData{
				Application: "Action item",
			},
			expected: false,
		},
		{
			name: "has prayer",
			data: &store.SOAPData{
				Prayer: "Prayer request",
			},
			expected: false,
		},
		{
			name: "has selected verses",
			data: &store.SOAPData{
				SelectedVerses: []string{"John 3:16"},
			},
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.data.IsEmpty(); got != tc.expected {
				t.Errorf("expected IsEmpty() = %v, got %v", tc.expected, got)
			}
		})
	}
}
