# oracle-mcp PII Integration: Stage 2 — Fix and Complete

**Status:** Active  
**Supersedes:** Relevant sections of SPEC_ORACLE_MCP_PII.md  
**Context:** Stage 1 infrastructure (ScanPolicy, ColumnHints, CallContext migration) is  
complete. The PII pipeline is enabled but producing no treatment. This spec describes  
the exact fixes required.

---

## Current State Assessment (from code review)

### What is complete and correct
- `models.go` — `ColumnInfo` has `ScanPolicy` and `MaxScanLength` fields ✓
- `scanpolicy.go` — `scanPolicyForColumn()` is correct and complete ✓
- `hints.go` — `BuildColumnHints()`, `buildHintsFromQuery()` are correct ✓
- `pipeline.go` — `derivePipelineKey()` is correct ✓
- `oracle.go` — `ExecuteReadTool` returns `ToolResult{Data: result.Rows, ColumnHints: hints}` ✓
- `oracle.go` — `CallContext` migration complete, `EnforcerProfile(args)` done ✓
- `oracle.go` — `ExecuteWriteTool.EnforcerProfile` correctly dynamic on `commit` ✓

### What is broken — three specific bugs

**Bug 1 (Critical): `PIIPipelineConfig` is constructed with no defaults**

In `oracle.go` `NewServer()`:
```go
piiConfig = &framework.PIIPipelineConfig{
    HMACKeyEnv: "ORACLE_PII_HMAC_KEY",
    // MinConfidence: 0  → nothing ever exceeds threshold, pipeline always skips
    // DefaultOperator: ""  → even if detected, no treatment applied
    // SampleSize: 0  → no rows sampled, engine sees no values to scan
}
```
All three missing fields have zero values that disable the pipeline in different ways.
This is confirmed by the agent's debug output:
`PIIConfig=&{HMACKeyEnv:ORACLE_PII_HMAC_KEY MinConfidence=0 DefaultOperator: EntityOperators:map[] SampleSize:0}`

**Bug 2 (Critical): `scanPolicyForColumn()` is never called**

In `database.go` `loadTableDetails()`, columns are built as:
```go
col.Nullable = nullable == "Y"
info.Columns = append(info.Columns, col)
// scanPolicyForColumn is NOT called — ScanPolicy and MaxScanLength are always 0
```
`ScanPolicy: 0` means `ScanPolicyDefault` for every column. Depending on how the
framework interprets `Default`, this may cause columns to be skipped entirely.
`BuildColumnHints` faithfully passes these zero values through.

**Bug 3 (Unknown severity): Framework pipeline stage may not process `ToolResult.Data`**

The framework receives `ToolResult{Data: rows, ColumnHints: hints}` but it is not
confirmed that the framework's dispatch pipeline actually invokes go-presidio on this
data before serialising to MCP wire format. If the framework serialises `Data` directly
without a PII stage, Bugs 1 and 2 are moot — fixing them would have no effect.

This must be verified before or alongside fixing Bugs 1 and 2. See the diagnostic
step below.

---

## Fix 1: Populate `PIIPipelineConfig` correctly

Replace the config construction block in `oracle.go` `NewServer()`:

```go
// oracle/config.go  (extract to new file for cleanliness)

func buildPIIConfig() *framework.PIIPipelineConfig {
    cfg := &framework.PIIPipelineConfig{
        HMACKeyEnv:      "ORACLE_PII_HMAC_KEY",
        MinConfidence:   0.5,
        DefaultOperator: "redact",
        SampleSize:      20,
        EntityOperators: map[string]string{},
    }

    if v := os.Getenv("ORACLE_PII_MIN_CONFIDENCE"); v != "" {
        if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 && f <= 1.0 {
            cfg.MinConfidence = f
        }
    }
    if v := os.Getenv("ORACLE_PII_SCAN_SAMPLE"); v != "" {
        if i, err := strconv.Atoi(v); err == nil && i > 0 {
            cfg.SampleSize = i
        }
    }
    if v := os.Getenv("ORACLE_PII_DEFAULT_OPERATOR"); v != "" {
        cfg.DefaultOperator = v
    }

    // Entity-specific operator overrides: ORACLE_PII_OP_EMAIL, ORACLE_PII_OP_PHONE, etc.
    // Pattern: ORACLE_PII_OP_{ENTITY_TYPE_UPPER} = operator
    for _, env := range os.Environ() {
        parts := strings.SplitN(env, "=", 2)
        if len(parts) != 2 {
            continue
        }
        if strings.HasPrefix(parts[0], "ORACLE_PII_OP_") {
            entity := strings.ToLower(strings.TrimPrefix(parts[0], "ORACLE_PII_OP_"))
            cfg.EntityOperators[entity] = parts[1]
        }
    }

    return cfg
}
```

