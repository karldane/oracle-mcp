package oracle

import "regexp"

// piiColumnPatterns defines regex patterns for detecting PII-relevant column names.
// Uses suffix-anchored patterns to match both bare names and Oracle-prefixed variants.
var piiColumnPatterns = []*regexp.Regexp{
	// Name variants
	regexp.MustCompile(`(?i)(^|_)FIRST_?NAME$`),
	regexp.MustCompile(`(?i)(^|_)LAST_?NAME$`),
	regexp.MustCompile(`(?i)(^|_)SURNAME$`),
	regexp.MustCompile(`(?i)(^|_)FORENAME$`),
	regexp.MustCompile(`(?i)(^|_)GIVEN_?NAME$`),
	regexp.MustCompile(`(?i)(^|_)FAMILY_?NAME$`),
	regexp.MustCompile(`(?i)(^|_)FULL_?NAME$`),
	regexp.MustCompile(`(?i)(^|_)MIDDLE_?NAME$`),
	// Contact details
	regexp.MustCompile(`(?i)(^|_)EMAIL(_ADDR(ESS)?)?$`),
	regexp.MustCompile(`(?i)(^|_)PHONE(_NO|_NUM|_NUMBER)?$`),
	regexp.MustCompile(`(?i)(^|_)MOBILE(_NO|_NUM|_NUMBER)?$`),
	regexp.MustCompile(`(?i)(^|_)FAX(_NO|_NUM|_NUMBER)?$`),
	// Address
	regexp.MustCompile(`(?i)(^|_)POST_?CODE$`),
	regexp.MustCompile(`(?i)(^|_)ZIP(_CODE)?$`),
	regexp.MustCompile(`(?i)(^|_)ADDR(ESS)?(_LINE[0-9])?$`),
	// Identity / government
	regexp.MustCompile(`(?i)(^|_)DOB$`),
	regexp.MustCompile(`(?i)(^|_)DATE_OF_BIRTH$`),
	regexp.MustCompile(`(?i)(^|_)NI_?(NO|NUMBER)?$`),
	regexp.MustCompile(`(?i)(^|_)SSN$`),
	regexp.MustCompile(`(?i)(^|_)PASSPORT(_NO|_NUMBER)?$`),
}

// IsPIIColumn returns true if the column name matches any known PII heuristic.
// This function is exported for use in unit tests and hint building.
func IsPIIColumn(colName string) bool {
	for _, p := range piiColumnPatterns {
		if p.MatchString(colName) {
			return true
		}
	}
	return false
}
