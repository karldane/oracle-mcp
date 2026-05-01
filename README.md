# Oracle MCP Server

A Go-native MCP (Model Context Protocol) server for Oracle databases, providing schema introspection and query execution tools with self-reporting safety metadata.

## Overview

This server provides secure, controlled access to Oracle databases. It is a pure Go implementation that requires no external dependencies like Oracle Instant Client.

It includes a comprehensive suite of tools for schema introspection and query execution, with a strong focus on safety. Key features include read-only by default, automatic PII (Personally Identifiable Information) detection and treatment, and structured, self-reporting metadata for all operations.

**For detailed configuration, feature explanations, and examples, please see the [USAGE.md](./USAGE.md) guide.**

## Features

- **Pure Go Implementation**: Uses `go-ora` driver (no Oracle Instant Client required).
- **Advanced Schema Introspection**: Deep table/column analysis with intelligent caching.
- **Safety First**: All tools include `EnforcerProfile` metadata for automated policy enforcement.
- **Robust PII Pipeline**: Features multiple PII handling strategies (`redact`, `hash`, `pseudonymise`) and supports round-trip decryption of PII tokens for use in subsequent queries.
- **Structured Tool Output**: Returns detailed, machine-readable results including row data, column PII reports, and query metadata.

## Installation

### Prerequisites

- Go 1.22 or later
- Access to an Oracle database

### Building from Source

```bash
git clone https://github.com/karldane/oracle-mcp.git
cd oracle-mcp
make
```

This will download dependencies and build the binary. For more options (e.g., cross-compilation), see the `Makefile` or run `make help`.

## Basic Usage

To run the server, set the connection string and execute the binary:
```bash
export ORACLE_CONNECTION_STRING="oracle://user:pass@host:1521/SERVICE"
./oracle-mcp
```

**Please see [USAGE.md](./USAGE.md) for advanced configuration, including enabling write mode and configuring the PII pipeline.**

