package recognizers

import (
	"regexp"

	"github.com/karldane/go-presidio/presidio"
)

var (
	ipv4Regex     = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\b`)
	ipv4CidrRegex = regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])\.){3}(?:25[0-5]|2[0-4][0-9]|1[0-9]{2}|[1-9]?[0-9])/(?:[0-9]|[1-2][0-9]|3[0-2])\b`)
)

type IPv4Recognizer struct{}

func NewIPv4Recognizer() *IPv4Recognizer {
	return &IPv4Recognizer{}
}

func (r *IPv4Recognizer) Name() string { return "IPv4Recognizer" }
func (r *IPv4Recognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityIPv4}
}

func (r *IPv4Recognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult
	matches := ipv4CidrRegex.FindAllStringSubmatchIndex(value, -1)
	if len(matches) == 0 {
		matches = ipv4Regex.FindAllStringSubmatchIndex(value, -1)
	}
	for _, match := range matches {
		results = append(results, presidio.RecognizerResult{
			EntityType: presidio.EntityIPv4,
			Start:      match[0],
			End:        match[1],
			Score:      0.85,
			Recognizer: r.Name(),
		})
	}
	return results
}
