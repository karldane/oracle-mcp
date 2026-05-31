# oracle-mcp: PII Pipeline and Schema-Aware Filtering

**Status:** Proposed  
**Author:** Karl Dane  
**Depends on:** `go-presidio` library, `mcp-framework` ToolResult envelope  

---

## Overview

This spec defines the changes to oracle-mcp required to integrate PII detection and
anonymisation into database query results. oracle-mcp is uniquely positioned among
the MCP backends to provide the richest PII filtering, because it has direct access
to Oracle's type system via its existing schema cache.

The implementation operates across three cooperating layers:

1. **Schema cache extension** — columns are classified at cache build time into `ScanPolicy` tiers, eliminating per-query type-checking overhead.
2. **Static policy filter** — an optional `policy.yaml` file provides explicit allow/deny/treat rules per table and column, evaluated before dynamic scanning.
3. **Dynamic PII pipeline** — `go-presidio`'s `StructuredAnalyzer` operates on remaining columns, using schema-derived `ColumnHint`s to drive type-aware scanning.

---

## Schema Cache Extension

The existing `CachedColumn` struct is extended with two fields set once at
cache population time and never recomputed per query.

```go
// oracle/schema.go

type CachedColumn struct {
    Name         string
    OracleType   string
    Nullable     bool
    Length       int
    Precision    int
    Scale        int
    IsPrimaryKey bool
    IsForeignKey bool
    // New fields:
    ScanPolicy   gopresidio.ScanPolicy // assigned from OracleType at cache build time
    MaxScanLength int                  // for TruncateThenScan columns; 0 = use default
}
```

### ScanPolicy Assignment

```go
// oracle/scanpolicy.go

// scanPolicyForColumn derives the ScanPolicy for a cached column.
// This function is called once per column at cache population time.
func scanPolicyForColumn(col OracleColumnDef) gopresidio.ScanPolicy {
    t := strings.ToUpper(col.DataType)
    switch {
    case isNumericType(t):
        return gopresidio.ScanPolicySafe
    case isBooleanType(t):
        return gopresidio.ScanPolicySafe
    case isIntervalType(t):
        return gopresidio.ScanPolicySafe
    case isBinaryType(t):    // RAW, BLOB, BFILE, LONG RAW
        return gopresidio.ScanPolicyStrip
    case isLargeTextType(t): // CLOB, NCLOB
        return gopresidio.ScanPolicyTruncateThenScan
    case isDateType(t):      // DATE, TIMESTAMP, TIMESTAMP WITH TIME ZONE
        return gopresidio.ScanPolicyNameOnly
    case isTextType(t):      // VARCHAR2, NVARCHAR2, CHAR, NCHAR
        return gopresidio.ScanPolicyFull
    case isStructuredType(t): // XMLTYPE, JSON
        return gopresidio.ScanPolicyTruncateThenScan
    default:
        return gopresidio.ScanPolicyFull // unknown: assume worst case
    }
}

func isNumericType(t string) bool {
    return t == "NUMBER" || t == "INTEGER" || t == "INT" ||
        t == "SMALLINT" || t == "FLOAT" || t == "REAL" ||
        t == "BINARY_FLOAT" || t == "BINARY_DOUBLE" ||
        strings.HasPrefix(t, "NUMBER(") ||
        strings.HasPrefix(t, "FLOAT(")
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

### Truncation Limit

CLOB and XMLTYPE columns use `MaxScanLength` to cap the string before it enters
the PII scanner. The default is 512 characters, configurable via environment variable
`ORACLE_PII_CLOB_LIMIT`. The original length is preserved in `ColumnReport.OriginalLength`
for decoration purposes.

---

## Static Policy Document

An optional `policy.yaml` file provides deterministic allow/deny/treat rules that
are evaluated before dynamic scanning. Static rules take full precedence — a column
denied by policy is removed before any scanning occurs.

### Format

```yaml
# oracle-mcp access policy
# version: 1
# default_mode: whitelist | blacklist
# When whitelist: only tables/columns explicitly listed are accessible.
# When blacklist: all tables/columns are accessible except those listed as deny.

version: 1
default_mode: whitelist
pii_hmac_key_env: ORACLE_PII_HMAC_KEY  # env var holding the HMAC secret

tables:
  CUSTOMERS:
    access: allow
    columns:
      CUSTOMER_ID:
        access: allow
        pii: false
      FULL_NAME:
        access: allow
        pii: true
        pii_treatment: pseudonymise   # redact | hash | mask | pseudonymise
      EMAIL_ADDRESS:
        access: allow
        pii: true
        pii_treatment: hash           # HMAC-SHA256
      PHONE_NUMBER:
        access: deny                  # column removed entirely
      CREDIT_SCORE:
        access: deny
      DATE_OF_BIRTH:
        access: allow
        pii: true
        pii_treatment: redact

  ORDERS:
    access: allow
    columns:
      ORDER_ID:       { access: allow, pii: false }
      CUSTOMER_ID:    { access: allow, pii: false }
      ORDER_TOTAL:    { access: allow, pii: false }
      CARD_LAST4:     { access: deny }

  AUDIT_LOG:
    access: deny    # entire table blocked
