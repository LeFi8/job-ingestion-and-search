package langdetect

import (
	"github.com/pemistahl/lingua-go"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

const MinRelativeDistance = 0.1

type Lingua struct {
	detector lingua.LanguageDetector
}

func New(minRelativeDistance float64) *Lingua {
	detector := lingua.NewLanguageDetectorBuilder().
		FromLanguages(
			lingua.English,
			lingua.French,
			lingua.German,
			lingua.Spanish,
			lingua.Italian,
			lingua.Portuguese,
			lingua.Dutch,
			lingua.Polish,
		).
		WithMinimumRelativeDistance(minRelativeDistance).
		WithPreloadedLanguageModels().
		Build()

	return &Lingua{detector: detector}
}

func (l *Lingua) Detect(text string) (posting.Language, bool) {
	lang, reliable := l.detector.DetectLanguageOf(text)
	if !reliable {
		return "", false
	}

	switch lang {
	case lingua.English:
		return posting.English, true
	case lingua.French:
		return posting.French, true
	case lingua.German:
		return posting.German, true
	case lingua.Polish:
		return posting.Polish, true
	default:
		return "", false
	}
}
