package oracle

import "testing"

// --- isNameColumn ---

func TestIsNameColumnExact(t *testing.T) {
	for _, name := range []string{"NAME", "FULLNAME", "FULL_NAME"} {
		if !isNameColumn(name) {
			t.Errorf("expected isNameColumn(%q) = true", name)
		}
	}
}

func TestIsNameColumnSuffixes(t *testing.T) {
	cases := []string{
		"CONT_FIRSTNAME", "CONT_FIRST_NAME",
		"CONT_SURNAME", "CONT_LAST_NAME",
		"EMP_FORENAME", "P_FAMILYNAME",
		"CUSTOMER_MIDDLENAME", "PREFERRED_NAME",
		"CONTACT_FULLNAME",
	}
	for _, name := range cases {
		if !isNameColumn(name) {
			t.Errorf("expected isNameColumn(%q) = true", name)
		}
	}
}

func TestIsNameColumnExclusions(t *testing.T) {
	excluded := []string{
		"FILE_NAME", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME",
		"OBJECT_NAME", "SCHEMA_NAME", "ROLE_NAME", "TYPE_NAME",
		"HOST_NAME", "DB_NAME", "APP_NAME", "EVENT_NAME",
		"QUEUE_NAME", "TOPIC_NAME", "SERVER_NAME",
		"PROC_NAME", "FUNC_NAME", "CLASS_NAME",
	}
	for _, name := range excluded {
		if isNameColumn(name) {
			t.Errorf("expected isNameColumn(%q) = false (technical column)", name)
		}
	}
}

func TestIsNameColumnNegatives(t *testing.T) {
	notNames := []string{
		"CONT_EMAIL", "CONT_PHONE_NO", "CONT_ID",
		"STATUS", "CREATED_AT", "AMOUNT", "NOTES",
		"FILE_NAME", "TABLE_NAME",
	}
	for _, name := range notNames {
		if isNameColumn(name) {
			t.Errorf("expected isNameColumn(%q) = false", name)
		}
	}
}

// --- isAddressColumn ---

func TestIsAddressColumn(t *testing.T) {
	cases := []string{
		"ADDRESSLINE1", "ADDRESS_LINE2", "STREET", "CITY", "TOWN",
		"POSTCODE", "POST_CODE", "ZIPCODE", "ZIP_CODE",
		"COUNTY", "COUNTRY", "COUNTRY_CODE",
		"CONT_ADDRESS1", "CONT_POSTCODE", "EMP_STREET",
	}
	for _, name := range cases {
		if !isAddressColumn(name) {
			t.Errorf("expected isAddressColumn(%q) = true", name)
		}
	}
}

func TestIsAddressColumnNegatives(t *testing.T) {
	notAddr := []string{"EMAIL", "PHONE", "NAME", "ID", "STATUS", "CONT_EMAIL", "CONT_PHONE_NO"}
	for _, name := range notAddr {
		if isAddressColumn(name) {
			t.Errorf("expected isAddressColumn(%q) = false", name)
		}
	}
}

// --- isDOBColumn ---

func TestIsDOBColumn(t *testing.T) {
	cases := []string{
		"DOB", "DATE_OF_BIRTH", "BIRTHDATE", "BIRTH_DATE", "BIRTHDAY",
		"CONT_DOB", "EMP_DATE_OF_BIRTH", "CUSTOMER_BIRTH_DATE",
		"PATIENT_DOB", "DRIVER_BIRTHDATE",
	}
	for _, name := range cases {
		if !isDOBColumn(name) {
			t.Errorf("expected isDOBColumn(%q) = true", name)
		}
	}
}

func TestIsDOBColumnNegatives(t *testing.T) {
	notDOB := []string{"CONT_EMAIL", "CONT_PHONE_NO", "CREATED_DATE", "MODIFIED_DATE", "START_DATE", "END_DATE"}
	for _, name := range notDOB {
		if isDOBColumn(name) {
			t.Errorf("expected isDOBColumn(%q) = false", name)
		}
	}
}

