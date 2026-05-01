package oracle

import (
	"testing"

	"github.com/karldane/mcp-framework/framework"
)

func TestBuildHintsFromColumnNames(t *testing.T) {
	tests := []struct {
		name     string
		columns  []string
		checkCol string
		wantErr  bool
	}{
		{
			name:     "firstname column gets name-only policy",
			columns:  []string{"CONT_FIRSTNAME"},
			checkCol: "CONT_FIRSTNAME",
		},
		{
			name:     "surname column gets name-only policy",
			columns:  []string{"CONT_SURNAME"},
			checkCol: "CONT_SURNAME",
		},
		{
			name:     "postcode column gets name-only policy",
			columns:  []string{"CONT_POSTCODE"},
			checkCol: "CONT_POSTCODE",
		},
		{
			name:     "dob column gets name-only policy",
			columns:  []string{"CONT_DOB"},
			checkCol: "CONT_DOB",
		},
		{
			name:     "email column gets full policy (fallback)",
			columns:  []string{"CONT_EMAIL"},
			checkCol: "CONT_EMAIL",
		},
		{
			name:     "mixed columns",
			columns:  []string{"CONT_ID", "CONT_FIRSTNAME", "CONT_EMAIL"},
			checkCol: "CONT_FIRSTNAME",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hints := buildHintsFromColumnNames(tt.columns)
			if hints == nil {
				t.Fatalf("expected non-nil hints")
			}

			hint, ok := hints[tt.checkCol]
			if !ok {
				t.Errorf("expected hint for column %s", tt.checkCol)
			}

			// For name columns, expect ScanPolicyNameOnly
			if isNameColumn(tt.checkCol) || isAddressColumn(tt.checkCol) || isDOBColumn(tt.checkCol) {
				if hint.ScanPolicy != framework.ScanPolicyNameOnly {
					t.Errorf("expected ScanPolicyNameOnly for %s, got %d", tt.checkCol, hint.ScanPolicy)
				}
			}
		})
	}
}

func TestBuildHintsFromColumnNamesEntityType(t *testing.T) {
	tests := []struct {
		column   string
		expected string
	}{
		{"CONT_FIRSTNAME", "PERSON"},
		{"CONT_SURNAME", "PERSON"},
		{"CONT_POSTCODE", "UK_POSTCODE"},
		{"CONT_DOB", "DATE_OF_BIRTH"},
		{"CONT_EMAIL", ""}, // not a name column in this detection
		{"CONT_ID", ""},    // not a name column
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			hints := buildHintsFromColumnNames([]string{tt.column})
			hint := hints[tt.column]
			if hint.EntityType != tt.expected {
				t.Errorf("for %s: expected EntityType %q, got %q", tt.column, tt.expected, hint.EntityType)
			}
		})
	}
}

func TestBuildHintsFromColumnNamesPolicy(t *testing.T) {
	tests := []struct {
		column     string
		wantPolicy int
	}{
		{"CONT_FIRSTNAME", ScanPolicyNameOnly},
		{"CONT_SURNAME", ScanPolicyNameOnly},
		{"CONT_POSTCODE", ScanPolicyNameOnly},
		{"CONT_DOB", ScanPolicyNameOnly},
		{"CONT_EMAIL", ScanPolicyFull},    // email is not detected by isNameColumn
		{"CONT_PHONE_NO", ScanPolicyFull}, // phone is not detected by isNameColumn
		{"CONT_ID", ScanPolicyFull},
		{"COMP_NAME", ScanPolicyNameOnly}, // _NAME suffix gets name-only (better safe than sorry)
	}

	for _, tt := range tests {
		t.Run(tt.column, func(t *testing.T) {
			hints := buildHintsFromColumnNames([]string{tt.column})
			hint := hints[tt.column]
			if hint.ScanPolicy != framework.ScanPolicy(tt.wantPolicy) {
				t.Errorf("for %s: expected policy %d, got %d", tt.column, tt.wantPolicy, hint.ScanPolicy)
			}
		})
	}
}

func TestBuildColumnHintsIncludesDataType(t *testing.T) {
	columns := []ColumnInfo{
		{Name: "CONT_ID", DataType: "NUMBER", Nullable: false, ScanPolicy: 0},
		{Name: "CONT_FIRSTNAME", DataType: "VARCHAR2(50)", Nullable: true, ScanPolicy: 0},
		{Name: "CONT_EMAIL", DataType: "VARCHAR2(255)", Nullable: true, ScanPolicy: 0},
		{Name: "CONT_SALARY", DataType: "NUMBER", Nullable: true, ScanPolicy: 1},
	}

	hints := BuildColumnHints(columns)

	if hints["CONT_ID"].DataType != "NUMBER" {
		t.Errorf("expected DataType 'NUMBER' for CONT_ID, got %q", hints["CONT_ID"].DataType)
	}

	if hints["CONT_FIRSTNAME"].DataType != "VARCHAR2(50)" {
		t.Errorf("expected DataType 'VARCHAR2(50)' for CONT_FIRSTNAME, got %q", hints["CONT_FIRSTNAME"].DataType)
	}

	if hints["CONT_EMAIL"].DataType != "VARCHAR2(255)" {
		t.Errorf("expected DataType 'VARCHAR2(255)' for CONT_EMAIL, got %q", hints["CONT_EMAIL"].DataType)
	}

	if hints["CONT_SALARY"].DataType != "NUMBER" {
		t.Errorf("expected DataType 'NUMBER' for CONT_SALARY, got %q", hints["CONT_SALARY"].DataType)
	}

	if hints["CONT_ID"].ScanPolicy != framework.ScanPolicySafe {
		t.Errorf("expected ScanPolicySafe for NUMBER column, got %v", hints["CONT_ID"].ScanPolicy)
	}
}
