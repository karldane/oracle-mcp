package recognizers

import (
	"regexp"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var (
	ipv6Regex    = regexp.MustCompile(`(?i)\b(?:[0-9a-f]{1,4}:){1,7}[0-9a-f]{1,4}(?:/[0-9]+)?\b`)
	ipv6Loopback = regexp.MustCompile(`(?i)::1\b`)
	ipv6Bare     = regexp.MustCompile(`(?i)^::$`)
)

type IPv6Recognizer struct{}

func NewIPv6Recognizer() *IPv6Recognizer {
	return &IPv6Recognizer{}
}

func (r *IPv6Recognizer) Name() string { return "IPv6Recognizer" }
func (r *IPv6Recognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityIPv6}
}

func (r *IPv6Recognizer) Analyse(value string) []presidio.RecognizerResult {
	var results []presidio.RecognizerResult

	if ipv6Loopback.MatchString(value) {
		loc := strings.Index(strings.ToLower(value), "::1")
		if loc >= 0 {
			results = append(results, presidio.RecognizerResult{
				EntityType: presidio.EntityIPv6,
				Start:      loc,
				End:        loc + 3,
				Score:      0.85,
				Recognizer: r.Name(),
			})
			return dedupeResults(results)
		}
	}

	matches := ipv6Regex.FindAllStringSubmatchIndex(value, -1)
	for _, match := range matches {
		matched := value[match[0]:match[1]]
		if isValidIPv6(matched) {
			results = append(results, presidio.RecognizerResult{
				EntityType: presidio.EntityIPv6,
				Start:      match[0],
				End:        match[1],
				Score:      0.85,
				Recognizer: r.Name(),
			})
		}
	}

	if len(results) == 0 {
		cidrMatch := findIPv6CIDR(value)
		if cidrMatch != nil {
			results = append(results, *cidrMatch)
		}
	}

	if len(results) == 0 && ipv6Bare.MatchString(value) {
		results = append(results, presidio.RecognizerResult{
			EntityType: presidio.EntityIPv6,
			Start:      0,
			End:        2,
			Score:      0.5,
			Recognizer: r.Name(),
		})
	}

	return dedupeResults(results)
}

func findIPv6CIDR(value string) *presidio.RecognizerResult {
	patterns := []string{"fe80::/10", "::/128"}
	for _, p := range patterns {
		idx := strings.Index(value, p)
		if idx >= 0 {
			return &presidio.RecognizerResult{
				EntityType: presidio.EntityIPv6,
				Start:      idx,
				End:        idx + len(p),
				Score:      0.85,
				Recognizer: "IPv6Recognizer",
			}
		}
	}
	lower := strings.ToLower(value)
	if strings.Contains(lower, "::/") {
		idx := strings.Index(lower, "::/")
		end := idx + 3
		for end < len(value) && value[end] >= '0' && value[end] <= '9' {
			end++
		}
		if end > idx+3 {
			return &presidio.RecognizerResult{
				EntityType: presidio.EntityIPv6,
				Start:      idx,
				End:        end,
				Score:      0.5,
				Recognizer: "IPv6Recognizer",
			}
		}
	}
	return nil
}

func isValidIPv6(s string) bool {
	lower := strings.ToLower(s)
	if strings.Contains(lower, "gggg") {
		return false
	}
	replaced := strings.ReplaceAll(lower, ":", "")
	replaced = strings.ReplaceAll(replaced, "/", "")
	replaced = strings.ReplaceAll(replaced, " ", "")
	for _, c := range replaced {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return len(replaced) >= 4
}

func dedupeResults(results []presidio.RecognizerResult) []presidio.RecognizerResult {
	seen := make(map[[2]int]bool)
	var uniq []presidio.RecognizerResult
	for _, r := range results {
		key := [2]int{r.Start, r.End}
		if !seen[key] {
			seen[key] = true
			uniq = append(uniq, r)
		}
	}
	return uniq
}
