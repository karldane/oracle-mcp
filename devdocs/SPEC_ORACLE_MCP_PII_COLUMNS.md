# oracle-mcp: Oracle-Prefixed PII Column Detection

## Overview

During JOIN query testing, the PII column detection in `oracle-mcp` was found to
miss person-name columns that follow Oracle's standard table-prefix naming convention
(e.g. `CONT_FIRSTNAME`, `CONT_SURNAME`). The column name heuristics used exact/anchored
regex patterns designed for flat schema naming (e.g. `FIRSTNAME`, `SURNAME`), which do
not match the `<PREFIX>_<FIELD>` pattern ubiquitous in Oracle HR and CRM schemas.

`mcp-framework` requires no changes. The `EnforcerProfile` interface and `WithPII`
option remain as-is; this spec concerns oracle-mcp internals only.

---

## Root Cause

Oracle conventionally prefixes columns with a short table abbreviation:

| Prefix | Domain |
|--------|--------|
| `CONT_` | Contact |
| `EMP_`  | Employee |
| `CUST_` | Customer |
| `PER_`  | Person |
| `ADDR_` | Address |

The existing heuristics matched patterns such as `^FIRST_?NAME$`, which anchors both
ends of the string. A column named `CONT_FIRSTNAME` does not match because the `CONT_`
prefix precedes the token. Email columns were caught because Presidio's data scanner
recognises `user@domain.com` value patterns regardless of column name; name columns
contain free-text strings that have no detectable data pattern, making the column name
heuristic the only available signal.

---

## Changes Required

### 1. `oracle-mcp` — PII column heuristics (`pii.go` or equivalent)

Replace anchored full-match patterns with **suffix-anchored** patterns of the form
`(^|_)<TOKEN>$`. This matches both the bare form (`SURNAME`) and any Oracle-prefixed
form (`CONT_SURNAME`, `EMP_SURNAME`), while avoiding false positives on mid-string
occurrences (e.g. a hypothetical `PERFORMANCE_SURNAME_RANK` would not match).

#### Pattern changes

```go
// BEFORE
var piiColumnPatterns = []*regexp.Regexp{
    regexp.MustCompile(`(?i)^FIRST_?NAME$`),
    regexp.MustCompile(`(?i)^LAST_?NAME$`),
    regexp.MustCompile(`(?i)^SURNAME$`),
    regexp.MustCompile(`(?i)^EMAIL$`),
    regexp.MustCompile(`(?i)^PHONE(_NUMBER)?$`),
    regexp.MustCompile(`(?i)^POST_?CODE$`),
}

// AFTER
var piiColumnPatterns = []*regexp.Regexp{
    // Name variants
    regexp.MustCompile(`(?i)(^|_)FIRST_?NAME$`),
    regexp.MustCompile(`(?i)(^|_)LAST_?NAME$`),
    regexp.MustCompile(`(?i)(^|_)SURNAME$`),
    regexp.MustCompile(`(?i)(^|_)FORENAME$`),
    regexp.MustCompile(`(?i)(^|_)GIVEN_?NAME$`),
    regexp.MustCompile(`(?i)(^|_)FAMILY_?NAME$`),
    regexp.MustCompile(`(?i)(^|_)FULL_?NAME$`),
    regexp.MustCompile(`(?i)(^|_)MIDDLE_?NAME$`),
    // Contact details
    regexp.MustCompile(`(?i)(^|_)EMAIL(_ADDR(ESS)?)?$`),
    regexp.MustCompile(`(?i)(^|_)PHONE(_NO|_NUM|_NUMBER)?$`),
    regexp.MustCompile(`(?i)(^|_)MOBILE(_NO|_NUM|_NUMBER)?$`),
    regexp.MustCompile(`(?i)(^|_)FAX(_NO|_NUM|_NUMBER)?$`),
    // Address
    regexp.MustCompile(`(?i)(^|_)POST_?CODE$`),
    regexp.MustCompile(`(?i)(^|_)ZIP(_CODE)?$`),
    regexp.MustCompile(`(?i)(^|_)ADDR(ESS)?(_LINE[0-9])?$`),
    // Identity / government
    regexp.MustCompile(`(?i)(^|_)DOB$`),
    regexp.MustCompile(`(?i)(^|_)DATE_OF_BIRTH$`),
    regexp.MustCompile(`(?i)(^|_)NI_?(NO|NUMBER)?$`),
    regexp.MustCompile(`(?i)(^|_)SSN$`),
    regexp.MustCompile(`(?i)(^|_)PASSPORT(_NO|_NUMBER)?$`),
}
```

