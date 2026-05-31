# SPEC: oracle-mcp Structured Output Plumbing

**Status:** Active  
**Repo:** oracle-mcp  
**Files touched:** `hints.go`  
**Depends on:** SPEC_MCP_FRAMEWORK_STRUCTURED_OUTPUT.md (must be applied first —
`framework.ColumnHint.DataType` must exist before this change compiles)

---

## Goal

Propagate the Oracle column data type (`VARCHAR2(50)`, `NUMBER`, etc.) through the
hint map so mcp-framework can include it in `ColumnReport` and surface it in the
PII audit output and `structuredContent`.

This is a small targeted change. All the output formatting logic lives in the framework;
oracle-mcp only needs to pass the data type through.

---

## Fix — `hints.go`: populate `DataType` in `BuildColumnHints`

`col.DataType` is already available in `ColumnInfo`. It just needs to be assigned to
the new `framework.ColumnHint.DataType` field.

**Current:**
```go
func BuildColumnHints(columns []ColumnInfo) map[string]framework.ColumnHint {
    hints := make(map[string]framework.ColumnHint, len(columns))
    for _, col := range columns {
        hints[col.Name] = framework.ColumnHint{
            ScanPolicy: framework.ScanPolicy(col.ScanPolicy),
            MaxLength:  col.MaxScanLength,
        }
    }
    return hints
}
```

**After:**
```go
func BuildColumnHints(columns []ColumnInfo) map[string]framework.ColumnHint {
    hints := make(map[string]framework.ColumnHint, len(columns))
    for _, col := range columns {
        hints[col.Name] = framework.ColumnHint{
            ScanPolicy: framework.ScanPolicy(col.ScanPolicy),
            MaxLength:  col.MaxScanLength,
            DataType:   col.DataType,
        }
    }
    return hints
}
```

That is the only change to `hints.go`.

---

## Fallback path: `buildHintsFromColumnNames`

The fallback path (used when schema cache cannot resolve the table — JOINs, CTEs, etc.)
builds hints from result column names only, with no data type information available.
In that case `DataType` is left as the zero value (empty string `""`).

This is correct and expected. The framework handles empty `DataType` gracefully —
the PII audit report simply omits the data type column for those fields rather than
erroring.

No change needed to `buildHintsFromColumnNames`.

---

## Files changed

| File | Change |
|---|---|
| `hints.go` | Add `DataType: col.DataType` to `BuildColumnHints` — 1 line |

---

## Verification

After both specs are applied, run the CONTACTS query:

```sql
SELECT CONT_ID, CONT_FIRSTNAME, CONT_SURNAME, CONT_EMAIL, CONT_PHONE_NO
FROM CONTACTS WHERE ROWNUM <= 3
```

`content[1]` should show data types next to each column name:

```
CONT_EMAIL            VARCHAR2(255)     EMAIL_ADDRESS   3/3 rows  hash
CONT_FIRSTNAME        VARCHAR2(50)      PERSON          3/3 rows  hash  [name_only]
CONT_ID               NUMBER            —               not scanned [safe]
```

If data types show as blank, the schema cache path is not being used — confirm
`buildHintsFromQuery` is using `GetTableInfo` (schema cache), not result columns.
