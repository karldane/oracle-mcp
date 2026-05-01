# Usage Guide

This document provides detailed instructions for configuring and using the Oracle MCP server.

## Configuration

Configuration is managed via environment variables.

### Primary Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `ORACLE_CONNECTION_STRING` | **Required.** The full connection string for the target Oracle database. | (none) |
| `ORACLE_READ_ONLY` | If `false`, allows write operations (`INSERT`, `UPDATE`, etc.). Write mode also requires the `-write-enabled` command-line flag. | `true` |
| `CACHE_DIR` | The directory to store the database schema cache. | `.cache` |

**Connection String Format:**

The connection string must be in the following format:
`oracle://user:password@host:port/service_name`

**Example:**
```bash
export ORACLE_CONNECTION_STRING="oracle://scott:tiger@db.example.com:1521/ORCL"
```

---

### PII Handling Configuration

The server includes a powerful PII (Personally Identifiable Information) detection and treatment pipeline.

| Variable | Description | Default |
|----------|-------------|---------|
| `ORACLE_PII_HMAC_KEY` | A secret key for PII operations. **Enabling this is required for `hash` and `pseudonymise` operators.** The key must be **32 or 64 bytes**. | (none) |
| `ORACLE_PII_DEFAULT_OPERATOR` | The default action to take on detected PII. See operators below. | `redact` |
| `ORACLE_PII_MIN_CONFIDENCE` | The minimum confidence score (0.0 to 1.0) for a PII entity to be detected. | `0.5` |
| `ORACLE_PII_OP_<ENTITY>` | Sets a specific operator for a given entity type, overriding the default. | (none) |

#### PII Operators

- **`redact`** (Default): Replaces detected PII with a placeholder (e.g., `<redacted: EMAIL_ADDRESS>`).
- **`hash`**: Replaces PII with a one-way HMAC-SHA256 hash. Requires `ORACLE_PII_HMAC_KEY`.
- **`pseudonymise`**: Replaces PII with a reversible, encrypted token (prefixed with `pii:`). This allows the value to be used in subsequent queries. Requires a **32 or 64-byte** `ORACLE_PII_HMAC_KEY`.

#### Generating a Secure Key

Use `openssl` to generate a secure, 64-byte key suitable for the `pseudonymise` operator.

```bash
# This command generates a 64-character hex string, which the server reads as a 64-byte key.
openssl rand -hex 32
```

#### Per-Entity Configuration Example

You can set different operators for different PII types. For example, to pseudonymise emails but hash phone numbers:

```bash
export ORACLE_PII_DEFAULT_OPERATOR="redact"
export ORACLE_PII_OP_EMAIL_ADDRESS="pseudonymise"
export ORACLE_PII_OP_PHONE_NUMBER="hash"
```

---

## Running the Server

### Read-Only Mode (Default)

```bash
export ORACLE_CONNECTION_STRING="<your_connection_string>"
./oracle-mcp
```

### Write-Enabled Mode

To enable write operations, you must set **both** the environment variable and the command-line flag.

```bash
export ORACLE_CONNECTION_STRING="<your_connection_string>"
export ORACLE_READ_ONLY=false
./oracle-mcp -write-enabled
```

---

## Tool Usage and Examples

### `oracle_execute_read`

Executes a `SELECT` query.

**Example: Simple Query**
```json
{
  "tool": "oracle_execute_read",
  "params": {
    "sql": "SELECT * FROM EMPLOYEES WHERE ROWNUM <= 5"
  }
}
```

**Example: Parameterized Queries**

To prevent SQL injection and safely handle PII tokens, use named bind parameters in your SQL and pass the values in the `params` object.

```json
{
  "tool": "oracle_execute_read",
  "params": {
    "sql": "SELECT * FROM EMPLOYEES WHERE LAST_NAME = :surname AND DEPARTMENT_ID = :dept",
    "params": {
      "surname": "Smith",
      "dept": 101
    }
  }
}
```

### PII Round-Trip Example

This demonstrates querying data, receiving an encrypted PII token, and using that token in a subsequent query.

**1. Run a query to get encrypted data.**

*Request:*
```json
{
  "tool": "oracle_execute_read",
  "params": {
    "sql": "SELECT CONT_ID, CONT_EMAIL FROM CONTACTS WHERE CONT_ID = 1006"
  }
}
```

*Response:*
The `CONT_EMAIL` is returned as a reversible `pii:` token.
```json
{
  "rows": [
    {
      "CONT_EMAIL": "pii:8bf486bcfed6246491e2cded3d524bb0ebe7...",
      "CONT_ID": "1006"
    }
  ]
}
```

**2. Use the PII token in a subsequent query.**

Pass the token from the previous step into the `params` map. The server will decrypt it before executing the query.

*Request:*
```json
{
  "tool": "oracle_execute_read",
  "params": {
    "sql": "SELECT CONT_ID, CONT_EMAIL FROM CONTACTS WHERE CONT_EMAIL = :email_addr",
    "params": {
      "email_addr": "pii:8bf486bcfed6246491e2cded3d524bb0ebe7..."
    }
  }
}
```

*Response:*
The query succeeds, returning the correct row. The PII in the final output is re-encrypted.
```json
{
  "rows": [
    {
      "CONT_EMAIL": "pii:8bf486bcfed6246491e2cded3d524bb0ebe7...",
      "CONT_ID": "1006"
    }
  ]
}
```

### Structured Output

Query tools now return a `structuredContent` object containing detailed results and metadata.

```json
{
  "structuredContent": {
    "rows": [
      { "COLUMN_A": "value1", "COLUMN_B": 123 },
      { "COLUMN_A": "value2", "COLUMN_B": 456 }
    ],
    "columns": [
      {
        "name": "COLUMN_A",
        "data_type": "VARCHAR2",
        "pii_detected": true,
        "entity_types": ["PERSON"],
        "treatment": "pseudonymise"
      },
      {
        "name": "COLUMN_B",
        "data_type": "NUMBER",
        "pii_detected": false
      }
    ],
    "meta": {
      "row_count": 2,
      "pii_scan_applied": true
    }
  }
}
```
