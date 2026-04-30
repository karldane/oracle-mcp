package recognizers

import (
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/karldane/go-presidio/presidio"
)

var (
	dobSlashRegex = regexp.MustCompile(`\b\d{1,2}[-/]\d{1,2}[-/]\d{2,4}\b`)
	dobIsoRegex   = regexp.MustCompile(`\b\d{4}-\d{2}-\d{2}\b`)
	dobMonRegex   = regexp.MustCompile(`\b\d{1,2}-[A-Za-z]{3}-\d{4}\b`)
)

type DobRecognizer struct{}

func NewDobRecognizer() *DobRecognizer {
	return &DobRecognizer{}
}

func (r *DobRecognizer) Name() string { return "DobRecognizer" }
func (r *DobRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityDateOfBirth}
}

func (r *DobRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	seen := make(map[int]bool)

	for _, regex := range []*regexp.Regexp{dobSlashRegex, dobIsoRegex, dobMonRegex} {
		matches := regex.FindAllStringSubmatchIndex(value, -1)
		for _, match := range matches {
			if !seen[match[0]] {
				seen[match[0]] = true
				matched := value[match[0]:match[1]]
				if isValidDate(matched) {
					results = append(results, presidio.RecognizerResult{
						EntityType: presidio.EntityDateOfBirth,
						Start:      match[0],
						End:        match[1],
						Score:      0.85,
						Recognizer: r.Name(),
					})
				}
			}
		}
	}
	return results
}

func isValidDate(s string) bool {
	monthNames := map[string]int{
		"JAN": 1, "FEB": 2, "MAR": 3, "APR": 4, "MAY": 5, "JUN": 6,
		"JUL": 7, "AUG": 8, "SEP": 9, "OCT": 10, "NOV": 11, "DEC": 12,
	}
	upper := strings.ToUpper(s)
	for name := range monthNames {
		if strings.Contains(upper, name) {
			parts := strings.Split(upper, "-")
			if len(parts) == 3 {
				day, err := strconv.Atoi(parts[0])
				if err != nil || day < 1 || day > 31 {
					return false
				}
				year, err := strconv.Atoi(parts[2])
				if err != nil || year < 1900 || year > 2100 {
					return false
				}
				return true
			}
			return false
		}
	}
	formats := []string{
		"2006-01-02",
		"02/01/2006",
		"01/02/2006",
		"2/1/2006",
		"1/2/2006",
		"02-01-2006",
		"01-02-2006",
	}
	for _, fmt := range formats {
		if _, err := time.Parse(fmt, s); err == nil {
			parts := strings.Split(s, "-/")
			if len(parts) == 3 {
				if len(parts[0]) == 4 {
					year, _ := strconv.Atoi(parts[0])
					if year < 1900 || year > 2100 {
						return false
					}
				}
				month, _ := strconv.Atoi(parts[1])
				if month < 1 || month > 12 {
					return false
				}
				day, _ := strconv.Atoi(parts[2])
				if day < 1 || day > 31 {
					return false
				}
			}
			return true
		}
	}
	return false
}
