package oracle

import "strings"

func scanPolicyForColumn(columnName, dataType string) (int, int) {
	if isNameColumn(columnName) || isAddressColumn(columnName) || isDOBColumn(columnName) {
		return ScanPolicyNameOnly, 0
	}

	t := strings.ToUpper(dataType)
	switch {
	case isNumericType(t):
		return ScanPolicySafe, 0
	case isBooleanType(t):
		return ScanPolicySafe, 0
	case isIntervalType(t):
		return ScanPolicySafe, 0
	case isBinaryType(t):
		return ScanPolicyStrip, 0
	case isLargeTextType(t):
		return ScanPolicyTruncateThenScan, DefaultMaxScanLength
	case isDateType(t):
		return ScanPolicyNameOnly, 0
	case isTextType(t):
		return ScanPolicyFull, 0
	case isStructuredType(t):
		return ScanPolicyTruncateThenScan, DefaultMaxScanLength
	default:
		return ScanPolicyFull, 0
	}
}

func isNameColumn(name string) bool {
	n := strings.ToUpper(name)

	exactNames := []string{"NAME", "FULLNAME", "FULL_NAME"}
	for _, exact := range exactNames {
		if n == exact {
			return true
		}
	}

	nameSuffixes := []string{
		"FIRSTNAME", "FIRST_NAME", "FORENAME", "FORE_NAME", "GIVENNAME", "GIVEN_NAME",
		"LASTNAME", "LAST_NAME", "SURNAME", "FAMILYNAME", "FAMILY_NAME",
		"MIDDLENAME", "MIDDLE_NAME", "MIDDLEINITIAL", "MIDDLE_INITIAL",
		"FULLNAME", "FULL_NAME",
		"PREFERREDNAME", "PREFERRED_NAME", "KNOWNNAME", "KNOWN_NAME",
		"INITIALS",
	}
	for _, suffix := range nameSuffixes {
		if strings.HasSuffix(n, suffix) {
			return true
		}
	}

	if strings.HasSuffix(n, "_NAME") {
		excluded := []string{
			"FILE_NAME", "FILENAME", "TABLE_NAME", "TABLENAME",
			"SCHEMA_NAME", "SCHEMANAME", "COLUMN_NAME", "COLUMNNAME",
			"INDEX_NAME", "INDEXNAME", "OBJECT_NAME", "OBJECTNAME",
			"ROLE_NAME", "ROLENAME", "TYPE_NAME", "TYPENAME",
			"PROC_NAME", "PROCNAME", "FUNC_NAME", "FUNCNAME",
			"EVENT_NAME", "EVENTNAME", "CLASS_NAME", "CLASSNAME",
			"HOST_NAME", "HOSTNAME", "SERVER_NAME", "SERVERNAME",
			"DB_NAME", "DBNAME", "APP_NAME", "APPNAME",
			"QUEUE_NAME", "QUEUENAME", "TOPIC_NAME", "TOPICNAME",
		}
		for _, exc := range excluded {
			if n == exc {
				return false
			}
		}
		return true
	}

	return false
}

func isAddressColumn(name string) bool {
	n := strings.ToUpper(name)

	addressSuffixes := []string{
		"ADDRESSLINE1", "ADDRESSLINE2", "ADDRESSLINE3",
		"ADDRESS_LINE1", "ADDRESS_LINE2", "ADDRESS_LINE3",
		"ADDRESS1", "ADDRESS2", "ADDRESS3",
		"STREETADDRESS", "STREET_ADDRESS", "STREET",
		"CITY", "TOWN", "COUNTY", "DISTRICT", "REGION", "STATE",
		"COUNTRY", "COUNTRYCODE", "COUNTRY_CODE",
		"POSTCODE", "POST_CODE", "POSTALCODE", "POSTAL_CODE",
		"ZIPCODE", "ZIP_CODE",
	}
	for _, suffix := range addressSuffixes {
		if strings.HasSuffix(n, suffix) || n == suffix {
			return true
		}
	}
	return false
}

func isDOBColumn(name string) bool {
	n := strings.ToUpper(name)

	dobPatterns := []string{
		"DOB", "DATEOFBIRTH", "DATE_OF_BIRTH",
		"BIRTHDATE", "BIRTH_DATE", "BIRTHDAY", "BIRTH_DAY",
	}
	for _, p := range dobPatterns {
		if n == p || strings.HasSuffix(n, "_"+p) || strings.HasSuffix(n, p) {
			return true
		}
	}
	return false
}

func isNumericType(t string) bool {
	return t == "NUMBER" || t == "INTEGER" || t == "INT" ||
		t == "SMALLINT" || t == "FLOAT" || t == "REAL" ||
		t == "BINARY_FLOAT" || t == "BINARY_DOUBLE" ||
		strings.HasPrefix(t, "NUMBER(") ||
		strings.HasPrefix(t, "FLOAT(")
}

func isBooleanType(t string) bool {
	return t == "BOOLEAN" || t == "RAW(1)" || t == "CHAR(1)"
}

func isIntervalType(t string) bool {
	return t == "INTERVAL" || t == "INTERVAL YEAR TO MONTH" ||
		t == "INTERVAL DAY TO SECOND"
}

func isBinaryType(t string) bool {
	return t == "RAW" || t == "BLOB" || t == "BFILE" ||
		t == "LONG RAW" || strings.HasPrefix(t, "RAW(")
}

func isLargeTextType(t string) bool {
	return t == "CLOB" || t == "NCLOB" || t == "LONG"
}

func isDateType(t string) bool {
	return t == "DATE" || t == "TIMESTAMP" ||
		strings.HasPrefix(t, "TIMESTAMP(") ||
		strings.HasPrefix(t, "TIMESTAMP WITH")
}

func isTextType(t string) bool {
	return strings.HasPrefix(t, "VARCHAR2") || strings.HasPrefix(t, "NVARCHAR2") ||
		strings.HasPrefix(t, "CHAR") || strings.HasPrefix(t, "NCHAR")
}

func isStructuredType(t string) bool {
	return t == "XMLTYPE" || t == "JSON"
}
