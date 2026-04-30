package recognizers

import (
	"regexp"

	"github.com/karldane/go-presidio/presidio"
)

var ukPostcodeRegex = regexp.MustCompile(`\b[A-Z]{1,2}[0-9][A-Z0-9]?\s?[0-9][A-Z]{2}\b`)

type UkPostcodeRecognizer struct{}

func NewUkPostcodeRecognizer() *UkPostcodeRecognizer {
	return &UkPostcodeRecognizer{}
}

func (r *UkPostcodeRecognizer) Name() string { return "UkPostcodeRecognizer" }
func (r *UkPostcodeRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityUkPostcode}
}

func (r *UkPostcodeRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := ukPostcodeRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		results = append(results, presidio.RecognizerResult{
			EntityType: presidio.EntityUkPostcode,
			Start:      match[0],
			End:        match[1],
			Score:      0.85,
			Recognizer: r.Name(),
		})
	}
	return results
}
