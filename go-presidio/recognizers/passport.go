package recognizers

import (
	"regexp"

	"github.com/karldane/go-presidio/presidio"
)

var (
	passportAlphaRegex  = regexp.MustCompile(`\b[A-Z][0-9]{8}\b`)
	passportAlphaRegex2 = regexp.MustCompile(`\b[A-Z]{2}[0-9]{7,8}\b`)
	passportNumRegex    = regexp.MustCompile(`\b[0-9]{8,9}\b`)
)

type PassportRecognizer struct{}

func NewPassportRecognizer() *PassportRecognizer {
	return &PassportRecognizer{}
}

func (r *PassportRecognizer) Name() string { return "PassportRecognizer" }
func (r *PassportRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityPassportNumber}
}

func (r *PassportRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	seen := make(map[int]bool)

	for _, regex := range []*regexp.Regexp{passportAlphaRegex, passportAlphaRegex2, passportNumRegex} {
		matches := regex.FindAllStringSubmatchIndex(value, -1)
		for _, match := range matches {
			if !seen[match[0]] {
				seen[match[0]] = true
				results = append(results, presidio.RecognizerResult{
					EntityType: presidio.EntityPassportNumber,
					Start:      match[0],
					End:        match[1],
					Score:      0.1,
					Recognizer: r.Name(),
				})
			}
		}
	}
	return results
}
