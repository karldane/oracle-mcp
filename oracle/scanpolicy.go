package oracle

import (
	"strings"
)

func scanPolicyForColumn(dataType string) (int, int) {
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
