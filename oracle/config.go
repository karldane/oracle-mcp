package oracle

import (
	"os"
	"strconv"
	"strings"

	"github.com/karldane/mcp-framework/framework"
)

func buildPIIConfig() *framework.PIIPipelineConfig {
	if os.Getenv("ORACLE_PII_HMAC_KEY") == "" {
		return nil
	}

	if os.Getenv("PRESIDIO_HMAC_KEY") == "" {
		os.Setenv("PRESIDIO_HMAC_KEY", os.Getenv("ORACLE_PII_HMAC_KEY"))
	}

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