In `NewServer()`, replace the inline config construction:
```go
var piiConfig *framework.PIIPipelineConfig
if os.Getenv("ORACLE_PII_HMAC_KEY") != "" {
    piiConfig = buildPIIConfig()
    fmt.Fprintf(os.Stderr, "[oracle-mcp] PII pipeline enabled: operator=%s confidence=%.2f sample=%d\n",
        piiConfig.DefaultOperator, piiConfig.MinConfidence, piiConfig.SampleSize)
}
piiEnabled := piiConfig != nil
```

The default operator is `"redact"` — not `"hash"` — because redaction requires no
HMAC key and provides the simplest possible test case. Once redaction is confirmed
working, switching to `"hash"` or `"pseudonymise"` is a one-env-var change.

---

## Fix 2: Call `scanPolicyForColumn` in `loadTableDetails`

In `database.go`, in `loadTableDetails`, change the column scan loop:

```go
// Before
for colRows.Next() {
    var col ColumnInfo
    var nullable string
    if err := colRows.Scan(&col.Name, &col.DataType, &nullable); err != nil {
        return nil, err
    }
    col.Nullable = nullable == "Y"
    info.Columns = append(info.Columns, col)
}

// After
for colRows.Next() {
    var col ColumnInfo
    var nullable string
    if err := colRows.Scan(&col.Name, &col.DataType, &nullable); err != nil {
        return nil, err
    }
    col.Nullable = nullable == "Y"
    col.ScanPolicy, col.MaxScanLength = scanPolicyForColumn(col.DataType)
    info.Columns = append(info.Columns, col)
}
```

This is a one-line addition. After this fix, every fully-loaded `TableInfo` will have
correct `ScanPolicy` values, and `BuildColumnHints` will pass them through correctly.

**Note on cache invalidation:** Existing cached `TableInfo` entries will have
`ScanPolicy: 0` because they were built before this fix. The cache should be
invalidated after deploying this fix. Set `ORACLE_CACHE_TTL=0` or delete the cache
files in the `.cache` directory to force a rebuild on next startup.

---

## Fix 3: Verify and fix the framework pipeline stage

