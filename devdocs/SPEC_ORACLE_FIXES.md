# oracle-mcp Improvement Specification

**Version:** 1.0  
**Status:** Draft  
**Date:** 2026-05-01  
**Scope:** Fixes for three issues identified in client review of `oracle-mcp` backend

---

## Background & Constraints

This spec targets fixes that are fully compliant with the MCP specification
(draft, 2025-06-18+). The following protocol rules govern every change here:

- **`content` MUST NOT be removed.** Per the MCP spec: *"For backwards
  compatibility, a tool that returns structured content SHOULD also return the
  serialized JSON in a TextContent block."* opencode (sst/opencode) consumes
  tool responses via `convertMcpTool()` / Vercel AI SDK `dynamicTool()` and
  expects `content` to be present.
- **`structuredContent` is additive.** It sits alongside `content`, not
  instead of it.
- **`outputSchema` is optional but recommended.** When declared on a tool,
  the server MUST return `structuredContent` conforming to it, and clients
  SHOULD validate against it.
- **`inputSchema` must remain a valid JSON Schema object.** Optional
  parameters belong in `properties` but must be absent from `required`.

---

## Fix 1 — Complete `inputSchema` declarations in `expand` summaries

### Problem

`MCP_Bridge_oracle_expand` summarises tools showing only required parameters.
Optional but functionally critical parameters (e.g. `params` on
`oracle_execute_read`) are invisible to the client. This forces trial-and-error
and prevents safe use of parameterised queries.

### Root cause

The `expand` handler iterates only the `required` array of each tool's
`inputSchema` when building its summary, ignoring `properties`.

### Required change

The `expand` output must enumerate **all** parameters from `properties`,
clearly marking each as required or optional.

#### Expand summary format (before)

```
oracle_execute_read
  Required params: sql (string)
```

#### Expand summary format (after)

```
oracle_execute_read
  Required params:
    - sql (string): The SQL SELECT statement to execute
  Optional params:
    - params (object): Bind parameters for parameterised queries and PII
                       token resolution. Use instead of string interpolation.
```

### Implementation notes

- Source the parameter list from `inputSchema.properties`.
- Classify each key: required if present in `inputSchema.required`, else optional.
- Include the `description` field from each property schema if available.
- No protocol changes required — this is purely a change to the expand
  handler's rendering logic.

---

## Fix 2 — Return `structuredContent` from discovery tools

### Problem

`oracle_list_tables` and `oracle_describe_table` return pre-formatted text
blobs. Clients must parse human-readable strings to extract data, which is
fragile and defeats the purpose of a machine-to-machine protocol.

### Affected tools

| Tool | Current response | Target response |
|------|-----------------|-----------------|
| `oracle_list_tables` | Single text string with heading + bullets | `structuredContent` with `tables` array |
| `oracle_describe_table` | Formatted text report | `structuredContent` with typed column objects |

### Required changes

#### 2a — `oracle_list_tables`

**Add `outputSchema` to tool definition:**

```json
{
  "name": "oracle_list_tables",
  "outputSchema": {
    "type": "object",
    "properties": {
      "tables": {
        "type": "array",
        "items": { "type": "string" },
        "description": "List of table names accessible to the current user"
      },
      "count": {
        "type": "integer",
        "description": "Total number of tables returned"
      }
    },
    "required": ["tables", "count"]
  }
}
```

**Tool response shape:**

```json
{
  "structuredContent": {
    "tables": ["VEHICLE", "POLICY", "CUSTOMER"],
    "count": 3
  },
  "content": [
    {
      "type": "text",
      "text": "{"tables":["VEHICLE","POLICY","CUSTOMER"],"count":3}"
    }
  ]
}
```

> **Protocol note:** `content` carries the serialised JSON string. This is
> required for backwards compatibility per the MCP spec. Do NOT remove it.
> opencode reads tool results via the Vercel AI SDK which may fall back to
> `content` if `structuredContent` is absent.

---

#### 2b — `oracle_describe_table`

**Add `outputSchema` to tool definition:**

```json
{
  "name": "oracle_describe_table",
  "outputSchema": {
    "type": "object",
    "properties": {
      "table": {
        "type": "string",
        "description": "Table name"
      },
      "columns": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "name":       { "type": "string" },
            "data_type":  { "type": "string" },
            "nullable":   { "type": "boolean" },
            "primary_key":{ "type": "boolean" },
            "foreign_key": {
              "type": ["object", "null"],
              "properties": {
                "references_table":  { "type": "string" },
                "references_column": { "type": "string" }
              }
            }
          },
          "required": ["name", "data_type", "nullable", "primary_key"]
        }
      }
    },
    "required": ["table", "columns"]
  }
}
```

**Tool response shape:**

```json
{
  "structuredContent": {
    "table": "VEHICLE",
    "columns": [
      {
        "name": "VEHICLE_ID",
        "data_type": "NUMBER",
        "nullable": false,
        "primary_key": true,
        "foreign_key": null
      },
      {
        "name": "OWNER_ID",
        "data_type": "NUMBER",
        "nullable": false,
        "primary_key": false,
        "foreign_key": {
          "references_table": "CUSTOMER",
          "references_column": "CUSTOMER_ID"
        }
      }
    ]
  },
  "content": [
    {
      "type": "text",
      "text": "{"table":"VEHICLE","columns":[...]}"
    }
  ]
}
```

---

## Fix 3 — Restructure `oracle_execute_read` response

### Problem

`oracle_execute_read` returns data in two places (`content` and
`structuredContent`) with inconsistent representations. The PII Scan Report
is trapped in a human-readable text block in `content[1]`, making it
inaccessible to clients without text parsing.

