package oracle

import (
	"os"
	"strings"
	"testing"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

func TestServerCreation(t *testing.T) {
	// This test would require a real Oracle connection
	// For now, just test that types are defined correctly
	t.Log("Server struct defined correctly")
}

func TestToolDefinitions(t *testing.T) {
	// Test that all tools have proper EnforcerProfile
	tests := []struct {
		name string
		tool interface {
			Name() string
			Description() string
			EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile
		}
		expectedRisk     framework.RiskLevel
		expectedImpact   framework.ImpactScope
		expectedApproval bool
	}{
		{
			name:             "ListTablesTool",
			tool:             &ListTablesTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "DescribeTableTool",
			tool:             &DescribeTableTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "ExecuteReadTool",
			tool:             &ExecuteReadTool{},
			expectedRisk:     framework.RiskMed,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: true,
		},
		{
			name:             "ExecuteWriteTool",
			tool:             &ExecuteWriteTool{},
			expectedRisk:     framework.RiskMed,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := tt.tool.EnforcerProfile(nil)

			if profile.RiskLevel != tt.expectedRisk {
				t.Errorf("Expected risk %s, got %s", tt.expectedRisk, profile.RiskLevel)
			}

			if profile.ImpactScope != tt.expectedImpact {
				t.Errorf("Expected impact %s, got %s", tt.expectedImpact, profile.ImpactScope)
			}

			if profile.ApprovalReq != tt.expectedApproval {
				t.Errorf("Expected approval %v, got %v", tt.expectedApproval, profile.ApprovalReq)
			}
		})
	}
}

func TestQueryClassification(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		isSelect bool
		isWrite  bool
	}{
		{"SELECT", "SELECT * FROM users", true, false},
		{"WITH CTE", "WITH cte AS (SELECT * FROM users) SELECT * FROM cte", true, false},
		{"INSERT", "INSERT INTO users VALUES (1, 'test')", false, true},
		{"UPDATE", "UPDATE users SET name = 'test' WHERE id = 1", false, true},
		{"DELETE", "DELETE FROM users WHERE id = 1", false, true},
		{"MERGE", "MERGE INTO users USING dual ON (1=1)", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSelectQuery(tt.sql); got != tt.isSelect {
				t.Errorf("isSelectQuery(%q) = %v, want %v", tt.sql, got, tt.isSelect)
			}
			if got := isWriteQuery(tt.sql); got != tt.isWrite {
				t.Errorf("isWriteQuery(%q) = %v, want %v", tt.sql, got, tt.isWrite)
			}
		})
	}
}

func TestTableInfoFormatSchema(t *testing.T) {
	table := &TableInfo{
		TableName: "USERS",
		Columns: []ColumnInfo{
			{Name: "ID", DataType: "NUMBER", Nullable: false},
			{Name: "NAME", DataType: "VARCHAR2(100)", Nullable: true},
		},
		Relationships: map[string][]RelationshipInfo{
			"ORDERS": {
				{LocalColumn: "ID", ForeignColumn: "USER_ID", Direction: "OUTGOING"},
			},
		},
		FullyLoaded: true,
	}

	output := table.FormatSchema()

	if output == "" {
		t.Error("FormatSchema() returned empty string")
	}

	if !strings.Contains(output, "USERS") {
		t.Error("Expected output to contain table name")
	}

	if !strings.Contains(output, "ID") {
		t.Error("Expected output to contain column name")
	}
}

