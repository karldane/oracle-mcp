package presidio

import (
	"testing"
)

func TestMatchColumnNameHint(t *testing.T) {
	tests := []struct {
		name           string
		columnName     string
		expectedEntity EntityType
	}{
		{"CONT prefix firstname", "CONT_FIRSTNAME", EntityPerson},
		{"CONT prefix surname", "CONT_SURNAME", EntityPerson},
		{"CONT prefix lastname", "CONT_LAST_NAME", EntityPerson},
		{"CONT prefix email", "CONT_EMAIL", EntityEmailAddress},
		{"CONT prefix phone", "CONT_PHONE", EntityPhoneNumber},
		{"CONT prefix mobile", "CONT_MOBILE", EntityPhoneNumber},
		{"CONT prefix postcode", "CONT_POSTCODE", EntityUkPostcode},
		{"CONT prefix dob", "CONT_DOB", EntityDateOfBirth},
		{"TBL prefix firstname", "TBL_FIRSTNAME", EntityPerson},
		{"no prefix firstname", "FIRSTNAME", EntityPerson},
		{"no prefix surname", "SURNAME", EntityPerson},
		{"no prefix email", "EMAIL", EntityEmailAddress},
		{"non-PII column", "CONT_NOTES", ""},
		{"non-PII column no prefix", "NOTES", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity, _ := MatchColumnNameHint(tt.columnName)
			if entity != tt.expectedEntity {
				t.Errorf("MatchColumnNameHint(%q) = %v, want %v", tt.columnName, entity, tt.expectedEntity)
			}
		})
	}
}

func TestStripTablePrefix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"CONT_FIRSTNAME", "FIRSTNAME"},
		{"CONT_SURNAME", "SURNAME"},
		{"TBL_USER", "USER"},
		{"TMP_DATA", "DATA"},
		{"STG_TABLE", "TABLE"},
		{"RAW_DATA", "DATA"},
		{"FIRSTNAME", "FIRSTNAME"},
		{"email", "EMAIL"},
		{"lower_case", "LOWER_CASE"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripTablePrefix(tt.input)
			if result != tt.expected {
				t.Errorf("stripTablePrefix(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
