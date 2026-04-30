package recognizers

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var usSsnRegex = regexp.MustCompile(`\b\d{9}\b|\b\d{3}[-.]?\d{2}[-.]?\d{4}\b`)

var woolworthBlocked = "078-05-1120"

type UsSsnRecognizer struct{}

func NewUsSsnRecognizer() *UsSsnRecognizer {
	return &UsSsnRecognizer{}
}

func (r *UsSsnRecognizer) Name() string { return "UsSsnRecognizer" }
func (r *UsSsnRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityUsSsn}
}

func (r *UsSsnRecognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := usSsnRegex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		raw := value[match[0]:match[1]]
		ssn := strings.ReplaceAll(strings.ReplaceAll(raw, "-", "."), " ", "")
		parts := strings.Split(ssn, ".")
		var area, group, serial int
		var err error
		if len(parts) != 3 {
			if len(ssn) == 9 {
				area, err = strconv.Atoi(ssn[:3])
				if err == nil {
					group, err = strconv.Atoi(ssn[3:5])
				}
				if err == nil {
					serial, err = strconv.Atoi(ssn[5:9])
				}
			} else {
				continue
			}
		} else {
			area, _ = strconv.Atoi(parts[0])
			group, _ = strconv.Atoi(parts[1])
			serial, _ = strconv.Atoi(parts[2])
		}
		if err != nil || !isValidSsn(area, group, serial) || isWoolworthBlocked(raw) {
			continue
		}
		results = append(results, presidio.RecognizerResult{
			EntityType: presidio.EntityUsSsn,
			Start:      match[0],
			End:        match[1],
			Score:      0.85,
			Recognizer: r.Name(),
		})
	}
	return results
}

func isValidSsn(area, group, serial int) bool {
	if area == 0 || area == 666 {
		return false
	}
	if group == 0 {
		return false
	}
	if serial == 0 {
		return false
	}
	return true
}

func isWoolworthBlocked(ssn string) bool {
	normalized := strings.ReplaceAll(strings.ToLower(ssn), "-", "")
	normalizedBlocked := strings.ReplaceAll(strings.ToLower(woolworthBlocked), "-", "")
	return normalized == normalizedBlocked
}
