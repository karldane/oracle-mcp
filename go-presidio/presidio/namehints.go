package presidio

import (
	"regexp"
	"strings"
)

var NameHintPatterns = map[EntityType][]*regexp.Regexp{
	EntityEmailAddress: {
		regexp.MustCompile(`(?i)^email$`),
		regexp.MustCompile(`(?i)^email_?address$`),
		regexp.MustCompile(`(?i)^e_?mail$`),
		regexp.MustCompile(`(?i)^contact_?email$`),
	},
	EntityPhoneNumber: {
		regexp.MustCompile(`(?i)^phone$`),
		regexp.MustCompile(`(?i)^telephone$`),
		regexp.MustCompile(`(?i)^mobile$`),
		regexp.MustCompile(`(?i)^cell_?phone$`),
		regexp.MustCompile(`(?i)^tel_?no$`),
		regexp.MustCompile(`(?i)^phone_?number$`),
	},
	EntityUkPostcode: {
		regexp.MustCompile(`(?i)^postcode$`),
		regexp.MustCompile(`(?i)^postal_?code$`),
		regexp.MustCompile(`(?i)^zip$`),
		regexp.MustCompile(`(?i)^zip_?code$`),
	},
	EntityUkNino: {
		regexp.MustCompile(`(?i)^nino$`),
		regexp.MustCompile(`(?i)^national_?insurance$`),
		regexp.MustCompile(`(?i)^ni_?number$`),
	},
	EntityUkNhsNumber: {
		regexp.MustCompile(`(?i)^nhs$`),
		regexp.MustCompile(`(?i)^nhs_?number$`),
		regexp.MustCompile(`(?i)^health_?number$`),
	},
	EntityCreditCard: {
		regexp.MustCompile(`(?i)^credit_?card$`),
		regexp.MustCompile(`(?i)^cc_?number$`),
		regexp.MustCompile(`(?i)^card_?number$`),
		regexp.MustCompile(`(?i)^cc$`),
	},
	EntityIban: {
		regexp.MustCompile(`(?i)^iban$`),
		regexp.MustCompile(`(?i)^bank_?account$`),
		regexp.MustCompile(`(?i)^account_?iban$`),
	},
	EntityIPv4: {
		regexp.MustCompile(`(?i)^ip_?address$`),
		regexp.MustCompile(`(?i)^ip$`),
		regexp.MustCompile(`(?i)^ip_?v4$`),
		regexp.MustCompile(`(?i)^host_?ip$`),
	},
	EntityIPv6: {
		regexp.MustCompile(`(?i)^ipv6$`),
		regexp.MustCompile(`(?i)^ip_?v6$`),
	},
	EntityDateOfBirth: {
		regexp.MustCompile(`(?i)^dob$`),
		regexp.MustCompile(`(?i)^date_?of_?birth$`),
		regexp.MustCompile(`(?i)^birth_?date$`),
		regexp.MustCompile(`(?i)^birthday$`),
	},
	EntityPerson: {
		regexp.MustCompile(`(?i)^name$`),
		regexp.MustCompile(`(?i)^first_?name$`),
		regexp.MustCompile(`(?i)^last_?name$`),
		regexp.MustCompile(`(?i)^full_?name$`),
		regexp.MustCompile(`(?i)^surname$`),
		regexp.MustCompile(`(?i)^forename$`),
		regexp.MustCompile(`(?i)^given_?name$`),
	},
	EntityUsSsn: {
		regexp.MustCompile(`(?i)^ssn$`),
		regexp.MustCompile(`(?i)^social_?security$`),
		regexp.MustCompile(`(?i)^ssn_?number$`),
	},
	EntityPassportNumber: {
		regexp.MustCompile(`(?i)^passport$`),
		regexp.MustCompile(`(?i)^passport_?number$`),
		regexp.MustCompile(`(?i)^passport_?no$`),
	},
}

func MatchColumnNameHint(columnName string) (EntityType, float64) {
	stripped := stripTablePrefix(columnName)
	for entity, patterns := range NameHintPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(stripped) {
				return entity, 0.85
			}
		}
	}
	return "", 0.0
}

func stripTablePrefix(name string) string {
	name = strings.ToUpper(name)
	prefixes := []string{"CONT_", "TBL_", "TMP_", "STG_", "RAW_"}
	for _, p := range prefixes {
		if strings.HasPrefix(name, p) {
			return name[len(p):]
		}
	}
	return name
}
