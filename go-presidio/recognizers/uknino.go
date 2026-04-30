package recognizers

import (
	"regexp"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var ukNinoRegex = regexp.MustCompile(`(?i)\b[A-Z]{2}\s?\d{2}\s?\d{2}\s?\d{2}\s?[A-Z]\b`)

var invalidPrefix1 = map[string]bool{"D": true, "F": true, "I": true, "O": true, "Q": true, "U": true}
var invalidSuffix = map[string]bool{"C": true, "E": true, "G": true, "H": true, "I": true, "M": true, "O": true, "Q": true, "U": true, "V": true, "Z": true}

type UkNinoRecognizer struct{}

func NewUkNinoRecognizer() *UkNinoRecognizer {
	return &UkNinoRecognizer{}
}

func (r *UkNinoRecognizer) Name() string { return "UkNinoRecognizer" }
func (r *UkNinoRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityUkNino}
}

func (r *UkNinoRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := ukNinoRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		raw := value[match[0]:match[1]]
		if !isValidNino(raw) {
			continue
		}
		results = append(results, presidio.RecognizerResult{
			EntityType: presidio.EntityUkNino,
			Start:      match[0],
			End:        match[1],
			Score:      0.85,
			Recognizer: r.Name(),
		})
	}
	return results
}

func isValidNino(nino string) bool {
	clean := strings.ToUpper(strings.ReplaceAll(nino, " ", ""))
	if len(clean) < 9 {
		return false
	}
	prefix1 := string(clean[0])
	if invalidPrefix1[prefix1] {
		return false
	}
	group := clean[2:4]
	if group == "00" || group == "99" {
		return false
	}
	suffix := string(clean[8])
	if invalidSuffix[suffix] {
		return false
	}
	return true
}
