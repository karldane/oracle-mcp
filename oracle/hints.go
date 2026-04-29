package oracle

import (
	"context"
	"strings"

	"github.com/karldane/mcp-framework/framework"
)

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
	tableName := extractTableName(sql)
	if tableName == "" {
		return nil
	}

	// Try to get table info from schema cache
	tableInfo, err := executor.GetTableInfo(ctx, tableName)
	if err != nil || tableInfo == nil || len(tableInfo.Columns) == 0 {
		return nil
	}

	return BuildColumnHints(tableInfo.Columns)
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
