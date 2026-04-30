package recognizers

import (
	"regexp"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var emailRegex = regexp.MustCompile(`[a-zA-Z0-9.!#$%&'*+/=?^_{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)+`)

type EmailRecognizer struct{}

func NewEmailRecognizer() *EmailRecognizer {
	return &EmailRecognizer{}
}

func (r *EmailRecognizer) Name() string {
	return "EmailRecognizer"
}

func (r *EmailRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityEmailAddress}
}

func (r *EmailRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := emailRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		start, end := match[0], match[1]
		matched := value[start:end]
		if !strings.Contains(matched, "@.") && !strings.HasPrefix(matched, "@") && !strings.HasSuffix(matched, ".") && strings.Count(matched, "@") == 1 {
			results = append(results, presidio.RecognizerResult{
				EntityType: presidio.EntityEmailAddress,
				Start:      start,
				End:        end,
				Score:      0.85,
				Recognizer: r.Name(),
			})
		}
	}
	return results
}
