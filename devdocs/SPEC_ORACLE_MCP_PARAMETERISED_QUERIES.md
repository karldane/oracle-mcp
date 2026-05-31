# SPEC_ORACLE_MCP_PARAMETERISED_QUERIES

## Goal

Add a `params` argument to `oracle_execute_read` and `oracle_execute_write` so that PII
tokens received from previous query results can be passed as named bind variables rather than
interpolated into the SQL string. This allows `PIIPipeline.Resolve()` to decrypt them cleanly
before execution, and eliminates a class of SQL injection risk.

## Background

The current tool contract accepts a single `sql` string. When the LLM receives encrypted PII
values in query results (e.g. `CONT_SURNAME = 'pii:a3f8c2...'`) and then constructs a
follow-up query filtering on that value, it has no choice but to embed the token inside the
SQL string:

```sql
SELECT * FROM CONTACTS WHERE CONT_SURNAME = 'pii:a3f8c2...'
```

The framework's `PIIPipeline.Resolve()` scans top-level `args` values only. It sees `args["sql"]`
as a single opaque string and passes it through untouched. Oracle then executes the literal
token string, which matches nothing.

The fix is a contract change: PII values — and any other parameterised values — must be
supplied separately in a `params` map, leaving the SQL string as a static template.

---

## Changes: `oracle_execute_read`

### Schema (before)

```go
"sql": { "type": "string", "description": "SELECT SQL query to execute" }
```

### Schema (after)

```go
"sql": {
    "type": "string",
    "description": "SELECT SQL query to execute. Use named placeholders (e.g. :surname) " +
        "for any values received from previous query results. Pass those values in params.",
},
"params": {
    "type": "object",
    "description": "Named bind parameters for the query. Any encrypted PII values " +
        "returned by previous queries must be passed here, not interpolated into sql. " +
        "Keys must match the placeholder names in sql (e.g. {"surname": "pii:a3f8c2..."}).",
    "additionalProperties": { "type": "string" },
},
```

`params` is optional. Queries without parameters work exactly as before.

### Handle (before)

```go
result, err := executor.ExecuteQuery(ctx, sql, maxRows)
```

### Handle (after)

```go
// Extract params map — may be nil for plain queries
var bindParams map[string]interface{}
if p, ok := args["params"].(map[string]interface{}); ok {
    bindParams = p
}

result, err := executor.ExecuteQuery(ctx, sql, maxRows, bindParams)
```

The framework's `Resolve()` runs on the top-level `args` map before `Handle` is called. It
will decrypt any `pii:` values in `args["params"]` automatically, because `params` is a
`map[string]interface{}` nested one level in. **This requires `Resolve()` to recurse one
level into nested object arguments** — see the framework note at the end of this spec.

---

## Changes: `oracle_execute_write`

Identical pattern. Add `params` to `Schema()` and extract it in `Handle()` before passing
to `executor.ExecuteWrite()`.

### Schema addition

```go
"params": {
    "type": "object",
    "description": "Named bind parameters for the DML statement. Pass any encrypted PII " +
        "values here rather than interpolating them into sql.",
    "additionalProperties": { "type": "string" },
},
```

### Handle addition

```go
var bindParams map[string]interface{}
if p, ok := args["params"].(map[string]interface{}); ok {
    bindParams = p
}

result, err := executor.ExecuteWrite(ctx, sql, commit, bindParams)
```

---

## Changes: `database.go` — `ExecuteQuery` and `ExecuteWrite`

Both functions need a `bindParams map[string]interface{}` parameter added. When `bindParams`
is non-nil and non-empty, pass them as named bind variables to the Oracle driver.

Using `godror` / `database/sql`, named parameters are passed as `sql.Named`:

```go
func (e *Executor) ExecuteQuery(ctx context.Context, sql string, maxRows int, params map[string]interface{}) (*QueryResult, error) {
    var namedArgs []interface{}
    for k, v := range params {
        namedArgs = append(namedArgs, godror.NamedArg{Name: k, Value: v})
    }
    rows, err := e.db.QueryContext(ctx, sql, namedArgs...)
    // ... existing result handling unchanged
}
```

