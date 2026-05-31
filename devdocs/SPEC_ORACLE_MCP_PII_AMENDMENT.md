# SPEC_ORACLE_MCP_PII_AMENDMENT

> Amendment to SPEC_ORACLE_MCP_STRUCTURED_OUTPUT  
> Confirms oracle-mcp code changes required for PII encryption/resolution support.

## Summary

**oracle-mcp requires zero code changes** to support AES-SIV PII token encryption and
resolution. This is by design: the framework absorbs both concerns entirely.

## How It Works End-to-End

```
LLM issues tool call:
  execute_query(sql: "SELECT * FROM CONTACTS WHERE CONT_SURNAME = 'pii:a3f8c2...'")

mcp-framework Initialize() closure:
  1. PIIPipeline.Resolve(args)
       → detects "pii:a3f8c2..." prefix
       → decrypts with ORACLE_PII_HMAC_KEY → "Bowditch"
       → args now: { sql: "SELECT * FROM CONTACTS WHERE CONT_SURNAME = 'Bowditch'" }
  2. ExecuteReadTool.Handle(ctx, args) — receives plaintext, executes normally
  3. PIIPipeline.Process(result)
       → detects PERSON entity in CONT_SURNAME column
       → re-encrypts "Bowditch" → "pii:a3f8c2..."
       → LLM receives token, never plaintext

Next tool call referencing the same person:
  → Same key + same plaintext → same token
  → LLM can correlate rows; cannot reverse the value
```

## oracle-mcp Configuration

The only oracle-mcp concern is ensuring the framework server is initialised with the PII
pipeline enabled and pointed at the correct key env var. This belongs in `main.go`:

```go
server := framework.NewServerWithConfig(&framework.Config{
    Name:           "oracle-mcp",
    Version:        version,
    WriteEnabled:   cfg.WriteEnabled,
    PIIScanEnabled: true,
    PIIConfig: &framework.PIIPipelineConfig{
        HMACKeyEnv:      "ORACLE_PII_HMAC_KEY",
        DefaultOperator: "pseudonymise",
        MinConfidence:   0.7,
    },
})
```

`ORACLE_PII_HMAC_KEY` must be set in the deployment environment (Kubernetes secret or local
`.env`). It is per-user: each user's mcp-bridge process injects their own key, ensuring tokens
produced for one user cannot be resolved by another user's oracle-mcp process.

## Key Length

AES-SIV requires a key that is **twice** the underlying AES key size:

| AES variant | Key bytes |
|-------------|-----------|
| AES-128-SIV | 32 bytes  |
| AES-256-SIV | 64 bytes  |

`ORACLE_PII_HMAC_KEY` must be 32 or 64 bytes. If the value is stored as a hex string in the
secret, `PseudonymiseOperator` must hex-decode it before use. Document the expected encoding
in the deployment runbook.

## Process-Affinity Non-Requirement

Because AES-SIV is deterministic and stateless, there is no lookup table and no session state.
oracle-mcp can be restarted, scaled horizontally, or replaced by a fresh mcp-bridge subprocess
at any time. Token resolution requires only the key — which is injected at startup — and
works identically across any number of processes.

## No Tests Required in oracle-mcp

All PII behaviour is tested in `go-presidio` and `mcp-framework`. oracle-mcp integration
tests should verify only that:

1. The server starts without error when `ORACLE_PII_HMAC_KEY` is set
2. The server starts without error when `ORACLE_PII_HMAC_KEY` is absent (PII pipeline
   disabled, tools behave as before)