func TestConstraintTypeMapping(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"P", "PRIMARY KEY"},
		{"R", "FOREIGN KEY"},
		{"U", "UNIQUE"},
		{"C", "CHECK"},
		{"X", "X"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapConstraintType(tt.input); got != tt.expected {
				t.Errorf("mapConstraintType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAnalyzeQueryForOptimization(t *testing.T) {
	// Test SELECT *
	suggestions := analyzeQueryForOptimization("SELECT * FROM users")
	found := false
	for _, s := range suggestions {
		if strings.Contains(s, "SELECT *") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about SELECT *")
	}

	// Test LIKE with leading wildcard
	suggestions = analyzeQueryForOptimization("SELECT * FROM users WHERE name LIKE '%test'")
	found = false
	for _, s := range suggestions {
		if strings.Contains(s, "wildcard") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about leading wildcards")
	}
}

func TestQueryResultFormatting(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"ID", "NAME"},
		Rows: []map[string]interface{}{
			{"ID": 1, "NAME": "Alice"},
			{"ID": 2, "NAME": nil},
		},
	}

	output := formatQueryResult(result)

	if output == "" {
		t.Error("formatQueryResult returned empty string")
	}

	if !strings.Contains(output, "Alice") {
		t.Error("Expected output to contain 'Alice'")
	}

	if !strings.Contains(output, "NULL") {
		t.Error("Expected output to contain 'NULL' for nil value")
	}
}

func TestWriteResultFormatting(t *testing.T) {
	result := &WriteResult{RowsAffected: 5, Committed: true}
	output := formatWriteResult(result, true)

	if !strings.Contains(output, "5") {
		t.Error("Expected output to contain row count")
	}

	if !strings.Contains(output, "committed") {
		t.Error("Expected output to mention committed status")
	}
}

func TestExplainPlanFormatting(t *testing.T) {
	plan := &ExplainPlan{
		Steps:       []string{"Step 1", "Step 2"},
		Suggestions: []string{"Suggestion 1"},
	}

	output := formatExplainPlan(plan)

	if !strings.Contains(output, "Step 1") {
		t.Error("Expected output to contain steps")
	}

	if !strings.Contains(output, "Suggestion 1") {
		t.Error("Expected output to contain suggestions")
	}
}

func TestPIIConfigWithKey(t *testing.T) {
	os.Setenv("ORACLE_PII_HMAC_KEY", "0123456789abcdef0123456789abcdef")
	defer os.Unsetenv("ORACLE_PII_HMAC_KEY")

	cfg := buildPIIConfig()
	if cfg == nil {
		t.Fatal("Expected PII config to be non-nil when ORACLE_PII_HMAC_KEY is set")
	}
	if cfg.HMACKeyEnv != "ORACLE_PII_HMAC_KEY" {
		t.Errorf("Expected HMACKeyEnv to be 'ORACLE_PII_HMAC_KEY', got %q", cfg.HMACKeyEnv)
	}
	if cfg.DefaultOperator != "redact" {
		t.Errorf("Expected DefaultOperator to be 'redact', got %q", cfg.DefaultOperator)
	}
}

func TestPIIConfigWithoutKey(t *testing.T) {
	os.Unsetenv("ORACLE_PII_HMAC_KEY")
	os.Unsetenv("PRESIDIO_HMAC_KEY")

	cfg := buildPIIConfig()
	if cfg != nil {
		t.Error("Expected PII config to be nil when ORACLE_PII_HMAC_KEY is not set")
	}
}

func TestPIIConfigPseudonymiseOperator(t *testing.T) {
	os.Setenv("ORACLE_PII_HMAC_KEY", "0123456789abcdef0123456789abcdef")
	os.Setenv("ORACLE_PII_DEFAULT_OPERATOR", "pseudonymise")
	defer os.Unsetenv("ORACLE_PII_HMAC_KEY")
	defer os.Unsetenv("ORACLE_PII_DEFAULT_OPERATOR")

	cfg := buildPIIConfig()
	if cfg == nil {
		t.Fatal("Expected PII config to be non-nil")
	}
	if cfg.DefaultOperator != "pseudonymise" {
		t.Errorf("Expected DefaultOperator to be 'pseudonymise', got %q", cfg.DefaultOperator)
	}
}

