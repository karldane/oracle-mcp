# SPEC: oracle-mcp Column Name–Based PII Detection

**Status:** Active  
**Repo:** oracle-mcp  
**Files touched:** `scanpolicy.go`, `database.go`, `scanpolicy_test.go` (additions)  
**Companion to:** SPEC_ORACLE_MCP_PII_STAGE2_final.md

---

## Problem

First names and surnames are not being redacted.  
Example: `CONT_FIRSTNAME = "Stevie"`, `CONT_SURNAME = "Bowditch"` pass through unredacted,  
while `CONT_EMAIL` and `CONT_PHONE_NO` are correctly treated.

### Root cause

`scanPolicyForColumn` in `scanpolicy.go` accepts only `dataType string`. It has no
knowledge of the column name. A `CONT_SURNAME VARCHAR2(50)` column is indistinguishable
from `CONT_NOTES VARCHAR2(50)` — both return `ScanPolicyFull`.

With `ScanPolicyFull`, the PII pipeline runs the `PersonRecognizer` against the raw
cell value. A single word like `"Bowditch"` or `"Stevie"` produces a confidence score
below the 0.5 threshold without surrounding prose context. The recognizer does not fire.

Emails and phone numbers work because their recognizers use fixed patterns (regex),
which match regardless of context or confidence — a valid email format is unambiguous.

`ScanPolicyNameOnly` is the correct policy for these columns. It tells the
`StructuredAnalyzer` to derive the entity type from the column name itself (treating
it as a strong signal), bypassing value-based confidence scoring entirely.

### Call site (confirmed in `database.go`, `loadTableDetails`):

```go
col.ScanPolicy, col.MaxScanLength = scanPolicyForColumn(col.DataType)
```

`col.Name` is in scope at this point but is not passed.

### Note on `hints.go`

The summary of recent fixes mentioned "build hints directly from query result columns."
The version of `hints.go` in this repo still uses the schema cache path correctly:

```go
func buildHintsFromQuery(...) map[string]presidio.ColumnHint {
    tableInfo, err := executor.GetTableInfo(ctx, tableName)
    ...
    return BuildColumnHints(tableInfo.Columns)
}
```

**If this was changed**, revert it. Query result columns carry no ScanPolicy — only the
schema cache (populated by `loadTableDetails`) does. If the schema cache path is broken,
the column name fix below will have no effect.

---

## Fix 1 — `scanpolicy.go`: accept column name, detect name columns

Change the function signature to accept both column name and data type. Add an
`isNameColumn` guard that runs before the data type switch.

**Full replacement for `scanpolicy.go`:**

```go
package oracle

import "strings"

// scanPolicyForColumn returns the ScanPolicy and MaxScanLength for a column.
// Column name is checked first; data type is the fallback.
func scanPolicyForColumn(columnName, dataType string) (int, int) {
    if isNameColumn(columnName) {
        return ScanPolicyNameOnly, 0
    }
    if isAddressColumn(columnName) {
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

// isNameColumn returns true if the column name indicates it holds a person name.
func isNameColumn(name string) bool {
    n := strings.ToUpper(name)

    // Exact matches
    exactNames := []string{"NAME", "FULLNAME", "FULL_NAME"}
    for _, exact := range exactNames {
        if n == exact {
            return true
        }
    }

    // Suffix matches — covers CONT_FIRSTNAME, EMP_SURNAME, MIDDLE_NAME etc.
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

    // Generic _NAME suffix — but exclude technical column names
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

// isAddressColumn returns true if the column name indicates it holds an address component.
// These columns get ScanPolicyNameOnly so address values are treated as location data.
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

// --- existing type detection functions unchanged below ---

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
```

---

## Fix 2 — `database.go`: pass column name to `scanPolicyForColumn`

In `loadTableDetails`, find the existing call:

```go
col.ScanPolicy, col.MaxScanLength = scanPolicyForColumn(col.DataType)
```

Change to:

```go
col.ScanPolicy, col.MaxScanLength = scanPolicyForColumn(col.Name, col.DataType)
```

That is the only change to `database.go`.

---

## Required tests: `scanpolicy_test.go` additions

Add to the existing `scanpolicy_test.go` (or create if it does not exist).

```go
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
    // Technical _NAME columns must not be treated as person names
    excluded := []string{
        "FILE_NAME", "TABLE_NAME", "COLUMN_NAME", "INDEX_NAME",
        "OBJECT_NAME", "SCHEMA_NAME", "ROLE_NAME", "TYPE_NAME",
        "HOST_NAME", "DB_NAME", "APP_NAME", "EVENT_NAME",
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
    }
    for _, name := range cases {
        if !isAddressColumn(name) {
            t.Errorf("expected isAddressColumn(%q) = true", name)
        }
    }
}

func TestIsAddressColumnNegatives(t *testing.T) {
    notAddr := []string{"EMAIL", "PHONE", "NAME", "ID", "STATUS"}
    for _, name := range notAddr {
        if isAddressColumn(name) {
            t.Errorf("expected isAddressColumn(%q) = false", name)
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
```

---

## Expected result after fix

```
CONT_EMAIL          | CONT_FIRSTNAME | CONT_ID | CONT_PHONE_NO    | CONT_SURNAME
------------------------------------------------------------
<redacted: EMAIL>   | <redacted: PERSON> | 1006 | <redacted: PHONE> | <redacted: PERSON>
<redacted: EMAIL>   | <redacted: PERSON> | 1008 | <redacted: PHONE> | <redacted: PERSON>
<redacted: EMAIL>   | <redacted: PERSON> | 1012 | <redacted: PHONE> | <redacted: PERSON>

[PII: pii detected in columns: CONT_EMAIL, CONT_FIRSTNAME, CONT_PHONE_NO, CONT_SURNAME]
```

`CONT_ID` and `CONT_FIRSTNAME` values like "Stevie" are a person name by column declaration,
not by value pattern. `ScanPolicyNameOnly` enforces this unconditionally.

---

## Important: schema cache must be rebuilt after this fix

`scanPolicyForColumn` is called in `loadTableDetails`, which populates the schema cache.
The cache is persisted to disk. If there is an existing cache for the CONTACTS table,
it will have the old `ScanPolicy: 0` (ScanPolicyDefault) values for name columns.

After deploying this fix, force a cache rebuild before testing:
- Delete the `.cache/` directory, OR
- Call `oracle_rebuild_cache` if that tool is available, OR
- Restart the server with an empty cache directory

If the cache is not cleared, the fix will appear to have no effect until the cache
naturally expires or is rebuilt.

---

## Files changed

| File | Change |
|---|---|
| `scanpolicy.go` | Full replacement — add `columnName` param, `isNameColumn`, `isAddressColumn` |
| `database.go` | One-line change — pass `col.Name` to `scanPolicyForColumn` |
| `scanpolicy_test.go` | Add tests for name/address detection and policy override |
