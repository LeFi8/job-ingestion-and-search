package langdetect_test

import (
	"testing"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/langdetect"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

func TestLinguaFixture(t *testing.T) {
	// Catches: threshold change silently flipping borderlines or misconfigured language detector.
	if testing.Short() {
		t.Skip("skipping slow lingua model test in short mode")
	}

	detector := langdetect.New()

	tests := []struct {
		name         string
		text         string
		wantLang     posting.Language
		wantReliable bool
	}{
		{
			name:         "French job description",
			text:         "Rejoignez notre équipe pour concevoir des systèmes distribués performants.",
			wantLang:     posting.French,
			wantReliable: true,
		},
		{
			name:         "English job description",
			text:         "Join our backend team to build scalable services.",
			wantLang:     posting.English,
			wantReliable: true,
		},
		{
			name:         "Spanish job description",
			text:         "Buscamos un desarrollador de software con experiencia en sistemas distribuidos y microservicios.",
			wantLang:     posting.Spanish,
			wantReliable: true,
		},
		{
			name:         "Short borderline description is unreliable",
			text:         "Develop mobile applications.",
			wantLang:     "",
			wantReliable: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLang, gotReliable := detector.Detect(tc.text)
			if gotReliable != tc.wantReliable {
				t.Errorf("Detect(%q) reliable = %v, want %v (gotLang=%q)", tc.text, gotReliable, tc.wantReliable, gotLang)
			}
			if gotLang != tc.wantLang {
				t.Errorf("Detect(%q) lang = %q, want %q", tc.text, gotLang, tc.wantLang)
			}
		})
	}
}
