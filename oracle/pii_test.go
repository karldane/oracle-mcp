package oracle

import "testing"

func TestIsPIIColumn(t *testing.T) {
	tests := []struct {
		name     string
		colName  string
		expected bool
	}{
		// Bare column names
		{"Bare SURNAME", "SURNAME", true},
		{"Bare FIRSTNAME", "FIRSTNAME", true},
		{"Bare EMAIL", "EMAIL", true},
		{"Bare PHONE", "PHONE", true},
		{"Bare POSTCODE", "POSTCODE", true},

		// Oracle-prefixed names
		{"Oracle CONT_FIRSTNAME", "CONT_FIRSTNAME", true},
		{"Oracle CONT_SURNAME", "CONT_SURNAME", true},
		{"Oracle CONT_EMAIL", "CONT_EMAIL", true},
		{"Oracle EMP_EMAIL", "EMP_EMAIL", true},
		{"Oracle CUST_PHONE", "CUST_PHONE", true},
		{"Oracle PER_SSN", "PER_SSN", true},

		// Common Oracle variants
		{"Variant CUST_FORENAME", "CUST_FORENAME", true},
		{"Variant PER_LAST_NAME", "PER_LAST_NAME", true},
		{"Variant ADDR_POSTCODE", "ADDR_POSTCODE", true},
		{"Variant CONT_EMAIL_ADDRESS", "CONT_EMAIL_ADDRESS", true},
		{"Variant CONT_PHONE_NUMBER", "CONT_PHONE_NUMBER", true},

		// Non-PII columns
		{"Non-PII CONT_ID", "CONT_ID", false},
		{"Non-PII CONT_STATUS", "CONT_STATUS", false},
		{"Non-PII PERFORMANCE_RANK", "PERFORMANCE_RANK", false},
		{"Non-PII CREATED_DATE", "CREATED_DATE", false},
		{"Non-PII AMOUNT", "AMOUNT", false},

		// Mid-string occurrences should NOT match
		{"Mid-string PERFORMANCE_SURNAME_RANK", "PERFORMANCE_SURNAME_RANK", false},
		{"Mid-string ACCOUNT_PHONE_HISTORY", "ACCOUNT_PHONE_HISTORY", false},
		{"Mid-string USER_EMAIL_ID", "USER_EMAIL_ID", false},

		// Case insensitive
		{"Lowercase cont_firstname", "cont_firstname", true},
		{"Mixed case Emp_Surname", "Emp_Surname", true},
		{"Uppercase CONT_EMAIL", "CONT_EMAIL", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPIIColumn(tt.colName); got != tt.expected {
				t.Errorf("IsPIIColumn(%q) = %v, want %v", tt.colName, got, tt.expected)
			}
		})
	}
}