If `params` is nil or empty, `namedArgs` is empty and `QueryContext` behaves identically to
before.

---

## Tool Description — LLM Instruction

The description strings for both tools must make the contract explicit so the LLM applies it
consistently. Update `Description()` on `ExecuteReadTool`:

```go
func (t *ExecuteReadTool) Description() string {
    base := "Execute a read-only SELECT query (limited to 100 rows by default). " +
        "If you are filtering on a value received from a previous query result " +
        "(including encrypted pii: values), you MUST pass that value in params " +
        "using a named placeholder in sql (e.g. sql: " +
        ""SELECT * FROM CONTACTS WHERE CONT_SURNAME = :surname", " +
        "params: {"surname": "<value>"}). " +
        "Do not interpolate received values directly into the sql string."
    if t.db.IsMultiDatabase() {
        return base + " Requires 'database' parameter."
    }
    return base
}
```

Apply the same addition to `ExecuteWriteTool.Description()`.

---

## Framework Note: `Resolve()` Must Recurse One Level

`PIIPipeline.Resolve()` currently scans only top-level string values in `args`. The `params`
map is a nested `map[string]interface{}` at `args["params"]`. To decrypt tokens in it,
`Resolve()` must detect nested maps and recurse one level:

```go
for k, v := range args {
    switch val := v.(type) {
    case string:
        if strings.HasPrefix(val, "pii:") {
            // decrypt and replace
        } else {
            resolved[k] = val
        }
    case map[string]interface{}:
        // recurse one level for nested param maps
        inner, err := p.Resolve(val)
        if err != nil {
            return nil, fmt.Errorf("pii resolve: nested arg %q: %w", k, err)
        }
        resolved[k] = inner
    default:
        resolved[k] = v
    }
}
```

This change is in `mcp-framework/piipipeline.go`. It is the only framework change required.
The recursion is intentionally limited to one level — deeply nested structures are out of scope.

---

## What Does Not Change

- `oracle_explain_query` — explain takes a SQL template; users should not be filtering on
  PII values in an EXPLAIN. No params needed.
- All schema introspection tools — no parameterised input.
- `NewServer` in `oracle.go` — PII pipeline configuration unchanged.
- All other `args` handling in existing tools — unaffected.

---

## End-to-End Flow After This Change

```
1. LLM calls oracle_execute_read:
   { sql: "SELECT * FROM CONTACTS WHERE CONT_SURNAME = :surname",
     params: { surname: "pii:a3f8c2..." } }

2. mcp-framework Initialize() closure:
   PIIPipeline.Resolve(args)
     → sees args["params"] is a map, recurses
     → decrypts "pii:a3f8c2..." → "Bowditch"
     → args["params"] = { surname: "Bowditch" }

3. ExecuteReadTool.Handle receives:
   sql = "SELECT * FROM CONTACTS WHERE CONT_SURNAME = :surname"
   params = { surname: "Bowditch" }

4. executor.ExecuteQuery binds surname="Bowditch" as a named parameter.
   Oracle executes: SELECT * FROM CONTACTS WHERE CONT_SURNAME = :surname
   with bind value "Bowditch" — cursor cache hit on subsequent calls.

5. Result rows returned. PIIPipeline.Process() encrypts PII in outbound result.
   LLM receives { CONT_SURNAME: "pii:a3f8c2...", ... }
```

---

## Required Tests

| Test                                              | Assertion                                                    |
|---------------------------------------------------|--------------------------------------------------------------|
| `TestExecuteReadWithParams`                       | Named params are bound and query executes correctly          |
| `TestExecuteReadWithPIIParams`                    | pii: token in params is decrypted before execution          |
| `TestExecuteReadWithoutParams`                    | Existing plain SQL queries work unchanged                    |
| `TestExecuteWriteWithParams`                      | Named params bound for DML                                   |
| `TestExecuteWriteWithPIIParams`                   | pii: token in write params decrypted before execution        |
| `TestResolveRecursesIntoNestedMap`                | Framework Resolve() decrypts tokens nested in params map     |
| `TestResolveRecursionLimitedToOneLevel`           | Doubly-nested maps are passed through unchanged              |
