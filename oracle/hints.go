package oracle

import (
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