### Current response shape

```json
{
  "structuredContent": { "rows": [...], "row_count": 5 },
  "content": [
    { "type": "text", "text": "{"rows":[...],"row_count":5}" },
    { "type": "text", "text": "=== PII Scan Report ===
COLUMN: EMAIL
Detected: true
..." }
  ]
}
```

### Required changes

#### 3a — Promote PII data into `structuredContent`

Move all PII metadata from the text report into the `columns` array within
`structuredContent`. The human-readable PII report in `content[1]` must be
replaced by the serialised JSON of the full structured response.

**Updated `outputSchema`:**

```json
{
  "name": "oracle_execute_read",
  "outputSchema": {
    "type": "object",
    "properties": {
      "rows": {
        "type": "array",
        "items": { "type": "object" },
        "description": "Result rows as objects keyed by column name"
      },
      "row_count": {
        "type": "integer"
      },
      "columns": {
        "type": "array",
        "items": {
          "type": "object",
          "properties": {
            "name":         { "type": "string" },
            "pii_detected": { "type": "boolean" },
            "entity_types": {
              "type": "array",
              "items": { "type": "string" }
            },
            "treatment": {
              "type": "string",
              "enum": ["none", "masked", "tokenised", "redacted"]
            }
          },
          "required": ["name", "pii_detected", "treatment"]
        }
      }
    },
    "required": ["rows", "row_count", "columns"]
  }
}
```

**Updated tool response shape:**

```json
{
  "structuredContent": {
    "rows": [
      { "VEHICLE_ID": 1, "EMAIL": "[MASKED]", "OWNER": "J. Smith" }
    ],
    "row_count": 1,
    "columns": [
      {
        "name": "VEHICLE_ID",
        "pii_detected": false,
        "entity_types": [],
        "treatment": "none"
      },
      {
        "name": "EMAIL",
        "pii_detected": true,
        "entity_types": ["EMAIL_ADDRESS"],
        "treatment": "masked"
      },
      {
        "name": "OWNER",
        "pii_detected": false,
        "entity_types": [],
        "treatment": "none"
      }
    ]
  },
  "content": [
    {
      "type": "text",
      "text": "{"rows":[...],"row_count":1,"columns":[...]}"
    }
  ]
}
```

#### 3b — Consolidate `content` to a single item

The previous two-item `content` array (data JSON + PII report text) is
replaced by a **single** `TextContent` item carrying the serialised JSON
of the full `structuredContent` object. This keeps the backwards-compat
`content` field intact while eliminating the redundant and confusing
split representation.

> **Protocol note:** Do NOT remove `content` entirely. The single remaining
> item is the backwards-compatibility serialisation required by the MCP spec.

---

## Compatibility matrix

| Change | `content` field | `structuredContent` field | opencode impact |
|--------|----------------|--------------------------|-----------------|
| Fix 1 (expand summaries) | Not applicable | Not applicable | None — expand handler only |
| Fix 2a (`list_tables`) | Serialised JSON string added | New | None — additive |
| Fix 2b (`describe_table`) | Serialised JSON string added | New | None — additive |
| Fix 3 (`execute_read`) | Consolidated to 1 item | Extended with `columns` | Low — clients using `structuredContent` gain PII data; clients reading `content[0]` get richer JSON |

---

## Implementation Notes — Output Schema Wiring

The `outputSchema` is declared on `mcp.Tool` at registration time, but the current `ToolHandler` interface in mcp-framework only covers input. Two changes are needed:

### mcp-framework — server.go

Add `OutputSchema()` to the `ToolHandler` interface:

```go
type ToolHandler interface {
    Name() string
    Description() string
    Schema() mcp.ToolInputSchema
    OutputSchema() *mcp.ToolOutputSchema  // new
    Execute(ctx context.Context, params json.RawMessage) (any, error)
}
```

Wire it into `mcp.Tool` in the `Initialize()` loop:

```go
tool := mcp.Tool{
    Name:         handler.Name(),
    Description:  handler.Description(),
    InputSchema:  handler.Schema(),
    OutputSchema: handler.OutputSchema(),  // new
}
```

If `OutputSchema()` returns `nil`, the framework skips setting the field, keeping existing tools non-breaking.

### oracle-mcp — tool implementations

Implement `OutputSchema()` on the following tools:

| Tool | Schema to return |
|------|-----------------|
| `ExecuteReadTool` | Fix 3's extended schema with `rows`, `row_count`, `columns` |
| `ExecuteWriteTool` | `nil` (existing text response unchanged) |
| `ListTablesTool` | Fix 2a's `tables` + `count` schema |
| `DescribeTableTool` | Fix 2b's `table` + `columns` schema |

All other tools return `nil` from `OutputSchema()`.

---

## Testing requirements

For each fix, the following should be verified:

1. **Schema conformance:** `structuredContent` validates against the declared
   `outputSchema` for every response.
2. **Backwards compatibility:** A client reading only `content[0]` receives
   valid JSON that can be parsed without error.
3. **PII coverage (Fix 3):** Every column present in `rows` has a
   corresponding entry in `columns` with `pii_detected`, `entity_types`,
   and `treatment` populated.
4. **Expand completeness (Fix 1):** For every tool exposed by oracle-mcp,
   `expand` lists all optional parameters with descriptions.
5. **opencode smoke test:** Connect oracle-mcp to opencode and verify tools
   appear, can be called, and that `structuredContent` is consumed without
   error by the Vercel AI SDK tool wrapper.