The `(^|_)` prefix anchor ensures:
- `SURNAME` → matches (bare form)
- `CONT_SURNAME` → matches (Oracle prefix)
- `EMP_SURNAME` → matches (Oracle prefix)
- `PERFORMANCE_SURNAME_RANK` → does **not** match (mid-string occurrence)

#### Helper function

Extract matching into a named helper so tests can exercise the pattern list directly:

```go
// IsPIIColumn returns true if the column name matches any known PII heuristic.
func IsPIIColumn(colName string) bool {
    for _, p := range piiColumnPatterns {
        if p.MatchString(colName) {
            return true
        }
    }
    return false
}
```

---

### 2. `oracle-mcp` — `EnforcerProfile` declarations on query tools

Any query tool that previously declared `WithPII(false)` must be corrected to
`WithPII(true)`. Any `SELECT` against an Oracle schema may return person data,
particularly via JOINs. The `EnforcerProfile` is static metadata transmitted at
`tools/list` time; it must reflect the worst-case behaviour of the tool class, not
the best case.

```go
// ExecuteQueryTool — correct declaration
func (t *ExecuteQueryTool) GetEnforcerProfile() framework.EnforcerProfile {
    return framework.NewEnforcerProfile(
        framework.WithRisk(framework.RiskMed),
        framework.WithImpact(framework.ImpactRead),
        framework.WithPII(true),        // ← must be true; JOINs can surface any table
        framework.WithIdempotent(true),
        framework.WithApprovalReq(false),
    )
}
```

---

## Tests Required

All tests live in `oracle-mcp`. No new tests are required in `mcp-framework`.

### Unit tests — `pii_test.go`

| Test name | Input | Expected |
|-----------|-------|----------|
| `TestBareColumnNames` | `SURNAME`, `FIRSTNAME`, `EMAIL` | `true` |
| `TestOraclePrefixedNames` | `CONT_FIRSTNAME`, `CONT_SURNAME`, `EMP_EMAIL` | `true` |
| `TestCommonOracleVariants` | `CUST_FORENAME`, `PER_LAST_NAME`, `ADDR_POSTCODE` | `true` |
| `TestNonPIIColumns` | `CONT_ID`, `CONT_STATUS`, `PERFORMANCE_RANK` | `false` |
| `TestMidStringNoMatch` | `PERFORMANCE_SURNAME_RANK`, `ACCOUNT_PHONE_HISTORY` | `false` |
| `TestCaseInsensitive` | `cont_firstname`, `Emp_Surname` | `true` |

### Integration test — JOIN query

Add a test that executes a mocked JOIN result containing columns `CONT_EMAIL`,
`CONT_FIRSTNAME`, `CONT_SURNAME` and asserts that all three are flagged as PII
in the post-processing step.

---

## Out of Scope

- `mcp-framework`: No interface or API changes. The `ToolHandler` and `EnforcerProfile`
  contracts are unchanged.
- `mcp-bridge`: No changes. The enforcer already evaluates `safety.piiExposure`
  from the self-reported profile; it does not inspect column names directly.
- Data-scan fallback (running Presidio's `PERSON` recogniser on column samples for
  unrecognised column names): desirable as a future enhancement but not part of this
  change.

---

## Acceptance Criteria

1. All unit tests in `pii_test.go` pass, including the new cases above.
2. The JOIN query test (`CONT_EMAIL`, `CONT_FIRSTNAME`, `CONT_SURNAME`) flags all
   three columns as PII.
3. No regressions on existing tests.
4. All query tools in `oracle-mcp` declare `WithPII(true)`.
5. The `IsPIIColumn` helper is exported and covered by the unit tests.
