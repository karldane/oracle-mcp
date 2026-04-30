package recognizers

import (
	"regexp"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

var (
	ukPhoneRegex   = regexp.MustCompile(`(?:\+44\s?[0-9]{9,10}|07[0-9]{9}|0[1-9][0-9]{8,9}|(01|02|03|08)[0-9]{7,9})`)
	usPhoneRegex   = regexp.MustCompile(`(?:\(\d{3}\)[-.\s]?\d{3}[-.\s]?\d{4}|\d{3}[-.\s]?\d{3}[-.\s]?\d{4})`)
	e164PhoneRegex = regexp.MustCompile(`\+[1-9][0-9]{6,14}`)
)

type PhoneRecognizer struct{}

func NewPhoneRecognizer() *PhoneRecognizer {
	return &PhoneRecognizer{}
}

func (r *PhoneRecognizer) Name() string { return "PhoneRecognizer" }
func (r *PhoneRecognizer) SupportedEntities() []presidio.EntityType {
	return []presidio.EntityType{presidio.EntityPhoneNumber}
}

func (r *PhoneRecognizer) Analyse(value string) []presidio.RecognizerResult {
	seen := make(map[[2]int]bool)

	clean := strings.ReplaceAll(value, " ", "")
	if clean == "" {
		return nil
	}

	if strings.HasPrefix(clean, "+44") {
		matched := clean[:min(len(clean), 13)]
		return []presidio.RecognizerResult{{
			EntityType: presidio.EntityPhoneNumber,
			Start:      0,
			End:        len(matched),
			Score:      0.85,
			Recognizer: r.Name(),
		}}
	}

	if strings.HasPrefix(clean, "+1") && len(clean) >= 11 {
		matched := clean[:min(len(clean), 12)]
		return []presidio.RecognizerResult{{
			EntityType: presidio.EntityPhoneNumber,
			Start:      0,
			End:        len(matched),
			Score:      0.85,
			Recognizer: r.Name(),
		}}
	}

	if strings.HasPrefix(clean, "07") && len(clean) >= 10 {
		return []presidio.RecognizerResult{{
			EntityType: presidio.EntityPhoneNumber,
			Start:      0,
			End:        10,
			Score:      0.85,
			Recognizer: r.Name(),
		}}
	}

	ukLandline := []string{"01", "02", "03", "08"}
	for _, prefix := range ukLandline {
		if strings.HasPrefix(clean, prefix) && len(clean) >= 9 {
			length := 9
			if len(clean) > 9 && clean[9] >= '0' && clean[9] <= '9' {
				length = 10
			}
			return []presidio.RecognizerResult{{
				EntityType: presidio.EntityPhoneNumber,
				Start:      0,
				End:        length,
				Score:      0.85,
				Recognizer: r.Name(),
			}}
		}
	}

	var results []presidio.RecognizerResult
	for _, regex := range []*regexp.Regexp{ukPhoneRegex, usPhoneRegex, e164PhoneRegex} {
		matches := regex.FindAllStringSubmatchIndex(value, -1)
		for _, match := range matches {
			key := [2]int{match[0], match[1]}
			if !seen[key] {
				seen[key] = true
				results = append(results, presidio.RecognizerResult{
					EntityType: presidio.EntityPhoneNumber,
					Start:      match[0],
					End:        match[1],
					Score:      0.85,
					Recognizer: r.Name(),
				})
			}
		}
	}
	return results
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
