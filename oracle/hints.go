package oracle

import (
	"context"
	"strings"

	"github.com/karldane/mcp-framework/framework"
)

// piiEntityTypes maps PII column names to their entity types for the PII pipeline
var piiEntityTypes = map[string]string{
	"EMAIL":           "EMAIL_ADDRESS",
	"EMAIL_ADDR":      "EMAIL_ADDRESS",
	"EMAIL_ADDRESS":   "EMAIL_ADDRESS",
	"FIRSTNAME":       "PERSON",
	"FIRST_NAME":      "PERSON",
	"LASTNAME":        "PERSON",
	"LAST_NAME":       "PERSON",
	"SURNAME":         "PERSON",
	"FORENAME":        "PERSON",
	"GIVENNAME":       "PERSON",
	"GIVEN_NAME":      "PERSON",
	"FULLNAME":        "PERSON",
	"FULL_NAME":       "PERSON",
	"MIDDLENAME":      "PERSON",
	"MIDDLE_NAME":     "PERSON",
	"PHONE":           "PHONE_NUMBER",
	"PHONE_NO":        "PHONE_NUMBER",
	"PHONE_NUM":       "PHONE_NUMBER",
	"PHONE_NUMBER":    "PHONE_NUMBER",
	"MOBILE":          "PHONE_NUMBER",
	"MOBILE_NO":       "PHONE_NUMBER",
	"MOBILE_NUM":      "PHONE_NUMBER",
	"MOBILE_NUMBER":   "PHONE_NUMBER",
	"FAX":             "PHONE_NUMBER",
	"FAX_NO":          "PHONE_NUMBER",
	"FAX_NUM":         "PHONE_NUMBER",
	"FAX_NUMBER":      "PHONE_NUMBER",
	"POSTCODE":        "UK_POSTCODE",
	"POST_CODE":       "UK_POSTCODE",
	"ZIP":             "US_POSTCODE",
	"ZIP_CODE":        "US_POSTCODE",
	"DOB":             "DATE_OF_BIRTH",
	"DATE_OF_BIRTH":   "DATE_OF_BIRTH",
	"NI":              "UK_NINO",
	"NI_NO":           "UK_NINO",
	"NI_NUMBER":       "UK_NINO",
	"SSN":             "US_SSN",
	"PASSPORT":        "PASSPORT_NUMBER",
	"PASSPORT_NO":     "PASSPORT_NUMBER",
	"PASSPORT_NUMBER": "PASSPORT_NUMBER",
}

func getEntityType(colName string) string {
	// Check for Oracle-prefixed columns (e.g., CONT_FIRSTNAME)
	upperName := strings.ToUpper(colName)
	for suffix, entity := range piiEntityTypes {
		if strings.HasSuffix(upperName, "_"+suffix) || upperName == suffix {
			return entity
		}
	}
	return ""
}

func BuildColumnHints(columns []ColumnInfo) map[string]framework.ColumnHint {
	hints := make(map[string]framework.ColumnHint, len(columns))
	for _, col := range columns {
		scanPolicy := framework.ScanPolicy(col.ScanPolicy)
		// If column is detected as PII via name heuristic, use name-only scan
		if IsPIIColumn(col.Name) {
			scanPolicy = framework.ScanPolicyNameOnly
		}
		hints[col.Name] = framework.ColumnHint{
			ScanPolicy: scanPolicy,
			MaxLength:  col.MaxScanLength,
		}
	}
	return hints
}

func buildHintsFromQuery(ctx context.Context, result *QueryResult, sql string, executor QueryExecutor) map[string]framework.ColumnHint {
	// Build hints directly from query result column names
	// This is more robust than relying on table schema cache
	if len(result.Columns) == 0 {
		// Fallback to table schema if no columns in result
		tableName := extractTableName(sql)
		if tableName == "" {
			return nil
		}
		tableInfo, err := executor.GetTableInfo(ctx, tableName)
		if err != nil || tableInfo == nil || len(tableInfo.Columns) == 0 {
			return nil
		}
		return BuildColumnHints(tableInfo.Columns)
	}

	// Build hints from result columns directly
	hints := make(map[string]framework.ColumnHint, len(result.Columns))
	for _, colName := range result.Columns {
		scanPolicy := framework.ScanPolicyDefault
		entityType := ""
		if IsPIIColumn(colName) {
			scanPolicy = framework.ScanPolicyNameOnly
			entityType = getEntityType(colName)
		}
		hints[colName] = framework.ColumnHint{
			ScanPolicy: scanPolicy,
			MaxLength:  0,
			EntityType: entityType,
		}
	}
	return hints
}

func extractTableName(sql string) string {
	// Simple regex to extract table name from SELECT ... FROM table_name
	sql = strings.TrimSpace(strings.ToUpper(sql))

	// Match FROM followed by table name (potentially schema.table)
	// In real implementation, this would be more sophisticated
	if strings.HasPrefix(sql, "SELECT") {
		// Very basic extraction - in production this would be a proper SQL parser
		fromIndex := strings.Index(sql, "FROM")
		if fromIndex != -1 {
			afterFrom := strings.TrimSpace(sql[fromIndex+4:])
			// Get first word after FROM (table name)
			parts := strings.Fields(afterFrom)
			if len(parts) > 0 {
				tableName := parts[0]
				// Remove schema prefix if present (schema.table)
				if dotIndex := strings.Index(tableName, "."); dotIndex != -1 {
					tableName = tableName[dotIndex+1:]
				}
				return tableName
			}
		}
	}
	return ""
}

func buildHintsFromWriteQuery(ctx context.Context, sql string, executor QueryExecutor) map[string]framework.ColumnHint {
	tableName := extractWriteTableName(sql)
	if tableName == "" {
		return nil
	}

	tableInfo, err := executor.GetTableInfo(ctx, tableName)
	if err != nil || tableInfo == nil || len(tableInfo.Columns) == 0 {
		return nil
	}

	return BuildColumnHints(tableInfo.Columns)
}

func extractWriteTableName(sql string) string {
	sql = strings.TrimSpace(strings.ToUpper(sql))

	if strings.HasPrefix(sql, "INSERT") {
		// INSERT INTO table_name
		parts := strings.Fields(sql)
		if len(parts) >= 3 {
			tableName := parts[2]
			// Remove schema prefix if present
			if dotIndex := strings.Index(tableName, "."); dotIndex != -1 {
				tableName = tableName[dotIndex+1:]
			}
			return tableName
		}
	} else if strings.HasPrefix(sql, "UPDATE") {
		// UPDATE table_name SET ...
		parts := strings.Fields(sql)
		if len(parts) >= 2 {
			tableName := parts[1]
			// Remove schema prefix if present
			if dotIndex := strings.Index(tableName, "."); dotIndex != -1 {
				tableName = tableName[dotIndex+1:]
			}
			return tableName
		}
	} else if strings.HasPrefix(sql, "DELETE") {
		// DELETE FROM table_name
		parts := strings.Fields(sql)
		if len(parts) >= 3 && parts[1] == "FROM" {
			tableName := parts[2]
			// Remove schema prefix if present
			if dotIndex := strings.Index(tableName, "."); dotIndex != -1 {
				tableName = tableName[dotIndex+1:]
			}
			return tableName
		}
	}

	return ""
}
