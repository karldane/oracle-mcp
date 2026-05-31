# Oracle-MCP PII Integration Plan

## Learnings from Requirements Gathering

### Current State
- **Framework Migration**: Completed in oracle/oracle.go - all tools now use v2.0.0 interface
- **ToolResult Already Present**: Codebase already uses framework.TextResult() helper
- **Schema Cache Exists**: ColumnInfo has Name, DataType, Nullable - extended with ScanPolicy/MaxScanLength
- **Database Layer Separated**: Connection handling in database.go, models in models.go

### Implementation Done

#### Phase 1: Framework Migration ✅
- All ~10 tools migrated in oracle.go
- Handle(ctx context.Context) → Handle(ctx framework.CallContext)
- GetEnforcerProfile() → EnforcerProfile(args map[string]interface{})
- Return framework.ToolResult via framework.TextResult()
- ExecuteWriteTool.EnforcerProfile now dynamic (checks commit arg)

#### Phase 2: Schema Extension ✅
- Added ScanPolicy (int) and MaxScanLength (int) to ColumnInfo in models.go
- Added ScanPolicy constants matching presidio package
- Created oracle/scanpolicy.go with type classification:
  - isNumericType, isBooleanType, isIntervalType, isBinaryType
  - isLargeTextType, isDateType, isTextType, isStructuredType
  - scanPolicyForColumn() returns (policy, maxLength)
- Updated loadTableDetails() in database.go to call scanPolicyForColumn at column access time

#### Phase 3: ColumnHint Builder ✅
- Created oracle/hints.go with BuildColumnHints()

#### Phase 4: Pipeline Helper ✅
- Created oracle/pipeline.go with derivePipelineKey() for session-keyed HMAC

### Files Modified/Created
| File | Change | Status |
|---|---|---|
| oracle/oracle.go | Framework migration | ✅ |
| oracle/models.go | Add ScanPolicy, MaxScanLength | ✅ |
| oracle/scanpolicy.go | New - Type classification | ✅ |
| oracle/database.go | Call scanPolicyForColumn | ✅ |
| oracle/hints.go | New - BuildColumnHints | ✅ |
| oracle/pipeline.go | New - derivePipelineKey | ✅ |

### Next Steps (Not Started)
- None - All phases complete

### Completed (Post-Migration)
- Phase 5: Tier 3 DataResult + ColumnHints ✅
  - ExecuteReadTool returns Data + ColumnHints from schema cache
  - ExecuteWriteTool returns RawText + Data + ColumnHints
  - Added buildHintsFromQuery, buildHintsFromWriteQuery
  - Added extractTableName, extractWriteTableName
- Phase 6: Tests migrated to new interface ✅
  - GetEnforcerProfile() → EnforcerProfile(args)
  - context.Context → framework.Background()
  - result string → result.RawText
  - result.Data for checking query results
- Test coverage: 80.4%
- 5 commits total

### MCP Bridge Testing
- Stdio: ✅ WORKS - Returns correct MCP format: `{"content":[{"type":"text","text":"..."}]}`
- HTTP (via mcp-bridge): ❌ Returns empty response
- Error: "Streamable HTTP error: Error POSTing to endpoint: Empty response from backend"
- Debug: mcp-bridge passes initialization (tools list works), fails on tool call
- Similar to git-lsp-mcp content-length issue but output side
- qdrant-mcp works (mark3labs/mcp-go v0.46.0) vs oracle-mcp (v0.45.0)