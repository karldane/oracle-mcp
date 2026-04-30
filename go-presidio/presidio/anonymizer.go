package presidio

import "os"
import "strings"

type AnonymizerEngine struct {
	operators       map[EntityType]Operator
	defaultOperator Operator
	hmacKey         []byte
}

type AnonymizerConfig struct {
	Operators       map[EntityType]Operator
	DefaultOperator Operator
	HMACKey         []byte
}

func NewAnonymizerEngine(cfg AnonymizerConfig) *AnonymizerEngine {
	a := &AnonymizerEngine{
		operators: make(map[EntityType]Operator),
	}
	if cfg.DefaultOperator != nil {
		a.defaultOperator = cfg.DefaultOperator
	} else {
		a.defaultOperator = &RedactOperator{}
	}
	for entity, op := range cfg.Operators {
		a.operators[entity] = op
	}
	if len(cfg.HMACKey) > 0 {
		a.hmacKey = cfg.HMACKey
	} else if key := os.Getenv("PRESIDIO_HMAC_KEY"); key != "" {
		a.hmacKey = []byte(key)
	}
	return a
}

func (a *AnonymizerEngine) AnonymiseText(value string, results []RecognizerResult) (string, PIIReport) {
	if len(results) == 0 {
		return value, PIIReport{Detected: false, Treatment: TreatmentNone}
	}

	report := PIIReport{
		Detected: true,
		Entities: results,
	}

	truncated := false
	truncateAt := 0
	originalLength := len(value)

	if originalLength > 512 {
		truncated = true
		truncateAt = 512
		value = value[:512]
		var truncatedResults []RecognizerResult
		for _, r := range results {
			if r.End <= 512 {
				truncatedResults = append(truncatedResults, r)
			}
		}
		results = truncatedResults
	}

	var sb strings.Builder
	sb.Grow(len(value) + 64)
	lastEnd := 0

	for _, r := range results {
		if r.Start < lastEnd {
			continue
		}
		if r.Start > len(value) {
			continue
		}
		sb.WriteString(value[lastEnd:r.Start])

		end := r.End
		if end > len(value) {
			end = len(value)
		}

		entity := r.EntityType
		op := a.operators[entity]
		if op == nil {
			op = a.defaultOperator
		}
		replacement := op.Anonymise(value, r.Start, end, entity)
		sb.WriteString(replacement)

		lastEnd = end
	}

	if lastEnd < len(value) {
		sb.WriteString(value[lastEnd:])
	}

	result := sb.String()

	if truncated {
		hasPII := len(report.Entities) > 0
		if hasPII {
			report.Treatment = TreatmentTruncatedRedacted
			report.TreatmentReason = "content truncated and redacted"
			result = SentinelContentTruncatedRedacted
		} else {
			report.Treatment = TreatmentTruncated
			report.TreatmentReason = "content truncated"
			result = SentinelContentTruncatedAt(512)
		}
		report.TruncatedAt = truncateAt
		report.OriginalLength = originalLength
	} else {
		report.Treatment = TreatmentRedacted
		report.TreatmentReason = "anonymised"
	}

	return result, report
}

func (a *AnonymizerEngine) AnonymiseValue(value string, entity EntityType) (string, PIIReport) {
	op := a.operators[entity]
	if op == nil {
		op = a.defaultOperator
	}

	replacement := op.Anonymise(value, 0, len(value), entity)
	report := PIIReport{
		Detected:        true,
		Treatment:       TreatmentRedacted,
		TreatmentReason: "anonymised",
		Entities: []RecognizerResult{
			{
				EntityType: entity,
				Start:      0,
				End:        len(value),
				Score:      1.0,
				Recognizer: "direct",
			},
		},
	}

	return replacement, report
}