Before fixing Bugs 1 and 2, or immediately after (if Bugs 1+2 don't resolve it),
verify whether the framework actually invokes the PII pipeline on `ToolResult.Data`.

**Diagnostic:**
Add a temporary log line in `ExecuteReadTool.Handle` immediately before the return:
```go
fmt.Fprintf(os.Stderr, "[PII-DIAG] Returning ToolResult: DataNil=%v HintsLen=%d\n",
    hints == nil, len(hints))
```

Then observe what the MCP client receives. If the client receives raw unredacted data
AND the hints are non-nil AND the config is correct (after Fix 1), then the framework
pipeline stage is not processing `ToolResult.Data`.

**If the framework pipeline is not wired:**

This is a framework bug, not an oracle-mcp bug. Per the push-back policy in
SPEC_ORACLE_MCP_PII.md: fix it in the framework, not in oracle-mcp.

The framework's tool dispatch path should:
1. Call `tool.Handle(ctx, args)` → `ToolResult`
2. If `ToolResult.Data != nil && ToolResult.ColumnHints != nil && piiEnabled`:
   - Run `pipelineEngine.ProcessResult(ctx, result)` 
   - Which invokes go-presidio `StructuredAnalyzer` on `Data` using `ColumnHints`
   - Applies treatment (redact/hash/mask) to matching values in-place
   - Sets `ToolResult.Meta.ColumnReports`
3. Serialise the (now-treated) `ToolResult` to MCP content array

If this stage is missing from the framework dispatch loop, add it. The call site in
the framework is wherever `tool.Handle()` is called and the result is serialised.

**Interim fallback (only if framework fix will be delayed):**

Implement PII scanning directly in `ExecuteReadTool.Handle` using go-presidio:

```go
func (t *ExecuteReadTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
    // ... existing query execution ...

    hints := buildHintsFromQuery(ctx, result, sql, executor)

    // If PII scanning is enabled, apply treatment directly
    if piiCfg := t.piiConfig; piiCfg != nil && hints != nil {
        pipeKey := derivePipelineKey(piiCfg.HMACKey(), ctx.SessionID)
        engine := presidio.NewStructuredAnalyzer(
            presidio.DefaultRegistry(),
            piiCfg.ToAnonymizerConfig(pipeKey),
        )
        treated, reports, err := engine.AnonymizeRows(result.Rows, hints)
        if err == nil {
            result.Rows = treated
            return framework.ToolResult{
                Data:        result.Rows,
                ColumnHints: hints,
                Meta:        framework.ResultMeta{ColumnReports: reports},
            }, nil
        }
        fmt.Fprintf(os.Stderr, "[oracle-mcp] PII treatment failed: %v\n", err)
    }

    return framework.ToolResult{Data: result.Rows, ColumnHints: hints}, nil
}
```

This approach makes oracle-mcp self-contained for PII treatment but duplicates logic
that should live in the framework. Treat it as temporary. Raise the framework gap as
a separate issue.

The tool will need `piiConfig *PIIRuntimeConfig` injected at construction time in
`registerTools()`. See the Configuration Injection section below.

---

## Configuration Injection (needed for fallback path)

If the direct scanning path is used, `ExecuteReadTool` needs access to the PII
config at call time. The cleanest approach is to pass it at construction:

```go
// oracle/pii_runtime.go

// PIIRuntimeConfig holds resolved PII configuration for direct-scan mode.
// Populated from buildPIIConfig() at startup. Nil if PII is disabled.
type PIIRuntimeConfig struct {
    HMACKey         []byte
    MinConfidence   float64
    DefaultOperator string
    SampleSize      int
    EntityOperators map[string]string
}

func newPIIRuntimeConfig(cfg *framework.PIIPipelineConfig) (*PIIRuntimeConfig, error) {
    if cfg == nil {
        return nil, nil
    }
    key := []byte(os.Getenv(cfg.HMACKeyEnv))
    if len(key) == 0 {
        return nil, fmt.Errorf("ORACLE_PII_HMAC_KEY is set but empty")
    }
    return &PIIRuntimeConfig{
        HMACKey:         key,
        MinConfidence:   cfg.MinConfidence,
        DefaultOperator: cfg.DefaultOperator,
        SampleSize:      cfg.SampleSize,
        EntityOperators: cfg.EntityOperators,
    }, nil
}
```

In `registerTools()`:
```go
func (s *Server) registerTools() {
    piiRT, _ := newPIIRuntimeConfig(s.piiFrameworkConfig) // store cfg on Server during NewServer

    // ...
    s.RegisterTool(&ExecuteReadTool{db: s.db, piiConfig: piiRT})
    s.RegisterTool(&ExecuteWriteTool{db: s.db, server: s})
}
```

---

## Test Protocol

After applying all fixes, test in this exact sequence:

### Step 1 — Redaction baseline (no HMAC needed)
```bash
export ORACLE_CONNECTION_STRING=<your-conn-string>
export ORACLE_PII_HMAC_KEY=test-key-not-for-production
export ORACLE_PII_DEFAULT_OPERATOR=redact
# MinConfidence and SampleSize will use defaults (0.5, 20)

echo '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"oracle_execute_read","arguments":{"sql":"SELECT COMP_FINANCE_EMAIL FROM <your_table> FETCH FIRST 5 ROWS ONLY"}}}' \
  | bin/oracle-mcp 2>/tmp/oracle-mcp-debug.log
```

Expected: email values replaced with `<REDACTED:EMAIL_ADDRESS>` or equivalent.  
If raw emails still appear: check `/tmp/oracle-mcp-debug.log` for pipeline diagnostics.

### Step 2 — Structured response shape
The JSON response should contain `_meta` with column reports:
```json
{
  "result": {
    "content": [{"type": "text", "text": "..."}],
    "_meta": {
      "column_reports": {
        "COMP_FINANCE_EMAIL": {
          "pii_detected": true,
          "entity_types": ["EMAIL_ADDRESS"],
          "treatment": "redacted",
          "rows_scanned": 5,
          "rows_treated": 5
        }
      }
    }
  }
}
```

### Step 3 — Hash/pseudonymise (HMAC path)
```bash
export ORACLE_PII_DEFAULT_OPERATOR=hash
# Re-run Step 1 query
```
Expected: emails replaced with hex strings. Same email in two rows should produce
the same hash. Different sessions should produce different hashes (once bridge
populates SessionID).

### Step 4 — ScanPolicySafe bypass
Run a query that returns only numeric columns (e.g. `SELECT invoice_id, amount FROM ...`).
Expected: no PII pipeline output — these columns have `ScanPolicySafe` and are skipped.
Verify via debug log that they appear in hints with `ScanPolicy: 1`.

### Step 5 — Column name boost (ScanPolicyNameOnly)
Run a query returning a `DATE` column with a name like `DATE_OF_BIRTH`.
Expected: despite no regex match on the value, column name heuristic boosts confidence.
(This tests the `ScanPolicyNameOnly` path in go-presidio.)

---

## Files to Change

| File | Change | Bug fixed |
|---|---|---|
| `oracle/config.go` (new) | `buildPIIConfig()` with defaults and env var reading | Bug 1 |
| `oracle/oracle.go` | Call `buildPIIConfig()` in `NewServer()`; inject into `registerTools()` | Bug 1 |
| `oracle/database.go` | Add `scanPolicyForColumn` call in `loadTableDetails` | Bug 2 |
| `mcp-framework` dispatch | Add PII pipeline stage between `tool.Handle()` and serialisation | Bug 3 |
| `oracle/oracle.go` (fallback) | Direct `go-presidio` call in `ExecuteReadTool.Handle` | Bug 3 fallback |
| `oracle/pii_runtime.go` (new) | `PIIRuntimeConfig` type and constructor | Bug 3 fallback |

---

## Push-Back Note

If the framework pipeline stage (Bug 3) is missing or incorrectly implemented, that
is a framework defect. Raise it as a framework issue. Do not duplicate the pipeline
logic in oracle-mcp permanently. The fallback path in this spec is explicitly
temporary — it must be removed once the framework pipeline is confirmed working.

The oracle-mcp codebase should remain clean: query execution, hint building, and
config wiring. PII treatment belongs in the framework's dispatch pipeline.

---

## Bug 4 (Architecture): oracle-mcp imports go-presidio directly

This is a dependency layering violation discovered during code review. It must be fixed
before or alongside the other bugs, as it affects which types the agent uses when
implementing the Bug 3 fallback path.

### The violation

`hints.go` imports `github.com/karldane/go-presidio/presidio` and returns
`map[string]presidio.ColumnHint`. Because `framework.ToolResult.ColumnHints` is
typed to accept this, the framework's own public API is coupled to a go-presidio
concrete type. oracle-mcp ends up with a hard transitive dependency on go-presidio.

`pipeline.go` implements `derivePipelineKey()` — pure HMAC key derivation logic that
is not oracle-specific at all. Any future MCP backend using the `hash` operator would
need the identical function. It belongs in mcp-framework, not oracle-mcp.

### The correct dependency graph

```
oracle-mcp  →  mcp-framework  →  go-presidio
               (owns PII types)   (implementation detail)
```

oracle-mcp must have zero imports from `go-presidio`. It only expresses *intent*
(scan policies, column metadata) using framework-owned types. The framework maps those
to presidio internals privately.

### Fix 4a — mcp-framework owns ColumnHint and ScanPolicy

Add to mcp-framework (e.g. `framework/pii.go`):

```go
// ScanPolicy describes how a column should be handled by the PII pipeline.
type ScanPolicy int

const (
    ScanPolicyDefault          ScanPolicy = 0 // use pipeline default
    ScanPolicySafe             ScanPolicy = 1 // skip scanning - known safe type
    ScanPolicyNameOnly         ScanPolicy = 2 // scan name heuristic only
    ScanPolicyStrip            ScanPolicy = 3 // strip binary / no scan
    ScanPolicyTruncateThenScan ScanPolicy = 4 // truncate then scan
    ScanPolicyFull             ScanPolicy = 5 // full scan
)

// ColumnHint carries per-column scanning metadata from a backend tool to the
// PII pipeline. It is a framework type; backends must not import go-presidio.
type ColumnHint struct {
    ScanPolicy ScanPolicy
    MaxLength  int // 0 = use pipeline default
}
```

Update `framework.ToolResult`:

```go
type ToolResult struct {
    RawText     string
    Data        interface{}
    ColumnHints map[string]ColumnHint  // framework type, not presidio type
    Meta        ResultMeta
}
```

The framework internally maps `framework.ColumnHint → presidio.ColumnHint` when
invoking go-presidio. This mapping is unexported and invisible to backends:

```go
// framework/pii_internal.go (unexported)
func toPresidioHints(hints map[string]ColumnHint) map[string]presidio.ColumnHint {
    out := make(map[string]presidio.ColumnHint, len(hints))
    for col, h := range hints {
        out[col] = presidio.ColumnHint{
            ScanPolicy: presidio.ScanPolicy(h.ScanPolicy),
            MaxLength:  h.MaxLength,
        }
    }
    return out
}
```

### Fix 4b — mcp-framework owns DerivePipelineKey

Move `derivePipelineKey` from oracle-mcp `pipeline.go` to mcp-framework and export it:

```go
// framework/pipeline.go

// DerivePipelineKey derives a per-session HMAC key from the base key and
// session ID. If sessionID is empty, the base key is returned unchanged.
func DerivePipelineKey(baseKey []byte, sessionID string) []byte {
    if sessionID == "" {
        return baseKey
    }
    h := hmac.New(sha256.New, baseKey)
    h.Write([]byte(sessionID))
    return h.Sum(nil)
}
```

oracle-mcp's `pipeline.go` is then deleted entirely. Any reference in oracle-mcp to
`derivePipelineKey(...)` becomes `framework.DerivePipelineKey(...)`.

### Fix 4c — oracle-mcp hints.go removes the presidio import

```go
// oracle/hints.go — after fix

import (
    "context"
    "regexp"
    "strings"

    "github.com/karldane/mcp-framework/framework"
    // No go-presidio import
)

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

The function signatures of `buildHintsFromQuery` and `buildHintsFromWriteQuery` also
change their return type from `map[string]presidio.ColumnHint` to
`map[string]framework.ColumnHint`. No other logic changes.

### Fix 4d — oracle-mcp models.go ScanPolicy constants

The local `ScanPolicyX int` constants in `models.go` are used to set
`ColumnInfo.ScanPolicy` in `scanpolicy.go` and to cast into `framework.ScanPolicy`
in `hints.go`. After Fix 4c, `hints.go` casts `int → framework.ScanPolicy`.

Two acceptable approaches:

**Option A — keep local int constants, cast at the boundary (minimal change)**
No change to `models.go`. `hints.go` casts `framework.ScanPolicy(col.ScanPolicy)` as
it already does. The local constants remain as plain `int` for use in `scanpolicy.go`.
This is the least-churn option and is acceptable.

**Option B — alias the framework type (cleaner, more churn)**
Remove the local `ScanPolicyX` constants from `models.go`. Change `ColumnInfo.ScanPolicy`
from `int` to `framework.ScanPolicy`. Update `scanpolicy.go` to return
`framework.ScanPolicy` instead of a pair of `int, int`. This makes the type intent
explicit throughout but requires touching more files.

**Recommendation: Option A for now.** The cast is safe because the int values are
defined to match. Option B can be done as a separate clean-up PR once the pipeline
is confirmed working.

### Updated file change table

| File | Change | Bug fixed |
|---|---|---|
| `mcp-framework/framework/pii.go` (new) | `ScanPolicy` type, `ColumnHint` type, constants | Bug 4 |
| `mcp-framework/framework/pii_internal.go` (new) | `toPresidioHints()` unexported mapping | Bug 4 |
| `mcp-framework/framework/pipeline.go` (new) | `DerivePipelineKey()` exported | Bug 4b |
| `mcp-framework/framework/tool.go` | `ToolResult.ColumnHints` type → `map[string]ColumnHint` | Bug 4 |
| `oracle/hints.go` | Remove presidio import; return `map[string]framework.ColumnHint` | Bug 4c |
| `oracle/pipeline.go` | Delete file | Bug 4b |
| `oracle/config.go` (new) | `buildPIIConfig()` with defaults and env var reading | Bug 1 |
| `oracle/oracle.go` | Call `buildPIIConfig()`; reference `framework.DerivePipelineKey` | Bug 1, 4b |
| `oracle/database.go` | Add `scanPolicyForColumn` call in `loadTableDetails` | Bug 2 |
| `mcp-framework` dispatch | Add PII pipeline stage between `Handle()` and serialisation | Bug 3 |
| `oracle/oracle.go` (fallback) | Direct scan in `ExecuteReadTool.Handle` if framework gap confirmed | Bug 3 fallback |
| `oracle/pii_runtime.go` (new) | `PIIRuntimeConfig` — only needed for fallback path | Bug 3 fallback |

### Sequencing

Fix Bug 4 (the layering violation) **first**, before implementing Bug 3's fallback path.
If the fallback path is written before Fix 4, it will introduce more presidio imports
into oracle-mcp, making the cleanup harder. The correct order is:

1. Fix 4 — mcp-framework adds owned types; oracle-mcp removes presidio import
2. Fix 1 — `buildPIIConfig()` with proper defaults
3. Fix 2 — `scanPolicyForColumn` called in `loadTableDetails`
4. Diagnostic — confirm whether framework pipeline stage processes `ToolResult.Data`
5. Fix 3 — framework pipeline stage (preferred) or fallback (temporary)

---

## REVISION — applies after SPEC_MCP_FRAMEWORK_PII_FIX.md

The framework code has now been fully reviewed. The following sections of this spec
are superseded. Pass this revision to the agent alongside the rest of the document.

---

### Sections that are superseded — do NOT implement

**"Fix 3: Verify and fix the framework pipeline stage"**  
The diagnostic step is no longer needed. The bug is confirmed (F3 in the framework
spec) and is fixed there. Do not add any diagnostic log lines to oracle-mcp.

**"Interim fallback" in Fix 3**  
Do not implement the fallback path in `ExecuteReadTool.Handle`. Do not create a
local `presidio.StructuredAnalyzer` call in oracle-mcp. The framework fix (F3) is
the correct fix.

**"Configuration Injection (needed for fallback path)"**  
Do not create `pii_runtime.go`. Do not add `piiConfig *PIIRuntimeConfig` to
`ExecuteReadTool`. This section exists only to support the fallback path, which is
not being built.

---

### Sections that are still correct — implement as written

- **Fix 1** (`buildPIIConfig()` in `oracle.go`) — unchanged, still required  
- **Fix 2** (`scanPolicyForColumn` in `loadTableDetails`) — unchanged, still required  
- **Bug 4c** (`hints.go` removes presidio import) — still correct; return type becomes
  `map[string]framework.ColumnHint` once the framework F1 types exist  
- **Bug 4d** (`models.go` Option A) — still correct; local int constants, cast at boundary

---

### Bug 4 simplification

The framework spec (SPEC_MCP_FRAMEWORK_PII_FIX.md) implements:
- **Fix 4a** — `framework.ScanPolicy`, `framework.ColumnHint`, `framework.ColumnReport`
  types are added in `result.go`. Oracle-mcp does not define these — it uses them.
- **Fix 4b** — `derivePipelineKey` does not need to move to the framework. Key
  derivation lives inside `PIIPipeline.Process()` internally. `oracle/pipeline.go`
  is simply deleted with no replacement.

The oracle-mcp side of Bug 4 is therefore just:

1. Delete `oracle/pipeline.go`
2. In `oracle/hints.go`: change `import "github.com/karldane/go-presidio/presidio"` 
   to `import "github.com/karldane/mcp-framework/framework"` and change the return
   type of `BuildColumnHints`, `buildHintsFromQuery`, and `buildHintsFromWriteQuery`
   from `map[string]presidio.ColumnHint` to `map[string]framework.ColumnHint`.
   Change the struct literal from `presidio.ColumnHint{...}` to `framework.ColumnHint{...}`.
   No other logic changes.
3. `oracle/models.go` — no changes required (Option A: keep local int constants).

---

### Corrected sequencing (across both repos)

Apply in this order. Steps 1–4 are in mcp-framework; steps 5–8 are in oracle-mcp.

1. **framework F1+F2** — add owned types in `result.go`; add converters in `piipipeline.go`
   *(commit together — they are a breaking pair)*
2. **framework F5** — fix `NewPIIPipeline` SampleSize bug
3. **framework F3** — wire pipeline into `Initialize()` closure in `server.go`
4. **framework F4** — fix `formatDataResult` in `server.go`
5. **oracle-mcp Bug 4** — delete `pipeline.go`; update `hints.go` to use framework types
   *(can only be done after step 1 — oracle-mcp will not compile until framework types exist)*
6. **oracle-mcp Bug 1** — `buildPIIConfig()` with correct env var defaults
7. **oracle-mcp Bug 2** — `scanPolicyForColumn` called in `loadTableDetails`
8. Run the test protocol from this spec (Steps 1–5)

---

### Updated oracle-mcp file change table

| File | Change | Source |
|---|---|---|
| `oracle/pipeline.go` | Delete entirely | Bug 4b |
| `oracle/hints.go` | Remove presidio import; use `framework.ColumnHint` | Bug 4c |
| `oracle/oracle.go` | Add `buildPIIConfig()`; remove zero-value config bug | Bug 1 |
| `oracle/database.go` | Call `scanPolicyForColumn` in `loadTableDetails` | Bug 2 |
| `oracle/models.go` | No changes | — |
| `oracle/pii_runtime.go` | Do not create | superseded |