```

### Policy Loading

```go
// oracle/policy.go

type AccessPolicy struct {
    Version     int                       `yaml:"version"`
    DefaultMode string                    `yaml:"default_mode"` // "whitelist" | "blacklist"
    HMACKeyEnv  string                    `yaml:"pii_hmac_key_env"`
    Tables      map[string]TablePolicy    `yaml:"tables"`
}

type TablePolicy struct {
    Access  string                    `yaml:"access"`   // "allow" | "deny"
    Columns map[string]ColumnPolicy   `yaml:"columns"`
}

type ColumnPolicy struct {
    Access       string `yaml:"access"`        // "allow" | "deny"
    PII          bool   `yaml:"pii"`
    PIITreatment string `yaml:"pii_treatment"` // "redact" | "hash" | "mask" | "pseudonymise"
}

// LoadPolicy reads and validates policy.yaml from the configured path.
// Returns nil (not an error) if no policy file is present — dynamic scanning only.
func LoadPolicy(path string) (*AccessPolicy, error)

// ApplyPolicy filters a row set according to the static policy.
// Returns the filtered rows and a list of ColumnReports for denied/treated columns.
// Columns not mentioned in a whitelist policy are removed.
// Columns not mentioned in a blacklist policy pass through.
func ApplyPolicy(
    rows []map[string]interface{},
    columns []CachedColumn,
    policy *AccessPolicy,
    tableName string,
    hmacKey []byte,
) ([]map[string]interface{}, []gopresidio.ColumnReport)
```

The policy file path is configured via the `ORACLE_POLICY_FILE` environment variable.
Default: `./policy.yaml`. If the file does not exist, oracle-mcp starts normally with
dynamic-only scanning. A missing file is not an error; a malformed file is a fatal
startup error.

---

## Query Result Pipeline

All query results from `oracle_execute_read` and `oracle_execute_write` pass through
the following pipeline before being placed into `ToolResult.Data`.

```
Raw Oracle rows ([]map[string]interface{})
        │
        ▼
┌────────────────────┐
│  Static Policy     │  ApplyPolicy() — deny columns, explicit PII treatment
│  Filter            │  Skipped if no policy.yaml
└────────────────────┘
        │ filtered rows + static ColumnReports
        ▼
┌────────────────────┐
│  ColumnHint        │  Build map[string]ColumnHint from schema cache
│  Builder           │  ScanPolicy + OracleType + MaxScanLength per column
└────────────────────┘
        │
        ▼
┌────────────────────┐
│  StructuredAnalyzer│  go-presidio processes remaining columns
│  (go-presidio)     │  Type-tier pre-filter, name heuristics, value scan
└────────────────────┘
        │ processed rows + dynamic ColumnReports
        ▼
┌────────────────────┐
│  Report Merge      │  Combine static + dynamic ColumnReports
└────────────────────┘
        │
        ▼
ToolResult{
    Data:        processed rows,
    ColumnHints: hints,          // passed to framework for ResultMeta
    Meta:        (set by framework dispatch pipeline)
}
```

---

## ToolResult Integration

The `oracle_execute_read` and `oracle_execute_write` tools are upgraded to Tier 3
(column hints + structured data). All schema introspection tools (`oracle_list_tables`,
`oracle_describe_table`, etc.) return metadata rather than row data and are upgraded
to Tier 2 (structured data, no column hints required).

```go
// oracle/tools/execute_read.go

func (t *ExecuteReadTool) Handle(
    ctx context.Context,
    args map[string]interface{},
) (framework.ToolResult, error) {

    rows, tableName, err := t.executor.Query(ctx, args)
    if err != nil {
        return framework.ToolResult{}, err
    }

    // Fetch schema from cache for the queried table
    columns, _ := t.cache.GetColumns(tableName)

    // Build ColumnHints from schema cache
    hints := buildColumnHints(columns)

    // Apply static policy if configured
    var staticReports []gopresidio.ColumnReport
    if t.policy != nil {
        rows, staticReports = applyPolicy(rows, columns, t.policy, tableName)
    }

    return framework.ToolResult{
        Data:        rows,
        ColumnHints: hints,
        // staticReports are merged into ResultMeta by the PII pipeline
        // via a mechanism in ColumnHints (see below)
    }, nil
}
```

### ColumnHint Builder

```go
// oracle/hints.go

