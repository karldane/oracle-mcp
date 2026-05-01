package presidio

type AccessKind string

const (
	AccessAllow AccessKind = "allow"
	AccessDeny  AccessKind = "deny"
)

type ColumnPolicy struct {
	Access    AccessKind
	PIIForced bool
	Operator  Operator
}

type ScanPolicy int

const (
	ScanPolicySafe ScanPolicy = iota
	ScanPolicyNameOnly
	ScanPolicyFull
	ScanPolicyOpaque
	ScanPolicyStrip
	ScanPolicyTruncateThenScan
)

type ColumnHint struct {
	ScanPolicy ScanPolicy
	OracleType string
	MaxLength  int
}

type ColumnReport struct {
	ColumnName      string
	OracleType      string
	ScanPolicy      ScanPolicy
	PIIDetected     bool
	PIIEntities     []EntityType
	Confidence      float64
	DetectionSource string
	Treatment       TreatmentKind
	TreatmentReason string
	OriginalLength  int
	TruncatedAt     int
}

type StructuredAnalyzer struct {
	analyzer   *AnalyzerEngine
	anonymizer *AnonymizerEngine
	policies   map[string]ColumnPolicy
	sampleSize int
}

type StructuredConfig struct {
	Analyzer   *AnalyzerEngine
	Anonymizer *AnonymizerEngine
	Policies   map[string]ColumnPolicy
	SampleSize int
}

func NewStructuredAnalyzer(cfg StructuredConfig) *StructuredAnalyzer {
	s := &StructuredAnalyzer{
		analyzer:   cfg.Analyzer,
		anonymizer: cfg.Anonymizer,
		policies:   cfg.Policies,
		sampleSize: 20,
	}
	if cfg.SampleSize > 0 {
		s.sampleSize = cfg.SampleSize
	}
	if s.policies == nil {
		s.policies = make(map[string]ColumnPolicy)
	}
	return s
}

func (s *StructuredAnalyzer) ProcessRows(
	rows []map[string]interface{},
	hints map[string]ColumnHint,
) ([]map[string]interface{}, []ColumnReport) {
	if len(rows) == 0 {
		return rows, []ColumnReport{}
	}

	columnNames := make([]string, 0, len(rows[0]))
	for name := range rows[0] {
		columnNames = append(columnNames, name)
	}

	columnReports := make([]ColumnReport, 0, len(columnNames))
	for _, colName := range columnNames {
		report := s.processColumn(colName, rows, hints)
		columnReports = append(columnReports, report)
	}

	outputRows := make([]map[string]interface{}, len(rows))
	for i, row := range rows {
		outputRows[i] = make(map[string]interface{})
		for _, report := range columnReports {
			originalValue := row[report.ColumnName]
			if originalValue == nil {
				outputRows[i][report.ColumnName] = nil
				continue
			}
			strValue, ok := originalValue.(string)
			if !ok {
				outputRows[i][report.ColumnName] = originalValue
				continue
			}
			if report.Treatment == TreatmentNone {
				outputRows[i][report.ColumnName] = strValue
				continue
			}
			if len(report.PIIEntities) == 0 {
				outputRows[i][report.ColumnName] = strValue
				continue
			}
			anonymized, _ := s.anonymizer.AnonymiseValue(strValue, report.PIIEntities[0])
			outputRows[i][report.ColumnName] = anonymized
		}
	}

	return outputRows, columnReports
}

func (s *StructuredAnalyzer) processColumn(
	columnName string,
	rows []map[string]interface{},
	hints map[string]ColumnHint,
) ColumnReport {
	report := ColumnReport{
		ColumnName: columnName,
	}

	if policy, ok := s.policies[columnName]; ok {
		if policy.Access == AccessDeny {
			report.PIIDetected = true
			report.DetectionSource = "static_policy"
			report.Treatment = TreatmentStripped
			report.TreatmentReason = "access denied by policy"
			return report
		}
		if policy.PIIForced {
			report.PIIDetected = true
			report.DetectionSource = "static_policy"
			report.PIIEntities = []EntityType{EntityPerson}
			report.Confidence = 1.0
			report.Treatment = TreatmentRedacted
			report.TreatmentReason = "forced by policy"
			return report
		}
	}

	hint := hints[columnName]
	report.ScanPolicy = hint.ScanPolicy
	report.OracleType = hint.OracleType

	switch hint.ScanPolicy {
	case ScanPolicySafe:
		report.DetectionSource = "none"
		report.Treatment = TreatmentNone
		return report
	case ScanPolicyStrip:
		report.PIIDetected = true
		report.DetectionSource = "opaque_type"
		report.Treatment = TreatmentStripped
		report.TreatmentReason = "binary column"
		return report
	case ScanPolicyNameOnly:
		if entity, score := MatchColumnNameHint(columnName); entity != "" {
			report.PIIDetected = true
			report.PIIEntities = []EntityType{entity}
			report.Confidence = score
			report.DetectionSource = "name_heuristic"
			report.Treatment = TreatmentRedacted
			report.TreatmentReason = "pii from column name"
			return report
		}
		report.DetectionSource = "none"
		return report
	}

	sampleSize := s.sampleSize
	if sampleSize > len(rows) {
		sampleSize = len(rows)
	}
	values := make([]string, 0, sampleSize)
	for i := 0; i < sampleSize; i++ {
		if v := rows[i][columnName]; v != nil {
			if str, ok := v.(string); ok {
				values = append(values, str)
			}
		}
	}

	results := s.analyzer.AnalyseColumn(columnName, values)

	if entity, score := MatchColumnNameHint(columnName); entity != "" {
		report.PIIDetected = true
		report.PIIEntities = []EntityType{entity}
		report.Confidence = score
		report.DetectionSource = "name_heuristic"
		report.Treatment = TreatmentRedacted
		report.TreatmentReason = "pii from column name"
		if len(results) > 0 && results[0].Score > score {
			report.Confidence = results[0].Score
			report.PIIEntities = []EntityType{results[0].EntityType}
			report.DetectionSource = "value_scan"
		}
		return report
	}

	if len(results) > 0 {
		report.PIIDetected = true
		report.Confidence = results[0].Score
		for _, r := range results {
			report.PIIEntities = append(report.PIIEntities, r.EntityType)
		}
		report.DetectionSource = "value_scan"
		report.Treatment = TreatmentRedacted
		report.TreatmentReason = "pii detected in values"
	} else {
		report.DetectionSource = "none"
		report.Treatment = TreatmentNone
	}

	return report
}