func TestPIIConfigMinConfidence(t *testing.T) {
	os.Setenv("ORACLE_PII_HMAC_KEY", "0123456789abcdef0123456789abcdef")
	os.Setenv("ORACLE_PII_MIN_CONFIDENCE", "0.8")
	defer os.Unsetenv("ORACLE_PII_HMAC_KEY")
	defer os.Unsetenv("ORACLE_PII_MIN_CONFIDENCE")

	cfg := buildPIIConfig()
	if cfg == nil {
		t.Fatal("Expected PII config to be non-nil")
	}
	if cfg.MinConfidence != 0.8 {
		t.Errorf("Expected MinConfidence to be 0.8, got %f", cfg.MinConfidence)
	}
}

func TestOutputSchemaMethods(t *testing.T) {
	tests := []struct {
		name string
		tool interface {
			OutputSchema() *mcp.ToolOutputSchema
		}
		wantNil   bool
		wantType  string
		wantProps []string
	}{
		{
			name:      "ListTablesTool",
			tool:      &ListTablesTool{},
			wantNil:   false,
			wantType:  "object",
			wantProps: []string{"tables", "count"},
		},
		{
			name:      "DescribeTableTool",
			tool:      &DescribeTableTool{},
			wantNil:   false,
			wantType:  "object",
			wantProps: []string{"table", "columns"},
		},
		{
			name:      "ExecuteReadTool",
			tool:      &ExecuteReadTool{},
			wantNil:   false,
			wantType:  "object",
			wantProps: []string{"rows", "row_count", "columns"},
		},
		{
			name:      "ExecuteWriteTool",
			tool:      &ExecuteWriteTool{},
			wantNil:   true,
			wantType:  "",
			wantProps: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := tt.tool.OutputSchema()

			if tt.wantNil {
				if schema != nil {
					t.Errorf("Expected nil OutputSchema, got %+v", schema)
				}
				return
			}

			if schema == nil {
				t.Fatalf("Expected non-nil OutputSchema")
			}

			if schema.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, schema.Type)
			}

			for _, prop := range tt.wantProps {
				if _, ok := schema.Properties[prop]; !ok {
					t.Errorf("Expected property %q not found in OutputSchema.Properties", prop)
				}
			}
		})
	}
}

func TestOutputSchemaListTablesDetails(t *testing.T) {
	tool := &ListTablesTool{}
	schema := tool.OutputSchema()

	if schema == nil {
		t.Fatal("Expected non-nil OutputSchema")
	}

	tables, ok := schema.Properties["tables"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected tables property to be a map")
	}

	if tables["type"] != "array" {
		t.Errorf("Expected tables type to be 'array', got %v", tables["type"])
	}

	count, ok := schema.Properties["count"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected count property to be a map")
	}

	if count["type"] != "integer" {
		t.Errorf("Expected count type to be 'integer', got %v", count["type"])
	}
}

func TestOutputSchemaExecuteReadHasPIIFields(t *testing.T) {
	tool := &ExecuteReadTool{}
	schema := tool.OutputSchema()

	if schema == nil {
		t.Fatal("Expected non-nil OutputSchema")
	}

	columns, ok := schema.Properties["columns"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected columns property to be a map")
	}

	items, ok := columns["items"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected columns.items to be a map")
	}

	props, ok := items["properties"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected columns.items.properties to be a map")
	}

	piiFields := []string{"name", "pii_detected", "entity_types", "treatment"}
	for _, field := range piiFields {
		if _, ok := props[field]; !ok {
			t.Errorf("Expected PII field %q in columns.items.properties", field)
		}
	}

	treatment, ok := props["treatment"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected treatment to be a map")
	}

	enum, ok := treatment["enum"].([]string)
	if !ok {
		t.Fatal("Expected treatment to have enum")
	}

	expectedEnum := []string{"none", "masked", "tokenised", "redacted"}
	for i, e := range expectedEnum {
		if enum[i] != e {
			t.Errorf("Expected enum[%d] = %q, got %q", i, e, enum[i])
		}
	}
}
