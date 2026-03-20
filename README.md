# Oracle MCP Server

A Go-native MCP (Model Context Protocol) server for Oracle databases, providing schema introspection and query execution tools with self-reporting safety metadata.

## Overview

This server provides secure, controlled access to Oracle databases with the following features:

- **Pure Go Implementation**: Uses `go-ora` driver (no Oracle Instant Client required)
- **Schema Introspection**: Deep table/column analysis with intelligent caching
- **Safety First**: All tools include `EnforcerProfile` metadata for automated policy enforcement
- **Read-Only by Default**: Write operations require explicit opt-in via environment variable and command-line flag
- **Transactional Safety**: Write operations use transactions with rollback-by-default protection

## Tools

### Schema Introspection (Read-Only)

| Tool | Description | Risk | Impact | Approval |
|------|-------------|------|--------|----------|
| `oracle_list_tables` | List all tables in the database | Low | Read | No |
| `oracle_describe_table` | Get table schema (columns, relationships) | Low | Read | No |
| `oracle_search_tables` | Search tables by name pattern | Low | Read | No |
| `oracle_search_columns` | Search columns across all tables | Low | Read | No |
| `oracle_get_constraints` | Get PK/FK/UNIQUE/CHECK constraints | Low | Read | No |
| `oracle_get_indexes` | Get table indexes | Low | Read | No |
| `oracle_get_related_tables` | Get FK relationships | Low | Read | No |
| `oracle_explain_query` | Get query execution plan | Low | Read | No |

### Query Execution

| Tool | Description | Risk | Impact | Approval |
|------|-------------|------|--------|----------|
| `oracle_execute_read` | Execute SELECT queries (100 row limit) | Med | Read | Yes |
| `oracle_execute_write` | Execute INSERT/UPDATE/DELETE | High | Write | Yes |

## Installation

### Prerequisites

- Go 1.22 or later
- Access to an Oracle database (12c/19c/21c/23ai)

### Building from Source

```bash
git clone https://github.com/karldane/oracle-mcp.git
cd oracle-mcp
make
```

This will automatically download dependencies and build a stripped binary.

#### Build Options

```bash
make              # Download deps and build (default)
make deps         # Download dependencies only
make build        # Build binary only (assumes deps exist)
make build-all    # Build for Linux, macOS, and Windows
make test         # Run tests
make clean        # Remove build artifacts
make install      # Install to GOPATH/bin
make help         # Show all options
```

### Download Binary

Pre-built binaries are available in the releases section.

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `ORACLE_CONNECTION_STRING` | Oracle connection string (required) | - |
| `ORACLE_READ_ONLY` | Enable read-only mode | `true` |
| `CACHE_DIR` | Schema cache directory | `.cache` |

### Connection String Format

```
oracle://user:password@host:port/service_name
```

Examples:
```bash
# Basic connection
export ORACLE_CONNECTION_STRING="oracle://scott:tiger@localhost:1521/ORCL"

# With service name
export ORACLE_CONNECTION_STRING="oracle://user:pass@db.example.com:1521/XEPDB1"
```

## Usage

### Basic Usage (Read-Only)

```bash
export ORACLE_CONNECTION_STRING="oracle://user:pass@host:1521/SERVICE"
./oracle-mcp
```

### Enable Write Operations

Write operations require TWO conditions:
1. `ORACLE_READ_ONLY=false` environment variable
2. `-write-enabled` command-line flag

```bash
export ORACLE_CONNECTION_STRING="oracle://user:pass@host:1521/SERVICE"
export ORACLE_READ_ONLY=false
./oracle-mcp -write-enabled
```

### Self-Reporting Mode (No Database Connection)

The server can start without a database connection to report tool metadata:

```bash
./oracle-mcp
# Server starts in disconnected mode
# Tools are registered but require ORACLE_CONNECTION_STRING to execute
```

## Safety Features

### EnforcerProfile Metadata

Every tool reports its safety characteristics via `EnforcerProfile`:

```go
type EnforcerProfile struct {
    RiskLevel    RiskLevel   // low, med, high, critical
    ImpactScope  ImpactScope // read, write, delete, admin
    ResourceCost int         // 1-10 (CPU/Memory weight)
    PIIExposure  bool        // Returns sensitive data?
    Idempotent   bool        // Safe to retry?
    ApprovalReq  bool        // Requires human approval?
}
```

This metadata is transmitted to the MCP Bridge during the `tools/list` handshake, enabling automated policy enforcement.

### Query Classification

The server automatically classifies SQL:

- **SELECT/WITH**: Allowed via `oracle_execute_read`
- **INSERT/UPDATE/DELETE/MERGE**: Require `oracle_execute_write` + write-enabled flag
- **DDL (CREATE/ALTER/DROP)**: Blocked in read-only mode

### Row Limiting

All SELECT queries are automatically limited to prevent resource exhaustion:
- Default: 100 rows
- Maximum: 1,000 rows
- Can be overridden with `max_rows` parameter

### Transaction Safety

Write operations use transactions:
- **Default**: Rollback (dry-run mode)
- **Commit**: Only when `commit=true` parameter is set

## Architecture

### Schema Caching

- Tables and columns are cached on startup
- Lazy loading: Full table details loaded on first access
- Cache persisted to disk (`.cache/{schema}.json`)
- Rebuild with `rebuild_schema_cache` tool

### Database Connection

- Uses `go-ora` pure Go driver
- Connection pooling via `database/sql`
- Configurable read-only mode at connection level

## Testing

Run the test suite:

```bash
go test ./oracle -v
```

Tests cover:
- EnforcerProfile metadata accuracy
- Schema introspection and caching
- Query classification and injection prevention
- Transactional integrity
- Read-only mode enforcement

## License

This project is licensed under the Functional Source License, Version 1.1, ALv2 Future License.

Copyright 2026 Karl Dane

See LICENSE file for full terms.

## References

- [MCP Framework](https://github.com/karldane/mcp-framework) - Base framework with EnforcerProfile support
- [go-ora](https://github.com/sijms/go-ora) - Pure Go Oracle driver
