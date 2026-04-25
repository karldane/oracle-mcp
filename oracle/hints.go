package oracle

import (
	"context"
	"regexp"
	"strings"

	"github.com/karldane/go-presidio/presidio"
)

func BuildColumnHints(columns []ColumnInfo) map[string]presidio.ColumnHint {
	hints := make(map[string]presidio.ColumnHint, len(columns))
	for _, col := range columns {
		hints[col.Name] = presidio.ColumnHint{
			ScanPolicy: presidio.ScanPolicy(col.ScanPolicy),
			MaxLength:  col.MaxScanLength,
		}
	}
	return hints
}

func buildHintsFromQuery(ctx context.Context, result *QueryResult, sql string, executor QueryExecutor) map[string]presidio.ColumnHint {
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
	fromRegex := regexp.MustCompile(`FROM\s+([A-Z0-9_]+(?:\.[A-Z0-9_]+)?)`)
	matches := fromRegex.FindStringSubmatch(sql)
	if len(matches) > 1 {
		return strings.ToUpper(matches[1])
	}

	return ""
}

func buildHintsFromWriteQuery(ctx context.Context, sql string, executor QueryExecutor) map[string]presidio.ColumnHint {
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
		regex := regexp.MustCompile(`INSERT\s+INTO\s+([A-Z0-9_]+)`)
		matches := regex.FindStringSubmatch(sql)
		if len(matches) > 1 {
			return matches[1]
		}
	} else if strings.HasPrefix(sql, "UPDATE") {
		regex := regexp.MustCompile(`UPDATE\s+([A-Z0-9_]+)`)
		matches := regex.FindStringSubmatch(sql)
		if len(matches) > 1 {
			return matches[1]
		}
	} else if strings.HasPrefix(sql, "DELETE") {
		regex := regexp.MustCompile(`DELETE\s+FROM\s+([A-Z0-9_]+)`)
		matches := regex.FindStringSubmatch(sql)
		if len(matches) > 1 {
			return matches[1]
		}
	}

	return ""
}
