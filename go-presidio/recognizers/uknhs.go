package recognizers

import (
	"regexp"
	"strconv"

	"github.com/karldane/go-presidio/presidio"
)

var ukNhsRegex = regexp.MustCompile(`\b\d{3}[ -]?\d{3}[ -]?\d{4}\b`)

type UKNHSRecognizer struct{}

func NewUKNHSRecognizer() *UKNHSRecognizer {
	return &UKNHSRecognizer{}
}

func (r *UKNHSRecognizer) Name() string { return "UKNHSRecognizer" }
func (r *UKNHSRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityUkNhsNumber}
}

func (r *UKNHSRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := ukNhsRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		num := value[match[0]:match[1]]
		num = removeDashes(num)
		if len(num) == 10 && nhsChecksumValid(num) {
			results = append(results, presidio.RecognizerResult{
				EntityType: presidio.EntityUkNhsNumber,
				Start:      match[0],
				End:        match[1],
				Score:      0.85,
				Recognizer: r.Name(),
			})
		}
	}
	return results
}

func nhsChecksumValid(nhs string) bool {
	sum := 0
	for i := 0; i < 9; i++ {
		d, _ := strconv.Atoi(string(nhs[i]))
		sum += d * (11 - i - 1)
	}
	mod := sum % 11
	checksumDigit := 11 - mod
	if checksumDigit == 11 {
		checksumDigit = 0
	}
	lastChar := nhs[9:]
	if lastChar == "X" {
		lastChar = "10"
	}
	check, _ := strconv.Atoi(lastChar)
	return check == checksumDigit
}

func removeDashes(s string) string {
	result := ""
	for _, c := range s {
		if c != '-' && c != ' ' {
			result += string(c)
		}
	}
	return result
}