func buildColumnHints(columns []CachedColumn) map[string]gopresidio.ColumnHint {
    hints := make(map[string]gopresidio.ColumnHint, len(columns))
    for _, col := range columns {
        hints[col.Name] = gopresidio.ColumnHint{
            ScanPolicy: col.ScanPolicy,
            OracleType: col.OracleType,
            MaxLength:  col.MaxScanLength,
        }
    }
    return hints
}
```

---

## Environment Variables

| Variable | Description | Default |
|---|---|---|
| `ORACLE_CONNECTION_STRING` | Oracle connection string (required) | — |
| `ORACLE_READ_ONLY` | Enable read-only mode | `true` |
| `CACHE_DIR` | Schema cache directory | `.cache` |
| `ORACLE_POLICY_FILE` | Path to static policy YAML | `./policy.yaml` |
| `ORACLE_PII_HMAC_KEY` | HMAC key for hash/pseudonymise operators | — |
| `ORACLE_PII_CLOB_LIMIT` | Max characters to scan in CLOB columns | `512` |
| `ORACLE_PII_MIN_CONFIDENCE` | Minimum detection confidence (0.0–1.0) | `0.5` |
| `ORACLE_PII_SCAN_SAMPLE` | Rows sampled per column for value scanning | `20` |
| `ORACLE_PII_ENABLED` | Master switch for PII pipeline | `true` |

---

## Schema Cache Rebuild

The `rebuild_schema_cache` tool triggers a full re-introspection of the Oracle schema.
`ScanPolicy` assignments are recomputed for every column during rebuild. This ensures
that new columns added to Oracle tables, or type changes on existing columns, are
reflected in the PII pipeline without a server restart.

---

## Response Decoration

The agent-facing response includes `_meta` from `ResultMeta`, providing full
provenance of what was done to the data. Example for a query on `CUSTOMERS`:

```json
{
  "data": [
    {
      "CUSTOMER_ID": 1001,
      "FULL_NAME": "<redacted: PERSON>",
      "EMAIL_ADDRESS": "a3f4b2...c8d1",
      "ORDER_COUNT": 14
    }
  ],
  "_meta": {
    "pii_scan_applied": true,
    "framework_version": "1.2.0",
    "column_reports": [
      {
        "column_name": "CUSTOMER_ID",
        "oracle_type": "NUMBER",
        "scan_policy": "safe",
        "pii_detected": false,
        "treatment": "none"
      },
      {
        "column_name": "FULL_NAME",
        "oracle_type": "VARCHAR2",
        "scan_policy": "full",
        "pii_detected": true,
        "pii_entities": ["PERSON"],
        "confidence": 0.82,
        "detection_source": "name_heuristic",
        "treatment": "redacted",
        "treatment_reason": "pii_detected"
      },
      {
        "column_name": "EMAIL_ADDRESS",
        "oracle_type": "VARCHAR2",
        "scan_policy": "full",
        "pii_detected": true,
        "pii_entities": ["EMAIL_ADDRESS"],
        "confidence": 0.95,
        "detection_source": "value_scan",
        "treatment": "hashed",
        "treatment_reason": "static_policy"
      },
      {
        "column_name": "ORDER_COUNT",
        "oracle_type": "NUMBER",
        "scan_policy": "safe",
        "pii_detected": false,
        "treatment": "none"
      }
    ]
  }
}
```

The agent prompt for oracle-mcp should include the following instruction:

> Columns with `treatment` values of `hashed`, `redacted`, `pseudonymised`, or
> `masked` in `_meta.column_reports` cannot be used as literal filter values in
> subsequent queries. Use the original user-supplied value in WHERE clauses instead.
> Columns listed in `_meta.column_reports` with `scan_policy: stripped` indicate
> binary data that cannot be accessed.

---

## Files Changed in oracle-mcp

| File | Change |
|---|---|
| `oracle/schema.go` | Add `ScanPolicy`, `MaxScanLength` to `CachedColumn` |
| `oracle/scanpolicy.go` | New — `scanPolicyForColumn` and type classification helpers |
| `oracle/policy.go` | New — `AccessPolicy`, `LoadPolicy`, `ApplyPolicy` |
| `oracle/hints.go` | New — `buildColumnHints` |
| `oracle/pipeline.go` | New — query result pipeline orchestrator |
| `oracle/tools/execute_read.go` | Return `ToolResult` with `Data` and `ColumnHints` (Tier 3) |
| `oracle/tools/execute_write.go` | Return `ToolResult` with `Data` and `ColumnHints` (Tier 3) |
| `oracle/tools/*.go` (introspection) | Return `ToolResult{Data: ...}` (Tier 2) |
| `go.mod` | Add `github.com/karldane/go-presidio` dependency |

---

## Testing

New test files:

- `oracle/scanpolicy_test.go` — table-driven tests for `scanPolicyForColumn` covering all Oracle types
- `oracle/policy_test.go` — tests for policy loading, whitelist/blacklist application, HMAC treatment
- `oracle/pipeline_test.go` — integration tests for the full pipeline with fixture row data
- `oracle/hints_test.go` — tests for `ColumnHint` construction from cached columns

Existing tests are updated to assert on `ToolResult` rather than `string`. The test
helper `framework.NewTestServer()` is updated in the framework to support `ToolResult`
assertions natively.

---

## Framework Dependency Notes

oracle-mcp is the first Tier 3 consumer of mcp-framework v2.0.0 and will exercise
the new API more deeply than any other backend. During implementation, the agent
working on oracle-mcp is expected to encounter framework gaps, ambiguities, or
bugs that are not visible from the framework spec alone.

### Push-Back Policy

**If something in mcp-framework is wrong, fix it in mcp-framework — do not work
around it in oracle-mcp.**

Specifically: if the agent discovers that a framework type is missing a field,
an interface is awkward to implement correctly, `BindArgs` doesn't handle a case,
`CallContext` is missing something needed by the PII pipeline, or `ToolError` codes
are insufficient — raise it as a framework issue. oracle-mcp should be clean idiomatic
code, not a collection of workarounds for framework limitations.

The framework is under active development in a parallel workstream. Issues found
in oracle-mcp are likely also issues for newrelic-mcp, slack-mcp, and future backends.
Fix them at source.

---

## Interface Updates (mcp-framework v2.0.0)

All tool Handle signatures use `framework.CallContext` not `context.Context`, and
`EnforcerProfile(args map[string]interface{})` not `GetEnforcerProfile()`.

### SessionID-Keyed HMAC

The PII pipeline's pseudonymisation operator should derive its working key by
combining the static `ORACLE_PII_HMAC_KEY` env var with the session ID from
`CallContext.SessionID`. This ensures:

- Same entity value → same pseudonym within a session (agent can reason consistently)
- Different session → different pseudonym (no cross-session correlation)
- No session ID (standalone mode) → fall back to static key only

```go
func derivePipelineKey(baseKey []byte, sessionID string) []byte {
    if sessionID == "" {
        return baseKey
    }
    h := hmac.New(sha256.New, baseKey)
    h.Write([]byte(sessionID))
    return h.Sum(nil)
}
```

This function is called in the pipeline orchestrator at the start of each
`Handle` invocation, not at startup. The `baseKey` is loaded once from the
environment at startup.

### Updated Tool Signatures

```go
// oracle/tools/execute_read.go

func (t *ExecuteReadTool) Handle(
    ctx framework.CallContext,
    args map[string]interface{},
) (framework.ToolResult, error) {
    params, err := framework.BindArgs[ExecuteReadParams](args)
    if err != nil {
        return framework.ErrorResult(framework.ToolError{
            Code:    framework.ErrCodeInvalidArgs,
            Message: err.Error(),
        }), nil
    }
    // ... query execution ...
}

func (t *ExecuteReadTool) EnforcerProfile(args map[string]interface{}) framework.EnforcerProfile {
    return framework.NewEnforcerProfile(
        framework.WithRisk(framework.RiskMed),
        framework.WithImpact(framework.ImpactRead),
        framework.WithApprovalReq(true),
        framework.WithPII(true),
    )
}
```

```go
// oracle/tools/execute_write.go

func (t *ExecuteWriteTool) EnforcerProfile(args map[string]interface{}) framework.EnforcerProfile {
    commit := false
    if args != nil {
        commit, _ = args["commit"].(bool)
    }
    if commit {
        return framework.NewEnforcerProfile(
            framework.WithRisk(framework.RiskHigh),
            framework.WithImpact(framework.ImpactWrite),
            framework.WithApprovalReq(true),
        )
    }
    // Dry-run / rollback-only mode
    return framework.NewEnforcerProfile(
        framework.WithRisk(framework.RiskMed),
        framework.WithImpact(framework.ImpactRead),
        framework.WithApprovalReq(false),
    )
}
```

### Oracle-Specific Error Codes

Define these in `oracle/errors.go` alongside the standard framework codes:

```go
const (
    ErrCodeOracleConnectionFailed = "ORACLE_CONNECTION_FAILED" // retryable: true
    ErrCodeOracleQueryRejected    = "ORACLE_QUERY_REJECTED"    // SQL blocked by classifier
    ErrCodeOraclePolicyDenied     = "ORACLE_POLICY_DENIED"     // static policy denied table/column
    ErrCodeOracleCacheNotReady    = "ORACLE_CACHE_NOT_READY"   // schema cache not yet populated
)
```

Connection failures should always set `Retryable: true`. All others are not retryable.
