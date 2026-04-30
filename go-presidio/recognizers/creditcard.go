package recognizers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var creditCardRegex = regexp.MustCompile(`\b(?:\d{4}[- ]?){3}\d{1,4}\b`)

type CreditCardRecognizer struct{}

func NewCreditCardRecognizer() *CreditCardRecognizer {
	return &CreditCardRecognizer{}
}

func (r *CreditCardRecognizer) Name() string { return "CreditCardRecognizer" }
func (r *CreditCardRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityCreditCard}
}

func (r *CreditCardRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := creditCardRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		raw := value[match[0]:match[1]]
		num := strings.ReplaceAll(raw, "-", "")
		num = strings.ReplaceAll(num, " ", "")
		if len(num) >= 13 && luhnValid(num) {
			results = append(results, presidio.RecognizerResult{
				EntityType: presidio.EntityCreditCard,
				Start:      match[0],
				End:        match[1],
				Score:      0.85,
				Recognizer: r.Name(),
			})
		}
	}
	return results
}

func luhnCheck(num string) int {
	sum := 0
	for i := len(num) - 2; i >= 0; i -= 2 {
		d, _ := strconv.Atoi(string(num[i]))
		d *= 2
		if d > 9 {
			d -= 9
		}
		sum += d
	}
	for i := len(num) - 1; i >= 0; i -= 2 {
		d, _ := strconv.Atoi(string(num[i]))
		sum += d
	}
	return sum % 10
}

func luhnValid(num string) bool {
	allZeros := true
	for _, c := range num {
		if c != '0' {
			allZeros = false
			break
		}
	}
	if allZeros {
		return false
	}
	if luhnCheck(num) == 0 {
		return true
	}
	for _, prefix := range []string{"4", "5", "37", "6"} {
		if strings.HasPrefix(num, prefix) {
			if luhnCheck("0"+num) == 0 {
				return true
			}
		}
	}
	return false
}