// --- scanPolicyForColumn with name ---

func TestScanPolicyNameColumnOverridesDataType(t *testing.T) {
	// Even though VARCHAR2 would normally be ScanPolicyFull,
	// a name column must get ScanPolicyNameOnly
	policy, _ := scanPolicyForColumn("CONT_SURNAME", "VARCHAR2(100)")
	if policy != ScanPolicyNameOnly {
		t.Errorf("CONT_SURNAME: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyFirstNameColumn(t *testing.T) {
	policy, _ := scanPolicyForColumn("CONT_FIRSTNAME", "VARCHAR2(50)")
	if policy != ScanPolicyNameOnly {
		t.Errorf("CONT_FIRSTNAME: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyAddressColumn(t *testing.T) {
	policy, _ := scanPolicyForColumn("CONT_POSTCODE", "VARCHAR2(10)")
	if policy != ScanPolicyNameOnly {
		t.Errorf("CONT_POSTCODE: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyDOBColumn(t *testing.T) {
	policy, _ := scanPolicyForColumn("CONT_DOB", "VARCHAR2(20)")
	if policy != ScanPolicyNameOnly {
		t.Errorf("CONT_DOB: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyEmailColumnUnchanged(t *testing.T) {
	// Email columns are VARCHAR2 and not name columns — ScanPolicyFull
	policy, _ := scanPolicyForColumn("CONT_EMAIL", "VARCHAR2(255)")
	if policy != ScanPolicyFull {
		t.Errorf("CONT_EMAIL: expected ScanPolicyFull (%d), got %d", ScanPolicyFull, policy)
	}
}

func TestScanPolicyNumericIdColumnSafe(t *testing.T) {
	policy, _ := scanPolicyForColumn("CONT_ID", "NUMBER")
	if policy != ScanPolicySafe {
		t.Errorf("CONT_ID: expected ScanPolicySafe (%d), got %d", ScanPolicySafe, policy)
	}
}

func TestScanPolicyTableNameColumnNotPerson(t *testing.T) {
	// TABLE_NAME should not be treated as a person name column
	policy, _ := scanPolicyForColumn("TABLE_NAME", "VARCHAR2(128)")
	if policy == ScanPolicyNameOnly {
		t.Errorf("TABLE_NAME: should not be ScanPolicyNameOnly — is a technical column")
	}
}

func TestScanPolicyDateColumnNameOnly(t *testing.T) {
	// DATE type should get ScanPolicyNameOnly via data type logic
	policy, _ := scanPolicyForColumn("CREATED_DATE", "DATE")
	if policy != ScanPolicyNameOnly {
		t.Errorf("DATE column: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyTimestampColumnNameOnly(t *testing.T) {
	// TIMESTAMP type should get ScanPolicyNameOnly via data type logic
	policy, _ := scanPolicyForColumn("MODIFIED_TIMESTAMP", "TIMESTAMP")
	if policy != ScanPolicyNameOnly {
		t.Errorf("TIMESTAMP column: expected ScanPolicyNameOnly (%d), got %d", ScanPolicyNameOnly, policy)
	}
}

func TestScanPolicyPhoneColumnFull(t *testing.T) {
	// Phone columns (VARCHAR2) get ScanPolicyFull — pattern recognizer handles it
	policy, _ := scanPolicyForColumn("CONT_PHONE_NO", "VARCHAR2(50)")
	if policy != ScanPolicyFull {
		t.Errorf("CONT_PHONE_NO: expected ScanPolicyFull (%d), got %d", ScanPolicyFull, policy)
	}
}

func TestScanPolicyCLOBColumnTruncateThenScan(t *testing.T) {
	// CLOB type should get ScanPolicyTruncateThenScan
	policy, _ := scanPolicyForColumn("NOTES", "CLOB")
	if policy != ScanPolicyTruncateThenScan {
		t.Errorf("CLOB column: expected ScanPolicyTruncateThenScan (%d), got %d", ScanPolicyTruncateThenScan, policy)
	}
}
