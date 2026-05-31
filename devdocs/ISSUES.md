# Oracle MCP Backend: Client Review & Issues

This document tracks findings from a critical review of the `oracle-mcp` backend from the perspective of an MCP client. The goal is to identify areas for improvement in tool design, contracts, and response quality.

---

## 1. `expand` command provides incomplete schema summaries

- **Tool**: `MCP_Bridge_oracle_expand`
- **Issue**: The summary of tools is missing optional but critical parameters. For example, `oracle_execute_read` shows `Required params: sql (string)` but omits the optional `params: object` used for secure bind parameterization and PII token resolution.
- **Impact**: A client cannot discover the full capabilities of a tool from the summary. This forces trial-and-error and makes it harder to use advanced features like parameterized queries.
- **Suggestion**: The `expand` summary should include optional parameters, perhaps marked as such, to provide a complete picture of the tool's contract.

---

## 2. Unstructured (Text) Responses for Structured Data

- **Tools**: `oracle_list_tables`, `oracle_describe_table` (and likely others)
- **Issue**: Tools that should return structured data are instead returning pre-formatted text blobs. For example, `oracle_list_tables` returns a single string with a heading and bullet points, rather than a JSON array of table names.
- **Impact**: The client is forced to write brittle text-parsing logic to extract the actual data. This is inefficient and error-prone. The primary purpose of a machine-to-machine protocol like MCP is to avoid this.
- **Suggestion**: Tools like `oracle_list_tables` should return a `structuredContent` object containing a simple array of strings (e.g., `{"tables": ["TABLE_A", "TABLE_B"]}`). Similarly, `oracle_describe_table` should return a structured object detailing columns and relationships, not a formatted report.

---

## 3. Redundant `content` field in `oracle_execute_read`

- **Tool**: `oracle_execute_read`
- **Issue**: The tool returns data in two places: the machine-readable `structuredContent` object and a human-readable `content` array. The first element of `content` is a string containing JSON, and the second is a string containing the PII report.
- **Impact**: This is confusing and redundant. A client has to ignore the `content` field and know to look for `structuredContent`. The PII report, which is valuable metadata, is also trapped in a human-readable text block instead of being part of the structured response.
- **Suggestion**:
    1. The `content` field should be deprecated or removed for this tool.
    2. The data from the PII Scan Report should be moved into the `structuredContent.columns` array, where it belongs. Each column object in that array should be populated with the `pii_detected`, `entity_types`, and `treatment` information, making the response fully machine-readable.
