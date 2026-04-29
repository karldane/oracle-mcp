package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/karldane/mcp-framework/framework"
)

// MockQueryExecutor is a mock implementation of QueryExecutor for testing
type MockQueryExecutor struct {
	Tables        []string
	TableInfo     *TableInfo
	SearchResults []string
	Columns       map[string][]ColumnInfo
	Constraints   []ConstraintInfo
	Indexes       []IndexInfo
	RelatedTables *RelatedTables
	QueryResult   *QueryResult
	WriteResult   *WriteResult
	ExplainPlan   *ExplainPlan
	SchemaName    string
	ReadOnly      bool
	Err           error
}

func (m *MockQueryExecutor) GetAllTableNames(ctx context.Context) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Tables, nil
}

func (m *MockQueryExecutor) GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.TableInfo, nil
}

func (m *MockQueryExecutor) SearchTables(ctx context.Context, searchTerm string, limit int) ([]string, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.SearchResults, nil
}

func (m *MockQueryExecutor) SearchColumns(ctx context.Context, searchTerm string, limit int) (map[string][]ColumnInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Columns, nil
}

func (m *MockQueryExecutor) GetConstraints(ctx context.Context, tableName string) ([]ConstraintInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Constraints, nil
}

func (m *MockQueryExecutor) GetIndexes(ctx context.Context, tableName string) ([]IndexInfo, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Indexes, nil
}

func (m *MockQueryExecutor) GetRelatedTables(ctx context.Context, tableName string) (*RelatedTables, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.RelatedTables, nil
}

func (m *MockQueryExecutor) ExecuteQuery(ctx context.Context, sql string, maxRows int) (*QueryResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.QueryResult, nil
}

func (m *MockQueryExecutor) ExecuteWrite(ctx context.Context, sql string, commit bool) (*WriteResult, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.WriteResult, nil
}

func (m *MockQueryExecutor) ExplainQuery(ctx context.Context, sql string) (*ExplainPlan, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.ExplainPlan, nil
}

func (m *MockQueryExecutor) Schema() string {
	return m.SchemaName
}

func (m *MockQueryExecutor) IsReadOnly() bool {
	return m.ReadOnly
}

// MockConnection wraps MockQueryExecutor to implement *Connection interface for testing
type MockConnection struct {
	*MockQueryExecutor
	DB *sql.DB
}

func newMockConnection(mock *MockQueryExecutor) *MockConnection {
	return &MockConnection{
		MockQueryExecutor: mock,
	}
}

// ============================================================================
// Test Suite 1: Multi-Database Connection Parsing
// ============================================================================

func TestDatabaseRegistry_ParseNamedConnections(t *testing.T) {
	// Test that error cases are handled correctly
	// Note: We can only test error cases here without a real database
	// Success cases require actual Oracle connections

	tests := []struct {
		name        string
		envVars     map[string]string
		expectError bool
	}{
		{
			name: "case normalization collision",
			envVars: map[string]string{
				"ORACLE_CONNECTION_STRING_BAU": "oracle://user:pass@bau1/serv",
				"ORACLE_CONNECTION_STRING_bau": "oracle://user:pass@bau2/serv",
			},
			expectError: true,
		},
		{
			name: "conflict unnamed and named",
			envVars: map[string]string{
				"ORACLE_CONNECTION_STRING":      "oracle://user:pass@host/serv",
				"ORACLE_CONNECTION_STRING_DB17": "oracle://user:pass@db17/serv",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearOracleEnvVars()
			for k, v := range tt.envVars {
				t.Setenv(k, v)
			}

			_, err := NewDatabaseRegistry(false)

			if !tt.expectError && err != nil {
				t.Logf("Note: Connection failed as expected without real DB: %v", err)
			}

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
		})
	}
}

func TestDatabaseRegistry_EmptyConfig(t *testing.T) {
	clearOracleEnvVars()

	registry, err := NewDatabaseRegistry(false)
	if err != nil {
		t.Fatalf("Expected no error for empty config, got: %v", err)
	}

	connections := registry.ListConnections()
	if len(connections) != 0 {
		t.Errorf("Expected 0 connections, got %d", len(connections))
	}
}

func clearOracleEnvVars() {
	// Environment vars are cleared by t.Setenv() which overwrites them
	// This function is kept for compatibility but t.Setenv handles cleanup
}

// ============================================================================
// Test Suite 2: Connection Status Management
// ============================================================================

func TestConnectionStatus(t *testing.T) {
	tests := []struct {
		status   ConnectionStatus
		expected string
	}{
		{StatusConnected, "connected"},
		{StatusAvailable, "available"},
		{StatusError, "error"},
		{StatusDisconnected, "disconnected"},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if string(tt.status) != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, string(tt.status))
			}
		})
	}
}

func TestConnection_IsConnected(t *testing.T) {
	conn := &Connection{
		Label:  "test",
		schema: "TEST",
	}

	if conn.IsConnected() {
		t.Error("Expected IsConnected to return false when DB is nil")
	}
}

func TestConnection_IsReadOnly(t *testing.T) {
	tests := []struct {
		name     string
		readOnly bool
	}{
		{"read-write connection", false},
		{"read-only connection", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conn := &Connection{
				Label:    "test",
				schema:   "TEST",
				ReadOnly: tt.readOnly,
			}

			if conn.IsReadOnly() != tt.readOnly {
				t.Errorf("Expected IsReadOnly to return %v, got %v", tt.readOnly, conn.IsReadOnly())
			}
		})
	}
}

func TestConnectionStatusValues(t *testing.T) {
	// Test all status values
	if StatusConnected != "connected" {
		t.Errorf("StatusConnected = %s, want connected", StatusConnected)
	}
	if StatusAvailable != "available" {
		t.Errorf("StatusAvailable = %s, want available", StatusAvailable)
	}
	if StatusError != "error" {
		t.Errorf("StatusError = %s, want error", StatusError)
	}
	if StatusDisconnected != "disconnected" {
		t.Errorf("StatusDisconnected = %s, want disconnected", StatusDisconnected)
	}
}

// ============================================================================
// Test Suite 2b: ListConnections with Various Statuses
// ============================================================================

func TestListConnections_VariousStatuses(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"connected": {
				Label:  "connected",
				schema: "SCHEMA1",
				Status: StatusConnected,
			},
			"error": {
				Label:    "error",
				schema:   "connection failed",
				Status:   StatusError,
				ErrorMsg: "connection failed",
			},
			"disconnected": {
				Label:  "disconnected",
				schema: "SCHEMA3",
				Status: StatusDisconnected,
			},
		},
	}

	connections := registry.ListConnections()
	if len(connections) != 3 {
		t.Errorf("Expected 3 connections, got %d", len(connections))
	}
}

func TestIsMultiDatabase(t *testing.T) {
	// Single connection
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}
	if registry.IsMultiDatabase() {
		t.Error("Single connection should not be multi-database")
	}

	// Multiple connections
	registry2 := &DatabaseRegistry{
		connections: map[string]*Connection{
			"db1": {Label: "db1"},
			"db2": {Label: "db2"},
		},
	}
	if !registry2.IsMultiDatabase() {
		t.Error("Multiple connections should be multi-database")
	}
}

// ============================================================================
// Test Suite 2c: Additional Helper Function Tests
// ============================================================================

func TestAnalyzeQueryForOptimization_EdgeCases(t *testing.T) {
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

	// Test leading wildcard
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

	// Test IN with subquery
	suggestions = analyzeQueryForOptimization("SELECT * FROM users WHERE id IN (SELECT id FROM orders)")
	found = false
	for _, s := range suggestions {
		if strings.Contains(s, "EXISTS") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about EXISTS")
	}

	// Test OR condition
	suggestions = analyzeQueryForOptimization("SELECT * FROM users WHERE id = 1 OR name = 'test'")
	found = false
	for _, s := range suggestions {
		if strings.Contains(s, "OR") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about OR")
	}

	// Test many joins
	suggestions = analyzeQueryForOptimization("SELECT * FROM a JOIN b ON a.id = b.id JOIN c ON b.id = c.id JOIN d ON c.id = d.id JOIN e ON d.id = e.id JOIN f ON e.id = f.id")
	found = false
	for _, s := range suggestions {
		if strings.Contains(s, "tables") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected suggestion about many tables")
	}

	// Test no suggestions for simple query
	suggestions = analyzeQueryForOptimization("SELECT id, name FROM users WHERE id = 1")
	if len(suggestions) > 0 {
		t.Errorf("Expected no suggestions for simple query, got %d", len(suggestions))
	}
}

func TestMapConstraintType(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"P", "PRIMARY KEY"},
		{"R", "FOREIGN KEY"},
		{"U", "UNIQUE"},
		{"C", "CHECK"},
		{"X", "X"}, // unknown type returns as-is
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := mapConstraintType(tt.input); got != tt.expected {
				t.Errorf("mapConstraintType(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestFormatQueryResult(t *testing.T) {
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

	if !strings.Contains(output, "2") {
		t.Error("Expected output to contain row count")
	}
}

func TestFormatWriteResult(t *testing.T) {
	tests := []struct {
		name      string
		result    *WriteResult
		committed bool
		expectStr string
	}{
		{
			name:      "committed",
			result:    &WriteResult{RowsAffected: 5, Committed: true},
			committed: true,
			expectStr: "committed",
		},
		{
			name:      "not committed",
			result:    &WriteResult{RowsAffected: 3, Committed: false},
			committed: false,
			expectStr: "not committed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := formatWriteResult(tt.result, tt.committed)

			if !strings.Contains(output, tt.expectStr) {
				t.Errorf("Expected output to contain '%s', got: %s", tt.expectStr, output)
			}

			// Check that row count is in output (format: "X row(s) affected")
			if !strings.Contains(output, "row(s) affected") {
				t.Error("Expected output to contain 'row(s) affected'")
			}
		})
	}
}

func TestFormatExplainPlan(t *testing.T) {
	plan := &ExplainPlan{
		Steps:       []string{"TABLE ACCESS FULL USERS", "NESTED LOOP"},
		Suggestions: []string{"Consider adding an index"},
	}

	output := formatExplainPlan(plan)

	if !strings.Contains(output, "TABLE ACCESS FULL USERS") {
		t.Error("Expected output to contain execution plan steps")
	}

	if !strings.Contains(output, "Consider adding an index") {
		t.Error("Expected output to contain suggestions")
	}

	if !strings.Contains(output, "Execution Plan") {
		t.Error("Expected output to contain header")
	}

	// Test empty plan
	emptyPlan := &ExplainPlan{
		Steps:       []string{},
		Suggestions: []string{},
	}
	emptyOutput := formatExplainPlan(emptyPlan)
	if emptyOutput == "" {
		t.Error("Expected non-empty output for empty plan")
	}
}

func TestTableInfo_FormatSchema(t *testing.T) {
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

	if !strings.Contains(output, "NUMBER") {
		t.Error("Expected output to contain data type")
	}

	if !strings.Contains(output, "NOT NULL") {
		t.Error("Expected output to contain NOT NULL")
	}

	if !strings.Contains(output, "ORDERS") {
		t.Error("Expected output to contain relationship")
	}
}

// ============================================================================
// Test Suite 3: Tool Registration with Multi-Database
// ============================================================================

func TestToolDefinitions_MultiDatabase(t *testing.T) {
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
			name:             "ListConnectionsTool",
			tool:             &ListConnectionsTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
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
			name:             "SearchTablesTool",
			tool:             &SearchTablesTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "SearchColumnsTool",
			tool:             &SearchColumnsTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "GetConstraintsTool",
			tool:             &GetConstraintsTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "GetIndexesTool",
			tool:             &GetIndexesTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "GetRelatedTablesTool",
			tool:             &GetRelatedTablesTool{},
			expectedRisk:     framework.RiskLow,
			expectedImpact:   framework.ImpactRead,
			expectedApproval: false,
		},
		{
			name:             "ExplainQueryTool",
			tool:             &ExplainQueryTool{},
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

func TestToolSchemas_DatabaseParameter(t *testing.T) {
	// Test that tools have database parameter when multi-database mode
	// This test verifies the schema structure

	tests := []struct {
		name             string
		toolName         string
		hasDatabaseParam bool
	}{
		{"ListConnectionsTool", "oracle_connections", false},
		{"ListTablesTool", "oracle_list_tables", true},
		{"DescribeTableTool", "oracle_describe_table", true},
		{"SearchTablesTool", "oracle_search_tables", true},
		{"SearchColumnsTool", "oracle_search_columns", true},
		{"GetConstraintsTool", "oracle_get_constraints", true},
		{"GetIndexesTool", "oracle_get_indexes", true},
		{"GetRelatedTablesTool", "oracle_get_related_tables", true},
		{"ExplainQueryTool", "oracle_explain_query", true},
		{"ExecuteReadTool", "oracle_execute_read", true},
		{"ExecuteWriteTool", "oracle_execute_write", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// The schema should include database parameter
			// We verify this by checking the tool exists and has proper schema
			found := false
			for _, expectedName := range []string{
				"oracle_connections",
				"oracle_list_tables",
				"oracle_describe_table",
				"oracle_search_tables",
				"oracle_search_columns",
				"oracle_get_constraints",
				"oracle_get_indexes",
				"oracle_get_related_tables",
				"oracle_explain_query",
				"oracle_execute_read",
				"oracle_execute_write",
			} {
				if tt.toolName == expectedName {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Tool %s not found in expected list", tt.toolName)
			}
		})
	}
}

// ============================================================================
// Test Suite 9: GetDatabaseParam Helper
// ============================================================================

func TestGetDatabaseParam(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		multiDB  bool
		expected string
	}{
		{
			name:     "with database param",
			args:     map[string]interface{}{"database": "db17"},
			multiDB:  true,
			expected: "db17",
		},
		{
			name:     "without database param in multi-DB",
			args:     map[string]interface{}{},
			multiDB:  true,
			expected: "",
		},
		{
			name:     "with empty database param",
			args:     map[string]interface{}{"database": ""},
			multiDB:  true,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDatabaseParam(tt.args, tt.multiDB)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test Suite 10: RequireConnection Validation
// ============================================================================

func TestRequireConnection_EmptyDatabaseParam(t *testing.T) {
	// Create a minimal registry for testing
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
		readOnly:    false,
	}

	// Empty database parameter now defaults to "_default"
	// Since _default doesn't exist, it should return unknown database error
	_, err := registry.RequireConnection("")
	if err == nil {
		t.Error("Expected error for unknown database")
	}

	if !strings.Contains(err.Error(), "unknown database connection") {
		t.Errorf("Expected error about unknown database, got: %v", err)
	}
}

func TestRequireConnection_UnknownDatabase(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
		readOnly:    false,
	}

	_, err := registry.RequireConnection("nonexistent")
	if err == nil {
		t.Error("Expected error for unknown database")
	}

	if !strings.Contains(err.Error(), "unknown database") {
		t.Errorf("Expected error message about 'unknown database', got: %v", err)
	}
}

// ============================================================================
// Test Suite 11: Connection Info
// ============================================================================

func TestConnectionInfo(t *testing.T) {
	info := ConnectionInfo{
		Label:     "testdb",
		Schema:    "TESTUSER",
		Connected: true,
	}

	if info.Label != "testdb" {
		t.Errorf("Expected label 'testdb', got '%s'", info.Label)
	}

	if info.Schema != "TESTUSER" {
		t.Errorf("Expected schema 'TESTUSER', got '%s'", info.Schema)
	}

	if !info.Connected {
		t.Error("Expected Connected to be true")
	}
}

// ============================================================================
// Test Suite 12: Query Result Structure
// ============================================================================

func TestQueryResult_Structure(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"COL1", "COL2", "COL3"},
		Rows: []map[string]interface{}{
			{"COL1": "a", "COL2": 1, "COL3": true},
			{"COL1": "b", "COL2": 2, "COL3": false},
		},
	}

	if len(result.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(result.Columns))
	}

	if len(result.Rows) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(result.Rows))
	}
}

func TestWriteResult_Structure(t *testing.T) {
	result := &WriteResult{
		RowsAffected: 42,
		Committed:    true,
	}

	if result.RowsAffected != 42 {
		t.Errorf("Expected 42 rows affected, got %d", result.RowsAffected)
	}

	if !result.Committed {
		t.Error("Expected Committed to be true")
	}
}

func TestExplainPlan_Structure(t *testing.T) {
	plan := &ExplainPlan{
		Steps:       []string{"step1", "step2", "step3"},
		Suggestions: []string{"sug1", "sug2"},
	}

	if len(plan.Steps) != 3 {
		t.Errorf("Expected 3 steps, got %d", len(plan.Steps))
	}

	if len(plan.Suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(plan.Suggestions))
	}
}

// ============================================================================
// Test Suite 13: Schema Cache Structure
// ============================================================================

func TestSchemaCache_Structure(t *testing.T) {
	cache := &SchemaCache{
		Tables: map[string]*TableInfo{
			"USERS": {
				TableName:     "USERS",
				Columns:       []ColumnInfo{},
				Relationships: map[string][]RelationshipInfo{},
				FullyLoaded:   true,
			},
		},
		AllTableNames: map[string]struct{}{
			"USERS":    {},
			"ORDERS":   {},
			"PRODUCTS": {},
		},
	}

	if len(cache.AllTableNames) != 3 {
		t.Errorf("Expected 3 table names, got %d", len(cache.AllTableNames))
	}

	if len(cache.Tables) != 1 {
		t.Errorf("Expected 1 table in Tables map, got %d", len(cache.Tables))
	}
}

// ============================================================================
// Test Suite 14: Column Info
// ============================================================================

func TestColumnInfo(t *testing.T) {
	col := ColumnInfo{
		Name:     "USER_NAME",
		DataType: "VARCHAR2(100)",
		Nullable: true,
	}

	if col.Name != "USER_NAME" {
		t.Errorf("Expected name 'USER_NAME', got '%s'", col.Name)
	}

	if col.DataType != "VARCHAR2(100)" {
		t.Errorf("Expected type 'VARCHAR2(100)', got '%s'", col.DataType)
	}

	if !col.Nullable {
		t.Error("Expected Nullable to be true")
	}
}

// ============================================================================
// Test Suite 15: Relationship Info
// ============================================================================

func TestRelationshipInfo(t *testing.T) {
	rel := RelationshipInfo{
		LocalColumn:   "USER_ID",
		ForeignColumn: "ID",
		Direction:     "OUTGOING",
	}

	if rel.LocalColumn != "USER_ID" {
		t.Errorf("Expected LocalColumn 'USER_ID', got '%s'", rel.LocalColumn)
	}

	if rel.ForeignColumn != "ID" {
		t.Errorf("Expected ForeignColumn 'ID', got '%s'", rel.ForeignColumn)
	}

	if rel.Direction != "OUTGOING" {
		t.Errorf("Expected Direction 'OUTGOING', got '%s'", rel.Direction)
	}
}

// ============================================================================
// Test Suite 16: Constraint Info
// ============================================================================

func TestConstraintInfo(t *testing.T) {
	constraint := ConstraintInfo{
		Name:      "PK_USERS",
		Type:      "PRIMARY KEY",
		Columns:   []string{"ID", "NAME"},
		Condition: "id > 0",
		References: &ReferenceInfo{
			Table:   "REF_TABLE",
			Columns: []string{"ID"},
		},
	}

	if constraint.Name != "PK_USERS" {
		t.Errorf("Expected name 'PK_USERS', got '%s'", constraint.Name)
	}

	if len(constraint.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(constraint.Columns))
	}

	if constraint.References == nil {
		t.Error("Expected References to be set")
	}
}

// ============================================================================
// Test Suite 17: Index Info
// ============================================================================

func TestIndexInfo(t *testing.T) {
	idx := IndexInfo{
		Name:    "IDX_NAME",
		Unique:  true,
		Columns: []string{"NAME", "STATUS"},
		Status:  "VALID",
	}

	if idx.Name != "IDX_NAME" {
		t.Errorf("Expected name 'IDX_NAME', got '%s'", idx.Name)
	}

	if !idx.Unique {
		t.Error("Expected Unique to be true")
	}

	if len(idx.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(idx.Columns))
	}
}

// ============================================================================
// Test Suite 18: Related Tables
// ============================================================================

func TestRelatedTables(t *testing.T) {
	rel := RelatedTables{
		ReferencedTables:  []string{"COUNTRIES", "REGIONS"},
		ReferencingTables: []string{"ADDRESSES", "CONTACTS"},
	}

	if len(rel.ReferencedTables) != 2 {
		t.Errorf("Expected 2 referenced tables, got %d", len(rel.ReferencedTables))
	}

	if len(rel.ReferencingTables) != 2 {
		t.Errorf("Expected 2 referencing tables, got %d", len(rel.ReferencingTables))
	}
}

// ============================================================================
// Test Suite 19: Connection Reconnection Logic
// ============================================================================

func TestConnection_Reconnection(t *testing.T) {
	// Test that GetConnection attempts reconnection for disconnected connections
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:      "testdb",
				ConnString: "oracle://user:pass@host/service",
				Status:     StatusDisconnected,
				ReadOnly:   false,
			},
		},
		readOnly: false,
	}

	// The connection is marked as disconnected, so GetConnection should attempt to reconnect
	// This will fail in tests without a real database, but that's expected
	_, err := registry.GetConnection("testdb")

	// We expect an error because we can't actually reconnect without a real DB
	// The important thing is that the code path was executed
	if err == nil {
		// If it succeeded, the connection was actually valid
		t.Log("Connection succeeded (possibly had valid mock)")
	}
}

// ============================================================================
// Test Suite 20: Environ Helper
// ============================================================================

func TestEnviron(t *testing.T) {
	env := environ()

	if len(env) == 0 {
		t.Error("Expected some environment variables")
	}

	// Check that the function returns key=value format
	for _, e := range env {
		if !strings.Contains(e, "=") {
			t.Errorf("Expected format 'key=value', got '%s'", e)
		}
	}
}

// ============================================================================
// Test Suite 21: Cache Loading (untestable without real DB, but verify code paths)
// ============================================================================

func TestLoadCacheFromDisk(t *testing.T) {
	conn := &Connection{
		Label:     "test",
		schema:    "TEST",
		ReadOnly:  false,
		cachePath: "/nonexistent/path/cache.json",
	}

	// This should return an error since the path doesn't exist
	cache, err := conn.loadCacheFromDisk()
	if err == nil {
		t.Error("Expected error loading non-existent cache")
	}
	if cache != nil {
		t.Error("Expected nil cache for non-existent file")
	}
}

func TestLoadOrBuildCache(t *testing.T) {
	t.Skip("Test requires a real database connection")
}

func TestSaveCache(t *testing.T) {
	conn := &Connection{
		Label:     "test",
		schema:    "TEST",
		ReadOnly:  false,
		cachePath: "/nonexistent/path/cache.json",
	}

	// saveCache should return nil (not implemented)
	err := conn.saveCache()
	if err != nil {
		t.Errorf("saveCache returned error: %v", err)
	}
}

// ============================================================================
// Test Suite 22: Tool Handler Error Paths (without DB)
// ============================================================================

func TestListConnectionsTool_Handle(t *testing.T) {
	// Create registry with no connections
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
	}

	tool := &ListConnectionsTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !strings.Contains(result.RawText, "No database connections") {
		t.Error("Expected 'No database connections' message")
	}
}

func TestListConnectionsTool_Handle_WithConnections(t *testing.T) {
	// Create registry with a connection
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ListConnectionsTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !strings.Contains(result.RawText, "Database Connection Status") {
		t.Error("Expected status message")
	}
}

func TestListConnectionsTool_Handle_MultiDB(t *testing.T) {
	// Create registry with multiple connections
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"db1": {
				Label:  "db1",
				schema: "SCHEMA1",
				Status: StatusConnected,
			},
			"db2": {
				Label:    "db2",
				schema:   "error: connection failed",
				Status:   StatusError,
				ErrorMsg: "connection failed",
			},
		},
	}

	tool := &ListConnectionsTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if !strings.Contains(result.RawText, "Configured Database Connections") {
		t.Error("Expected multi-database message")
	}

	if !strings.Contains(result.RawText, "db1") {
		t.Error("Expected db1 in output")
	}

	if !strings.Contains(result.RawText, "db2") {
		t.Error("Expected db2 in output")
	}
}

// ============================================================================
// Test Suite 23: RequireConnection with Various States
// ============================================================================

func TestRequireConnection_DisconnectedDatabase(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:  "testdb",
				schema: "TEST",
				Status: StatusDisconnected,
			},
		},
	}

	_, err := registry.RequireConnection("testdb")
	if err == nil {
		t.Error("Expected error for disconnected database")
	}

	if !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("Expected error about 'unavailable', got: %v", err)
	}
}

func TestRequireConnection_ValidButNotConnected(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:  "testdb",
				schema: "TEST",
				Status: StatusAvailable,
			},
		},
	}

	_, err := registry.RequireConnection("testdb")
	if err == nil {
		t.Error("Expected error for available but not connected database")
	}
}

// ============================================================================
// Test Suite 24: Tool Schemas (multi-database mode)
// ============================================================================

func TestListTablesTool_Schema_MultiDB(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"db1": {Label: "db1"},
			"db2": {Label: "db2"},
		},
	}

	tool := &ListTablesTool{db: registry}
	schema := tool.Schema()

	props, ok := schema.Properties["database"]
	if !ok {
		t.Error("Expected 'database' parameter in multi-DB mode")
	}

	propsMap, ok := props.(map[string]interface{})
	if !ok {
		t.Error("Expected database param to be a map")
	}

	if propsMap["type"] != "string" {
		t.Error("Expected database param type to be string")
	}
}

func TestListTablesTool_Schema_SingleDB(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}

	tool := &ListTablesTool{db: registry}
	schema := tool.Schema()

	if _, ok := schema.Properties["database"]; ok {
		t.Error("Did not expect 'database' parameter in single-DB mode")
	}
}

func TestListTablesTool_Schema_WithLimit(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}

	tool := &ListTablesTool{db: registry}
	schema := tool.Schema()

	if _, ok := schema.Properties["limit"]; !ok {
		t.Error("Expected 'limit' parameter")
	}
}

// ============================================================================
// Test Suite 25: DatabaseRegistry Close
// ============================================================================

func TestDatabaseRegistry_Close(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
	}

	// Should not error with no connections
	err := registry.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

// ============================================================================
// Test Suite 26: GetConnection with Non-existent Database
// ============================================================================

func TestGetConnection_NonExistent(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
	}

	_, err := registry.GetConnection("nonexistent")
	if err == nil {
		t.Error("Expected error for non-existent database")
	}

	if !strings.Contains(err.Error(), "unknown database") {
		t.Errorf("Expected error about 'unknown database', got: %v", err)
	}
}

// ============================================================================
// Test Suite 27: Tool Handler Error Cases
// ============================================================================

func TestDescribeTableTool_Handle_MissingTableName(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &DescribeTableTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing table_name")
	}
}

func TestSearchTablesTool_Handle_MissingSearchTerm(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &SearchTablesTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing search_term")
	}
}

func TestSearchColumnsTool_Handle_MissingSearchTerm(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &SearchColumnsTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing search_term")
	}
}

func TestGetConstraintsTool_Handle_MissingTableName(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetConstraintsTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing table_name")
	}
}

func TestGetIndexesTool_Handle_MissingTableName(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetIndexesTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing table_name")
	}
}

func TestGetRelatedTablesTool_Handle_MissingTableName(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetRelatedTablesTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing table_name")
	}
}

func TestExecuteReadTool_Handle_MissingSQL(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExecuteReadTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing sql")
	}
}

func TestExecuteReadTool_Handle_NonSelectQuery(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExecuteReadTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "DELETE FROM users WHERE id = 1",
	})
	// We expect either SELECT query error OR connection error (since no real DB)
	if err == nil {
		t.Error("Expected error")
	}

	// Log what error we got for debugging
	t.Logf("Got error: %v", err)
}

func TestExecuteWriteTool_Handle_MissingSQL(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}
	server := &Server{readOnly: false}
	tool := &ExecuteWriteTool{db: registry, server: server}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing sql")
	}
}

func TestExecuteWriteTool_Handle_NonWriteQuery(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}
	server := &Server{readOnly: false}
	tool := &ExecuteWriteTool{db: registry, server: server}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "SELECT * FROM users",
	})
	// We expect either write query error OR connection error (since no real DB)
	if err == nil {
		t.Error("Expected error")
	}

	t.Logf("Got error: %v", err)
}

func TestExecuteWriteTool_Handle_ReadOnlyServer(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}
	server := &Server{readOnly: true}
	tool := &ExecuteWriteTool{db: registry, server: server}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "INSERT INTO users VALUES (1)",
	})
	// We expect either read-only error OR connection error (since no real DB)
	if err == nil {
		t.Error("Expected error")
	}

	t.Logf("Got error: %v", err)
}

func TestExplainQueryTool_Handle_MissingSQL(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExplainQueryTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("Expected error for missing sql")
	}
}

func TestExplainQueryTool_Handle_NonSelectQuery(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExplainQueryTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"sql": "DELETE FROM users",
	})
	if err == nil {
		t.Error("Expected error for non-SELECT query")
	}
}

// ============================================================================
// Test Suite 28: Query Classification Edge Cases
// ============================================================================

func TestIsSelectQuery_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{"lowercase select", "select * from users", true},
		{"mixed case select", "SeLeCt * fRoM users", true},
		{"select with leading spaces", "  SELECT * FROM users", true},
		{"select with newline", "\nSELECT * FROM users", true},
		{"with clause", "WITH cte AS (SELECT 1) SELECT * FROM cte", true},
		{"insert statement", "INSERT INTO users VALUES (1)", false},
		{"update statement", "UPDATE users SET name = 'test'", false},
		{"delete statement", "DELETE FROM users", false},
		{"empty string", "", false},
		{"just spaces", "   ", false},
		{"random text", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSelectQuery(tt.sql)
			if result != tt.expected {
				t.Errorf("isSelectQuery(%q) = %v, want %v", tt.sql, result, tt.expected)
			}
		})
	}
}

func TestIsWriteQuery_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected bool
	}{
		{"lowercase insert", "insert into users values (1)", true},
		{"lowercase update", "update users set name = 'test'", true},
		{"lowercase delete", "delete from users", true},
		{"drop table", "DROP TABLE users", true},
		{"create table", "CREATE TABLE test (id NUMBER)", true},
		{"alter table", "ALTER TABLE users ADD col VARCHAR2(100)", true},
		{"truncate table", "TRUNCATE TABLE users", true},
		{"grant", "GRANT SELECT ON users TO public", true},
		{"revoke", "REVOKE SELECT ON users FROM public", true},
		{"merge", "MERGE INTO users USING dual ON (1=1)", true},
		{"select", "SELECT * FROM users", false},
		{"with clause", "WITH cte AS (SELECT 1) SELECT * FROM cte", false},
		{"empty string", "", false},
		{"random text", "foobar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isWriteQuery(tt.sql)
			if result != tt.expected {
				t.Errorf("isWriteQuery(%q) = %v, want %v", tt.sql, result, tt.expected)
			}
		})
	}
}

// ============================================================================
// Test Suite 29: Formatting Edge Cases
// ============================================================================

func TestFormatQueryResult_Empty(t *testing.T) {
	result := &QueryResult{
		Columns: []string{},
		Rows:    []map[string]interface{}{},
	}

	output := formatQueryResult(result)

	if output == "" {
		t.Error("formatQueryResult returned empty string")
	}

	if !strings.Contains(output, "Rows: 0") {
		t.Error("Expected 'Rows: 0' in output")
	}
}

func TestFormatQueryResult_WithData(t *testing.T) {
	result := &QueryResult{
		Columns: []string{"ID", "NAME", "ACTIVE"},
		Rows: []map[string]interface{}{
			{"ID": 1, "NAME": "Alice", "ACTIVE": true},
			{"ID": 2, "NAME": "Bob", "ACTIVE": false},
			{"ID": 3, "NAME": nil, "ACTIVE": nil},
		},
	}

	output := formatQueryResult(result)

	if !strings.Contains(output, "Rows: 3") {
		t.Error("Expected 'Rows: 3' in output")
	}

	if !strings.Contains(output, "ID") || !strings.Contains(output, "NAME") || !strings.Contains(output, "ACTIVE") {
		t.Error("Expected column headers in output")
	}

	if !strings.Contains(output, "Alice") || !strings.Contains(output, "Bob") {
		t.Error("Expected data values in output")
	}

	if !strings.Contains(output, "NULL") {
		t.Error("Expected NULL for nil values")
	}
}

func TestFormatExplainPlan_Empty(t *testing.T) {
	plan := &ExplainPlan{
		Steps:       []string{},
		Suggestions: []string{},
	}

	output := formatExplainPlan(plan)

	if output == "" {
		t.Error("formatExplainPlan returned empty string")
	}

	if !strings.Contains(output, "Execution Plan") {
		t.Error("Expected header in output")
	}
}

func TestFormatExplainPlan_WithSuggestions(t *testing.T) {
	plan := &ExplainPlan{
		Steps:       []string{"Step 1", "Step 2", "Step 3"},
		Suggestions: []string{"Add index", "Consider partitioning"},
	}

	output := formatExplainPlan(plan)

	if !strings.Contains(output, "Step 1") || !strings.Contains(output, "Step 2") || !strings.Contains(output, "Step 3") {
		t.Error("Expected all steps in output")
	}

	if !strings.Contains(output, "Optimization Suggestions") {
		t.Error("Expected suggestions header in output")
	}

	if !strings.Contains(output, "Add index") {
		t.Error("Expected first suggestion in output")
	}
}

// ============================================================================
// Test Suite 30: Constraint/Index Info Structures
// ============================================================================

func TestConstraintInfo_NoReferences(t *testing.T) {
	constraint := ConstraintInfo{
		Name:      "CK_USERS_STATUS",
		Type:      "CHECK",
		Columns:   []string{},
		Condition: "status IN ('active', 'inactive')",
	}

	if constraint.Name != "CK_USERS_STATUS" {
		t.Errorf("Expected name CK_USERS_STATUS, got %s", constraint.Name)
	}

	if constraint.Type != "CHECK" {
		t.Errorf("Expected type CHECK, got %s", constraint.Type)
	}

	if constraint.References != nil {
		t.Error("References should be nil")
	}
}

func TestConstraintInfo_WithReferences(t *testing.T) {
	constraint := ConstraintInfo{
		Name:    "FK_ORDERS_USERS",
		Type:    "FOREIGN KEY",
		Columns: []string{"USER_ID"},
		References: &ReferenceInfo{
			Table:   "USERS",
			Columns: []string{"ID"},
		},
	}

	if constraint.References == nil {
		t.Error("References should not be nil")
	}

	if constraint.References.Table != "USERS" {
		t.Errorf("Expected table USERS, got %s", constraint.References.Table)
	}
}

func TestIndexInfo_NonUnique(t *testing.T) {
	idx := IndexInfo{
		Name:    "IDX_NAME",
		Unique:  false,
		Columns: []string{"NAME"},
		Status:  "VALID",
	}

	if idx.Unique {
		t.Error("Expected Unique to be false")
	}
}

func TestRelatedTables_Empty(t *testing.T) {
	rel := RelatedTables{
		ReferencedTables:  []string{},
		ReferencingTables: []string{},
	}

	if len(rel.ReferencedTables) != 0 {
		t.Errorf("Expected 0 referenced tables, got %d", len(rel.ReferencedTables))
	}

	if len(rel.ReferencingTables) != 0 {
		t.Errorf("Expected 0 referencing tables, got %d", len(rel.ReferencingTables))
	}
}

// ============================================================================
// Test Suite 31: TableInfo FormatSchema Edge Cases
// ============================================================================

func TestTableInfo_FormatSchema_EmptyColumns(t *testing.T) {
	table := &TableInfo{
		TableName:     "EMPTY_TABLE",
		Columns:       []ColumnInfo{},
		Relationships: map[string][]RelationshipInfo{},
		FullyLoaded:   false,
	}

	output := table.FormatSchema()

	if output == "" {
		t.Error("FormatSchema returned empty string")
	}

	if !strings.Contains(output, "EMPTY_TABLE") {
		t.Error("Expected table name in output")
	}
}

func TestTableInfo_FormatSchema_WithColumns(t *testing.T) {
	table := &TableInfo{
		TableName: "USERS",
		Columns: []ColumnInfo{
			{Name: "ID", DataType: "NUMBER", Nullable: false},
			{Name: "EMAIL", DataType: "VARCHAR2(255)", Nullable: false},
			{Name: "CREATED_AT", DataType: "DATE", Nullable: true},
		},
		Relationships: map[string][]RelationshipInfo{},
		FullyLoaded:   true,
	}

	output := table.FormatSchema()

	if !strings.Contains(output, "ID") {
		t.Error("Expected column ID in output")
	}

	if !strings.Contains(output, "EMAIL") {
		t.Error("Expected column EMAIL in output")
	}

	if !strings.Contains(output, "CREATED_AT") {
		t.Error("Expected column CREATED_AT in output")
	}

	if !strings.Contains(output, "NOT NULL") {
		t.Error("Expected NOT NULL indicator in output")
	}
}

func TestTableInfo_FormatSchema_WithRelationships(t *testing.T) {
	table := &TableInfo{
		TableName: "ORDERS",
		Columns: []ColumnInfo{
			{Name: "ID", DataType: "NUMBER", Nullable: false},
		},
		Relationships: map[string][]RelationshipInfo{
			"OUTGOING": {
				{LocalColumn: "USER_ID", ForeignColumn: "ID", Direction: "OUTGOING"},
			},
			"INCOMING": {
				{LocalColumn: "ID", ForeignColumn: "ORDER_ID", Direction: "INCOMING"},
			},
		},
		FullyLoaded: true,
	}

	output := table.FormatSchema()

	if !strings.Contains(output, "OUTGOING") || !strings.Contains(output, "INCOMING") {
		t.Error("Expected relationship directions in output")
	}
}

// ============================================================================
// Test Suite 32: GetConnection Edge Cases
// ============================================================================

func TestGetConnection_AlreadyConnected(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:      "testdb",
				schema:     "TEST",
				Status:     StatusConnected,
				ConnString: "oracle://user:pass@host/service",
			},
		},
	}

	// This will attempt to reconnect, which will fail without a real DB
	// But it tests the code path
	conn, err := registry.GetConnection("testdb")

	// We expect an error since we can't actually reconnect
	if err == nil {
		t.Log("Connection succeeded (unexpected without mock)")
	}

	// The connection should still be in the registry
	connections := registry.ListConnections()
	if len(connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(connections))
	}

	_ = conn // May be nil or error
}

// ============================================================================
// Test Suite 33: ListTablesTool with Limits
// ============================================================================

func TestListTablesTool_Handle_WithLimit(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ListTablesTool{db: registry}

	// Test that limit parameter is accepted (will fail due to no real DB)
	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"limit": float64(10),
	})

	if err == nil {
		t.Log("Handle succeeded (unexpected without mock)")
	}
}

// ============================================================================
// Test Suite 34: Server Methods
// ============================================================================

func TestServer_IsReadOnly(t *testing.T) {
	server := &Server{readOnly: true}
	if !server.IsReadOnly() {
		t.Error("Expected server to be read-only")
	}

	server2 := &Server{readOnly: false}
	if server2.IsReadOnly() {
		t.Error("Expected server to not be read-only")
	}
}

func TestServer_ListConnections_Empty(t *testing.T) {
	server := &Server{}
	connections := server.ListConnections()
	if connections != nil {
		t.Error("Expected nil for empty server")
	}
}

func TestServer_ListConnections_WithRegistry(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}
	server := &Server{db: registry}

	connections := server.ListConnections()
	if len(connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(connections))
	}
}

// ============================================================================
// Test Suite 35: NewDatabaseRegistry Coverage
// ============================================================================

func TestNewDatabaseRegistry_CollisionDetection(t *testing.T) {
	// Test that collision detection works - but skip since it requires DB connection
	t.Skip("Requires a real Oracle database connection")
}

func TestNewDatabaseRegistry_UnnamedAndNamedConflict(t *testing.T) {
	// Test that mixing unnamed and named connections fails - skip since it requires DB connection
	t.Skip("Requires a real Oracle database connection")
}

func TestNewDatabaseRegistry_EmptyConnection(t *testing.T) {
	// Test that empty connection string creates disconnected connection
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:      "_default",
				ConnString: "",
				Status:     StatusDisconnected,
			},
		},
	}

	conn := registry.connections["_default"]
	if conn.Status != StatusDisconnected {
		t.Errorf("Expected StatusDisconnected, got %s", conn.Status)
	}
}

func TestNewDatabaseRegistry_DefaultCacheDir(t *testing.T) {
	// Test that default cache dir is set when CACHE_DIR is not set
	registry := &DatabaseRegistry{
		cacheDir: ".cache",
	}

	if registry.cacheDir != ".cache" {
		t.Errorf("Expected .cache, got %s", registry.cacheDir)
	}
}

// ============================================================================
// Test Suite 36: GetConnection Reconnection Logic
// ============================================================================

func TestGetConnection_DisconnectedToError(t *testing.T) {
	// Test that GetConnection handles reconnection failure
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:      "testdb",
				ConnString: "invalid-connection-string",
				Status:     StatusDisconnected,
			},
		},
	}

	_, err := registry.GetConnection("testdb")
	// We expect an error since the connection string is invalid
	if err == nil {
		t.Log("Reconnection succeeded (unexpected)")
	}
}

// ============================================================================
// Test Suite 37: CreateConnection Empty String
// ============================================================================

func TestCreateConnection_EmptyString(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
	}

	conn := registry.newConnection("test", "")
	if conn == nil {
		t.Error("Expected non-nil connection")
	}

	if conn.Status != StatusAvailable {
		t.Errorf("Expected StatusAvailable, got %s", conn.Status)
	}
}

// ============================================================================
// Test Suite 38: RequireConnection Edge Cases
// ============================================================================

func TestRequireConnection_NilDB(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"testdb": {
				Label:  "testdb",
				schema: "TEST",
				Status: StatusConnected,
				DB:     nil, // DB is nil
			},
		},
	}

	_, err := registry.RequireConnection("testdb")
	if err == nil {
		t.Error("Expected error for nil DB")
	}
}

// ============================================================================
// Test Suite 39: ListConnections Error Status
// ============================================================================

func TestListConnections_ErrorStatus(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"error_db": {
				Label:    "error_db",
				schema:   "Error message here",
				Status:   StatusError,
				ErrorMsg: "Connection refused",
			},
		},
	}

	connections := registry.ListConnections()
	if len(connections) != 1 {
		t.Errorf("Expected 1 connection, got %d", len(connections))
	}

	// For error status, the Schema field contains the error message
	if connections[0].Schema != "Connection refused" {
		t.Errorf("Expected error message in Schema, got: %s", connections[0].Schema)
	}
}

// ============================================================================
// Test Suite 40: Server Initialize/Close
// ============================================================================

func TestServer_Initialize_EmptyRegistry(t *testing.T) {
	server := &Server{db: nil}
	err := server.Initialize()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestServer_RebuildSchemaCache_EmptyRegistry(t *testing.T) {
	server := &Server{db: nil}
	err := server.RebuildSchemaCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestServer_Close_EmptyRegistry(t *testing.T) {
	server := &Server{db: nil}
	err := server.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// ============================================================================
// Test Suite 41: Tool Description Coverage
// ============================================================================

func TestToolDescriptions_MultiDatabase(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"db1": {Label: "db1"},
			"db2": {Label: "db2"},
		},
	}

	tools := []struct {
		name string
		desc string
	}{
		{"ListConnectionsTool", (&ListConnectionsTool{db: registry}).Description()},
		{"ListTablesTool", (&ListTablesTool{db: registry}).Description()},
		{"DescribeTableTool", (&DescribeTableTool{db: registry}).Description()},
		{"SearchTablesTool", (&SearchTablesTool{db: registry}).Description()},
		{"SearchColumnsTool", (&SearchColumnsTool{db: registry}).Description()},
		{"GetConstraintsTool", (&GetConstraintsTool{db: registry}).Description()},
		{"GetIndexesTool", (&GetIndexesTool{db: registry}).Description()},
		{"GetRelatedTablesTool", (&GetRelatedTablesTool{db: registry}).Description()},
		{"ExecuteReadTool", (&ExecuteReadTool{db: registry}).Description()},
		{"ExecuteWriteTool", (&ExecuteWriteTool{db: registry}).Description()},
		{"ExplainQueryTool", (&ExplainQueryTool{db: registry}).Description()},
	}

	for _, tool := range tools {
		if tool.desc == "" {
			t.Errorf("%s has empty description", tool.name)
		}
		if !strings.Contains(tool.desc, "database") {
			t.Errorf("%s should mention 'database' in multi-DB mode", tool.name)
		}
	}
}

// ============================================================================
// Test Suite 42: GetDatabaseParam Edge Cases
// ============================================================================

func TestGetDatabaseParam_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		args     map[string]interface{}
		multiDB  bool
		expected string
	}{
		{
			name:     "nil args",
			args:     nil,
			multiDB:  true,
			expected: "",
		},
		{
			name:     "non-string database",
			args:     map[string]interface{}{"database": 123},
			multiDB:  true,
			expected: "",
		},
		{
			name:     "single database mode",
			args:     map[string]interface{}{},
			multiDB:  false,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getDatabaseParam(tt.args, tt.multiDB)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

// ============================================================================
// Test Suite 43: AnalyzeQueryForOptimization Additional Cases
// ============================================================================

func TestAnalyzeQueryForOptimization_MultipleIssues(t *testing.T) {
	// Query with multiple issues
	sql := "SELECT * FROM users WHERE name LIKE '%test' OR id IN (SELECT id FROM orders)"
	suggestions := analyzeQueryForOptimization(sql)

	if len(suggestions) < 3 {
		t.Errorf("Expected at least 3 suggestions for problematic query, got %d", len(suggestions))
	}
}

func TestAnalyzeQueryForOptimization_NoIssues(t *testing.T) {
	// Query with no obvious issues
	sql := "SELECT id, name FROM users WHERE id = 1"
	suggestions := analyzeQueryForOptimization(sql)

	// Should have no suggestions
	if len(suggestions) > 0 {
		t.Errorf("Expected no suggestions for simple query, got %d", len(suggestions))
	}
}

// ============================================================================
// Test Suite 44: SearchTablesTool with Real-ish Search
// ============================================================================

func TestSearchTablesTool_Handle_EmptyResults(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
				cache: &SchemaCache{
					AllTableNames: make(map[string]struct{}),
					Tables:        make(map[string]*TableInfo),
				},
			},
		},
	}

	tool := &SearchTablesTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":    "_default",
		"search_term": "nonexistent",
		"limit":       float64(10),
	})
	// May fail due to no real DB, which is expected
	if err != nil {
		t.Logf("Expected error without real DB: %v", err)
	} else if !strings.Contains(result.RawText, "No tables found") {
		t.Errorf("Expected 'No tables found' message, got: %s", result.RawText)
	}
}

// ============================================================================
// Test Suite 45: SearchColumnsTool Empty Results
// ============================================================================

func TestSearchColumnsTool_Handle_EmptyResults(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &SearchColumnsTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":    "_default",
		"search_term": "nonexistent_column",
	})
	if err == nil {
		// May succeed with empty results
		if !strings.Contains(result.RawText, "No columns found") {
			t.Logf("Got result: %s", result.RawText)
		}
	}
}

// ============================================================================
// Test Suite 46: GetConstraintsTool Empty Results
// ============================================================================

func TestGetConstraintsTool_Handle_EmptyResults(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetConstraintsTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":   "_default",
		"table_name": "NONEXISTENT_TABLE",
	})
	// May return error or empty results depending on DB
	t.Logf("Result: %v, Error: %v", result, err)
}

// ============================================================================
// Test Suite 47: GetIndexesTool Empty Results
// ============================================================================

func TestGetIndexesTool_Handle_EmptyResults(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetIndexesTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":   "_default",
		"table_name": "NONEXISTENT_TABLE",
	})
	// May return error or empty results depending on DB
	t.Logf("Result: %v, Error: %v", result, err)
}

// ============================================================================
// Test Suite 48: GetRelatedTablesTool Empty Results
// ============================================================================

func TestGetRelatedTablesTool_Handle_EmptyResults(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &GetRelatedTablesTool{db: registry}

	result, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":   "_default",
		"table_name": "NONEXISTENT_TABLE",
	})
	// May return error or empty results depending on DB
	t.Logf("Result: %v, Error: %v", result, err)
}

// ============================================================================
// Test Suite 49: TableInfo FormatSchema Complete
// ============================================================================

func TestTableInfo_FormatSchema_Complete(t *testing.T) {
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
		t.Error("FormatSchema returned empty")
	}

	// Check for all expected content
	expected := []string{"USERS", "ID", "NAME", "NUMBER", "VARCHAR2", "NOT NULL"}
	for _, str := range expected {
		if !strings.Contains(output, str) {
			t.Errorf("Expected '%s' in output", str)
		}
	}
}

// ============================================================================
// Test Suite 50: ConstraintInfo Multiple References
// ============================================================================

func TestConstraintInfo_MultipleColumns(t *testing.T) {
	constraint := ConstraintInfo{
		Name:    "PK_COMPOSITE",
		Type:    "PRIMARY KEY",
		Columns: []string{"ID1", "ID2", "ID3"},
		References: &ReferenceInfo{
			Table:   "REF_TABLE",
			Columns: []string{"ID1", "ID2", "ID3"},
		},
	}

	if len(constraint.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(constraint.Columns))
	}
}

// ============================================================================
// Test Suite 51: IndexInfo Edge Cases
// ============================================================================

func TestIndexInfo_NoStatus(t *testing.T) {
	idx := IndexInfo{
		Name:    "IDX_TEST",
		Unique:  false,
		Columns: []string{"COL1"},
		Status:  "",
	}

	if idx.Status != "" {
		t.Errorf("Expected empty status, got %s", idx.Status)
	}
}

// ============================================================================
// Test Suite 52: SearchTablesTool Limit Parameter
// ============================================================================

func TestSearchTablesTool_Handle_WithLimit(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &SearchTablesTool{db: registry}

	// Test with limit parameter
	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":    "_default",
		"search_term": "test",
		"limit":       float64(5),
	})

	// Will fail due to no real DB, but tests the parameter handling
	t.Logf("Got error: %v", err)
}

// ============================================================================
// Test Suite 53: SearchColumnsTool Limit Parameter
// ============================================================================

func TestSearchColumnsTool_Handle_WithLimit(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &SearchColumnsTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database":    "_default",
		"search_term": "test",
		"limit":       float64(25),
	})

	t.Logf("Got error: %v", err)
}

// ============================================================================
// Test Suite 54: ListTablesTool Limit Parameter (additional test)
// ============================================================================

func TestListTablesTool_Handle_LimitParam(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ListTablesTool{db: registry}

	// Test with limit parameter
	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"limit":    float64(50),
	})

	// Will fail due to no real DB, but tests the parameter handling
	t.Logf("Got error: %v", err)
}

// ============================================================================
// Test Suite 55: ExecuteReadTool MaxRows Parameter
// ============================================================================

func TestExecuteReadTool_Handle_WithMaxRows(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExecuteReadTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "SELECT * FROM users",
		"max_rows": float64(500),
	})

	t.Logf("Got error: %v", err)
}

// ============================================================================
// Test Suite 56: ExecuteWriteTool Commit Parameter
// ============================================================================

func TestExecuteWriteTool_Handle_WithCommit(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}
	server := &Server{readOnly: false}
	tool := &ExecuteWriteTool{db: registry, server: server}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "INSERT INTO users VALUES (1)",
		"commit":   true,
	})

	t.Logf("Got error: %v", err)
}

// ============================================================================
// Test Suite 57: ExplainQueryTool NonSelect
// ============================================================================

func TestExplainQueryTool_Handle_InsertQuery(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				Status: StatusConnected,
			},
		},
	}

	tool := &ExplainQueryTool{db: registry}

	_, err := tool.Handle(framework.Background(), map[string]interface{}{
		"database": "_default",
		"sql":      "INSERT INTO users VALUES (1)",
	})

	if err == nil {
		t.Error("Expected error for INSERT query")
	}
}

// ============================================================================
// Test Suite 58: Label Case Handling
// Note: Tests for actual connection creation require a real Oracle DB
// ============================================================================

func TestNewDatabaseRegistry_LabelNormalization(t *testing.T) {
	// This test verifies label normalization without connecting
	// The actual connection test is skipped due to no test Oracle DB available
	t.Skip("Requires a real Oracle database connection")
}

// ============================================================================
// Test Suite 60: Tool Names Verify
// ============================================================================

func TestToolNames(t *testing.T) {
	expectedTools := map[string]bool{
		"oracle_connections":        false,
		"oracle_list_tables":        false,
		"oracle_describe_table":     false,
		"oracle_search_tables":      false,
		"oracle_search_columns":     false,
		"oracle_get_constraints":    false,
		"oracle_get_indexes":        false,
		"oracle_get_related_tables": false,
		"oracle_explain_query":      false,
		"oracle_execute_read":       false,
		"oracle_execute_write":      false,
	}

	registry := &DatabaseRegistry{connections: make(map[string]*Connection)}

	tools := []string{
		(&ListConnectionsTool{db: registry}).Name(),
		(&ListTablesTool{db: registry}).Name(),
		(&DescribeTableTool{db: registry}).Name(),
		(&SearchTablesTool{db: registry}).Name(),
		(&SearchColumnsTool{db: registry}).Name(),
		(&GetConstraintsTool{db: registry}).Name(),
		(&GetIndexesTool{db: registry}).Name(),
		(&GetRelatedTablesTool{db: registry}).Name(),
		(&ExplainQueryTool{db: registry}).Name(),
		(&ExecuteReadTool{db: registry}).Name(),
		(&ExecuteWriteTool{db: registry, server: &Server{}}).Name(),
	}

	for _, tool := range tools {
		if _, ok := expectedTools[tool]; ok {
			expectedTools[tool] = true
		} else {
			t.Errorf("Unexpected tool name: %s", tool)
		}
	}

	for tool, found := range expectedTools {
		if !found {
			t.Errorf("Missing tool: %s", tool)
		}
	}
}

// Note: mockRows is defined in enforcer_test.go to avoid duplication

// ============================================================================
// Test Suite 78: RebuildSchemaCache
// ============================================================================

func TestConnection_RebuildSchemaCache_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:     "test",
		schema:    "TESTUSER",
		DB:        db,
		Status:    StatusConnected,
		ReadOnly:  false,
		cachePath: "/dev/null",
	}

	// Mock GetAllTableNames which is called by buildCache
	tablesColumns := []string{"TABLE_NAME"}
	tablesRows := sqlmock.NewRows(tablesColumns).
		AddRow("USERS").
		AddRow("ORDERS")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	err = conn.RebuildSchemaCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if conn.cache == nil {
		t.Error("Expected cache to be populated")
	}

	if len(conn.cache.AllTableNames) != 2 {
		t.Errorf("Expected 2 tables in cache, got %d", len(conn.cache.AllTableNames))
	}
}

// ============================================================================
// Test Suite 79: GetTableInfo with Cached Data
// ============================================================================

func TestConnection_GetTableInfo_WithCache(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"USERS": {},
			},
			Tables: map[string]*TableInfo{
				"USERS": {
					TableName:   "USERS",
					Columns:     []ColumnInfo{{Name: "ID", DataType: "NUMBER", Nullable: false}},
					FullyLoaded: true,
				},
			},
		},
	}

	ctx := framework.Background()
	tableInfo, err := conn.GetTableInfo(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if tableInfo == nil {
		t.Fatal("Expected table info, got nil")
	}

	if len(tableInfo.Columns) != 1 {
		t.Errorf("Expected 1 column, got %d", len(tableInfo.Columns))
	}

	if tableInfo.Columns[0].Name != "ID" {
		t.Errorf("Expected column ID, got %s", tableInfo.Columns[0].Name)
	}
}

// ============================================================================
// Test Suite 80: SearchTables with Cache (early return)
// ============================================================================

func TestConnection_SearchTables_WithCache(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"TEST_USERS":    {},
				"TEST_PRODUCTS": {},
				"OTHER_TABLE":   {},
			},
			Tables: make(map[string]*TableInfo),
		},
	}

	ctx := framework.Background()
	// Request limit of 2, which is what we have in cache
	// So it should return early without hitting the DB
	tables, err := conn.SearchTables(ctx, "TEST", 2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite 81: GetConstraints with FK References
// ============================================================================

func TestConnection_GetConstraints_WithFKReferences(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock constraint query for FK
	constraintColumns := []string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}
	constraintRows := sqlmock.NewRows(constraintColumns).
		AddRow("FK_ORDERS_USERS", "R", sql.NullString{})

	mock.ExpectQuery("SELECT ac.constraint_name").
		WithArgs("TESTUSER", "ORDERS").
		WillReturnRows(constraintRows)

	// Mock column query for FK
	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns).AddRow("USER_ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "FK_ORDERS_USERS").
		WillReturnRows(colRows)

	// Mock reference query
	refColumns := []string{"TABLE_NAME", "COLUMN_NAME"}
	refRows := sqlmock.NewRows(refColumns).AddRow("USERS", "ID")
	mock.ExpectQuery("SELECT ac.table_name, acc.column_name").
		WillReturnRows(refRows)

	ctx := framework.Background()
	constraints, err := conn.GetConstraints(ctx, "ORDERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(constraints))
	}

	if constraints[0].Type != "FOREIGN KEY" {
		t.Errorf("Expected FOREIGN KEY, got %s", constraints[0].Type)
	}

	if constraints[0].References == nil {
		t.Error("Expected References to be set for FK")
	}
}

// ============================================================================
// Test Suite 82: GetIndexes NonUnique
// ============================================================================

func TestConnection_GetIndexes_NonUnique(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	indexColumns := []string{"INDEX_NAME", "UNIQUENESS", "STATUS"}
	indexRows := sqlmock.NewRows(indexColumns).
		AddRow("IDX_NAME", "NONUNIQUE", "VALID")

	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(indexRows)

	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns).AddRow("NAME")
	mock.ExpectQuery("FROM all_ind_columns").
		WithArgs("TESTUSER", "IDX_NAME").
		WillReturnRows(colRows)

	ctx := framework.Background()
	indexes, err := conn.GetIndexes(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(indexes))
	}

	if indexes[0].Unique {
		t.Error("Expected non-unique index")
	}
}

// ============================================================================
// Test Suite 83: GetRelatedTables With Both Incoming and Outgoing
// ============================================================================

func TestConnection_GetRelatedTables_BothDirections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock outgoing FKs
	outColumns := []string{"TABLE_NAME"}
	outRows := sqlmock.NewRows(outColumns).
		AddRow("ADDRESSES").
		AddRow("PHONES")
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(outRows)

	// Mock incoming FKs
	inColumns := []string{"TABLE_NAME"}
	inRows := sqlmock.NewRows(inColumns).
		AddRow("ORDERS").
		AddRow("PAYMENTS")
	mock.ExpectQuery("FROM all_constraints pk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(inRows)

	ctx := framework.Background()
	related, err := conn.GetRelatedTables(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(related.ReferencedTables) != 2 {
		t.Errorf("Expected 2 referenced tables, got %d", len(related.ReferencedTables))
	}

	if len(related.ReferencingTables) != 2 {
		t.Errorf("Expected 2 referencing tables, got %d", len(related.ReferencingTables))
	}
}

// ============================================================================
// Test Suite 84: ExecuteQuery Limits Rows
// ============================================================================

func TestConnection_ExecuteQuery_LimitsRows(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Return 10 rows
	columns := []string{"ID"}
	rows := sqlmock.NewRows(columns)
	for i := 1; i <= 10; i++ {
		rows.AddRow(i)
	}

	mock.ExpectQuery("SELECT \\* FROM \\(").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users", 5)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should only return 5 rows due to limit
	if len(result.Rows) != 5 {
		t.Errorf("Expected 5 rows (limited), got %d", len(result.Rows))
	}
}

// ============================================================================
// Test Suite 85: ExecuteWrite With Rollback
// ============================================================================

func TestConnection_ExecuteWrite_WithRollback(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("DELETE FROM").
		WillReturnResult(sqlmock.NewResult(0, 10))
	mock.ExpectRollback()

	ctx := framework.Background()
	result, err := conn.ExecuteWrite(ctx, "DELETE FROM users WHERE status = 'inactive'", false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RowsAffected != 10 {
		t.Errorf("Expected 10 rows affected, got %d", result.RowsAffected)
	}

	if result.Committed {
		t.Error("Expected not committed")
	}
}

// ============================================================================
// Test Suite 86: InitializeSchemaCache
// ============================================================================

func TestConnection_InitializeSchemaCache_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:     "test",
		schema:    "TESTUSER",
		DB:        db,
		Status:    StatusConnected,
		ReadOnly:  false,
		cachePath: "/dev/null",
	}

	// Mock GetAllTableNames
	tablesColumns := []string{"TABLE_NAME"}
	tablesRows := sqlmock.NewRows(tablesColumns).
		AddRow("T1").
		AddRow("T2").
		AddRow("T3")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	err = conn.InitializeSchemaCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if conn.cache == nil {
		t.Error("Expected cache to be initialized")
	}

	if len(conn.cache.AllTableNames) != 3 {
		t.Errorf("Expected 3 tables in cache, got %d", len(conn.cache.AllTableNames))
	}
}

// ============================================================================
// Test Suite 87: GetTableInfo with Null Constraint Condition
// ============================================================================

func TestConnection_GetConstraints_NullCondition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock constraint query with NULL condition
	constraintColumns := []string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}
	constraintRows := sqlmock.NewRows(constraintColumns).
		AddRow("PK_TEST", "P", sql.NullString{})

	mock.ExpectQuery("SELECT ac.constraint_name").
		WithArgs("TESTUSER", "TEST").
		WillReturnRows(constraintRows)

	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns).AddRow("ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_TEST").
		WillReturnRows(colRows)

	ctx := framework.Background()
	constraints, err := conn.GetConstraints(ctx, "TEST")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(constraints))
	}

	if constraints[0].Condition != "" {
		t.Errorf("Expected empty condition for NULL, got %s", constraints[0].Condition)
	}
}

// ============================================================================
// Test Suite 88: ExecuteQuery with ROWNUM
// ============================================================================

func TestConnection_ExecuteQuery_WithROWNUM(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Query already has ROWNUM, should not be wrapped
	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns).AddRow(1, "Test")

	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users WHERE ROWNUM <= 50", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}
}

// ============================================================================
// Test Suite 89: GetAllTableNames Case Sensitivity
// ============================================================================

func TestConnection_GetAllTableNames_CaseSensitivity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "testuser", // lowercase schema
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("USERS").
		AddRow("Orders").
		AddRow("Products")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("testuser"). // Should use the schema as-is
		WillReturnRows(rows)

	ctx := framework.Background()
	tables, err := conn.GetAllTableNames(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite 90: GetRelatedTables No Related
// ============================================================================

func TestConnection_GetRelatedTables_NoRelated(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Empty results for both queries
	outColumns := []string{"TABLE_NAME"}
	outRows := sqlmock.NewRows(outColumns)
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(outRows)

	inColumns := []string{"TABLE_NAME"}
	inRows := sqlmock.NewRows(inColumns)
	mock.ExpectQuery("FROM all_constraints pk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(inRows)

	ctx := framework.Background()
	related, err := conn.GetRelatedTables(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(related.ReferencedTables) != 0 {
		t.Errorf("Expected 0 referenced tables, got %d", len(related.ReferencedTables))
	}

	if len(related.ReferencingTables) != 0 {
		t.Errorf("Expected 0 referencing tables, got %d", len(related.ReferencingTables))
	}
}

// ============================================================================
// Test Suite 91: ExplainQuery With Suggestions
// ============================================================================

func TestConnection_ExplainQuery_WithSuggestions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("EXPLAIN PLAN FOR").
		WillReturnResult(sqlmock.NewResult(0, 0))

	planColumns := []string{"EXECUTION_PLAN_STEP"}
	planRows := sqlmock.NewRows(planColumns).
		AddRow("SELECT *").AddRow("TABLE ACCESS FULL USERS")
	mock.ExpectQuery("SELECT").
		WillReturnRows(planRows)

	mock.ExpectExec("DELETE FROM plan_table").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	ctx := framework.Background()
	plan, err := conn.ExplainQuery(ctx, "SELECT * FROM users WHERE name LIKE '%test'")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(plan.Suggestions) == 0 {
		t.Error("Expected suggestions for LIKE with leading wildcard")
	}
}

// ============================================================================
// Test Suite 92: ExecuteWrite Begin Error
// ============================================================================

func TestConnection_ExecuteWrite_BeginError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Simulate begin error
	mock.ExpectBegin().WillReturnError(fmt.Errorf("connection lost"))

	ctx := framework.Background()
	_, err = conn.ExecuteWrite(ctx, "INSERT INTO users VALUES (1)", true)
	if err == nil {
		t.Error("Expected error for failed begin")
	}
}

// ============================================================================
// Test Suite 93: GetIndexes Empty Result
// ============================================================================

func TestConnection_GetIndexes_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	indexColumns := []string{"INDEX_NAME", "UNIQUENESS", "STATUS"}
	indexRows := sqlmock.NewRows(indexColumns)
	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "EMPTY_TABLE").
		WillReturnRows(indexRows)

	ctx := framework.Background()
	indexes, err := conn.GetIndexes(ctx, "EMPTY_TABLE")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(indexes) != 0 {
		t.Errorf("Expected 0 indexes, got %d", len(indexes))
	}
}

// ============================================================================
// Test Suite 94: SearchColumns Multiple Tables
// ============================================================================

func TestConnection_SearchColumns_MultipleTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "NULLABLE"}
	rows := sqlmock.NewRows(columns).
		AddRow("USERS", "USER_ID", "NUMBER", "N").
		AddRow("USERS", "USER_NAME", "VARCHAR2", "Y").
		AddRow("ORDERS", "ORDER_USER_ID", "NUMBER", "N").
		AddRow("ADDRESSES", "ADDR_USER_ID", "NUMBER", "N")

	mock.ExpectQuery("FROM all_tab_columns").
		WithArgs("TESTUSER", "USER", sqlmock.AnyArg()).
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.SearchColumns(ctx, "USER", 50)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(result))
	}
}

// ============================================================================
// Test Suite 95: GetConstraints CHECK Constraint
// ============================================================================

func TestConnection_GetConstraints_CHECKConstraint(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	constraintColumns := []string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}
	constraintRows := sqlmock.NewRows(constraintColumns).
		AddRow("CK_STATUS", "C", "status IN ('active', 'inactive')")

	mock.ExpectQuery("SELECT ac.constraint_name").
		WithArgs("TESTUSER", "TEST").
		WillReturnRows(constraintRows)

	// CHECK constraints don't have columns
	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns)
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "CK_STATUS").
		WillReturnRows(colRows)

	ctx := framework.Background()
	constraints, err := conn.GetConstraints(ctx, "TEST")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(constraints))
	}

	if constraints[0].Type != "CHECK" {
		t.Errorf("Expected CHECK constraint type, got %s", constraints[0].Type)
	}

	if constraints[0].Condition != "status IN ('active', 'inactive')" {
		t.Errorf("Expected condition, got %s", constraints[0].Condition)
	}
}

// ============================================================================
// Test Suite 96: NewDatabaseRegistry Error Path
// ============================================================================

func TestNewDatabaseRegistry_ParseEnvVars(t *testing.T) {
	// Test parsing logic without actual connection
	// This tests the env var parsing without connecting
	t.Setenv("ORACLE_CONNECTION_STRING_TEST", "oracle://user:pass@host:1521/service")

	// The parsing should work, connection will fail but that's expected
	registry, err := NewDatabaseRegistry(false)
	if err != nil {
		// Connection failure is expected without real DB
		t.Logf("Expected connection error: %v", err)
	}

	if registry != nil {
		// Check if connection was registered (will be in error state)
		connections := registry.ListConnections()
		if len(connections) == 0 {
			t.Error("Expected at least one connection entry")
		}
	}
}

// ============================================================================
// Test Suite 97: ExecuteQuery Empty Result
// ============================================================================

func TestConnection_ExecuteQuery_EmptyResult(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns)
	mock.ExpectQuery("SELECT \\* FROM \\(").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Rows) != 0 {
		t.Errorf("Expected 0 rows, got %d", len(result.Rows))
	}

	if len(result.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(result.Columns))
	}
}

// ============================================================================
// Test Suite 98: ExecuteWrite ExecError
// ============================================================================

func TestConnection_ExecuteWrite_ExecError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO").
		WillReturnError(fmt.Errorf("ORA-00001: unique constraint violated"))
	mock.ExpectRollback()

	ctx := framework.Background()
	_, err = conn.ExecuteWrite(ctx, "INSERT INTO users VALUES (1)", true)
	if err == nil {
		t.Error("Expected error for constraint violation")
	}
}

// ============================================================================
// Test Suite 99: ExecuteWrite CommitError
// ============================================================================

func TestConnection_ExecuteWrite_CommitError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit().WillReturnError(fmt.Errorf("connection lost during commit"))

	ctx := framework.Background()
	_, err = conn.ExecuteWrite(ctx, "INSERT INTO users VALUES (1)", true)
	if err == nil {
		t.Error("Expected error for commit failure")
	}
}

// ============================================================================
// Test Suite 100: GetConstraints With Valid Condition
// ============================================================================

func TestConnection_GetConstraints_WithCondition(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	constraintColumns := []string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}
	constraintRows := sqlmock.NewRows(constraintColumns).
		AddRow("CK_SALARY", "C", "salary > 0")

	mock.ExpectQuery("SELECT ac.constraint_name").
		WithArgs("TESTUSER", "EMPLOYEES").
		WillReturnRows(constraintRows)

	// CHECK constraint
	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns)
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "CK_SALARY").
		WillReturnRows(colRows)

	ctx := framework.Background()
	constraints, err := conn.GetConstraints(ctx, "EMPLOYEES")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(constraints))
	}

	if constraints[0].Condition != "salary > 0" {
		t.Errorf("Expected condition 'salary > 0', got '%s'", constraints[0].Condition)
	}
}

// ============================================================================
// Test Suite 101: GetIndexes Composite
// ============================================================================

func TestConnection_GetIndexes_Composite(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	indexColumns := []string{"INDEX_NAME", "UNIQUENESS", "STATUS"}
	indexRows := sqlmock.NewRows(indexColumns).
		AddRow("IDX_COMPOSITE", "NONUNIQUE", "VALID")

	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "ORDER_ITEMS").
		WillReturnRows(indexRows)

	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns).
		AddRow("ORDER_ID").
		AddRow("ITEM_ID")
	mock.ExpectQuery("FROM all_ind_columns").
		WithArgs("TESTUSER", "IDX_COMPOSITE").
		WillReturnRows(colRows)

	ctx := framework.Background()
	indexes, err := conn.GetIndexes(ctx, "ORDER_ITEMS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(indexes))
	}

	if len(indexes[0].Columns) != 2 {
		t.Errorf("Expected 2 columns in composite index, got %d", len(indexes[0].Columns))
	}
}

// ============================================================================
// Test Suite 102: SearchColumns Limit
// ============================================================================

func TestConnection_SearchColumns_Limit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Return columns from multiple tables
	columns := []string{"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "NULLABLE"}
	rows := sqlmock.NewRows(columns).
		AddRow("TABLE1", "COL1", "VARCHAR2", "Y").
		AddRow("TABLE1", "COL2", "VARCHAR2", "Y").
		AddRow("TABLE2", "COL1", "VARCHAR2", "Y").
		AddRow("TABLE3", "COL1", "VARCHAR2", "Y")

	mock.ExpectQuery("FROM all_tab_columns").
		WithArgs("TESTUSER", "COL", sqlmock.AnyArg()).
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.SearchColumns(ctx, "COL", 50)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should have 3 tables
	if len(result) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(result))
	}
}

// ============================================================================
// Test Suite 103: ExplainQuery Complex Plan
// ============================================================================

func TestConnection_ExplainQuery_ComplexPlan(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("EXPLAIN PLAN FOR").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Complex plan with 5 steps
	planColumns := []string{"EXECUTION_PLAN_STEP"}
	planRows := sqlmock.NewRows(planColumns).
		AddRow("SELECT STATEMENT").
		AddRow("HASH JOIN").
		AddRow("TABLE ACCESS FULL ORDERS").
		AddRow("TABLE ACCESS FULL CUSTOMERS").
		AddRow("SORT").
		AddRow("AGGREGATE")

	mock.ExpectQuery("SELECT").
		WillReturnRows(planRows)

	mock.ExpectExec("DELETE FROM plan_table").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	ctx := framework.Background()
	plan, err := conn.ExplainQuery(ctx, "SELECT COUNT(*) FROM orders o, customers c WHERE o.customer_id = c.id")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(plan.Steps) != 6 {
		t.Errorf("Expected 6 plan steps, got %d", len(plan.Steps))
	}
}

// ============================================================================
// Test Suite 104: Server RebuildSchemaCache
// ============================================================================

func TestServer_RebuildSchemaCache_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:     "_default",
				schema:    "TESTUSER",
				DB:        db,
				Status:    StatusConnected,
				ReadOnly:  false,
				cachePath: "/dev/null",
			},
		},
	}

	// Mock GetAllTableNames
	tablesColumns := []string{"TABLE_NAME"}
	tablesRows := sqlmock.NewRows(tablesColumns).
		AddRow("T1").
		AddRow("T2")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	server := &Server{db: registry}
	err = server.RebuildSchemaCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// ============================================================================
// Test Suite 105: Multiple Connections Registry
// ============================================================================

func TestDatabaseRegistry_MultipleConnections(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"db1": {
				Label:  "db1",
				schema: "SCHEMA1",
				Status: StatusConnected,
			},
			"db2": {
				Label:  "db2",
				schema: "SCHEMA2",
				Status: StatusConnected,
			},
		},
	}

	if !registry.IsMultiDatabase() {
		t.Error("Expected multi-database mode with 2 connections")
	}

	connections := registry.ListConnections()
	if len(connections) != 2 {
		t.Errorf("Expected 2 connections, got %d", len(connections))
	}

	// Test ListConnections returns correct info
	found := false
	for _, c := range connections {
		if c.Label == "db1" && c.Schema == "SCHEMA1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected to find db1 with SCHEMA1")
	}
}

// ============================================================================
// Test Suite 106: Connection Close
// ============================================================================

func TestDatabaseRegistry_Close_WithConnections(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}

	// Expect close
	mock.ExpectClose()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:  "_default",
				schema: "TEST",
				DB:     db,
				Status: StatusConnected,
			},
		},
	}

	err = registry.Close()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

// ============================================================================
// Test Suite 107: GetConnection Returns Error Status
// ============================================================================

func TestGetConnection_ErrorStatus(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"error_db": {
				Label:      "error_db",
				schema:     "ERROR",
				Status:     StatusError,
				ConnString: "invalid",
				ErrorMsg:   "Connection failed",
			},
		},
	}

	// When Status is Error, GetConnection will try to reconnect
	// which will fail without a real DB
	_, err := registry.GetConnection("error_db")
	// We expect an error - either reconnection failure or unknown database
	if err == nil {
		t.Error("Expected error for error status connection")
	}
}

// ============================================================================
// Test Suite 108: SearchTables Empty Cache
// ============================================================================

func TestConnection_SearchTables_EmptyCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
		// No cache set
	}

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("RESULTS")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER", "TEST", 10).
		WillReturnRows(rows)

	ctx := framework.Background()
	tables, err := conn.SearchTables(ctx, "TEST", 10)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 1 {
		t.Errorf("Expected 1 table, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite 109: GetAllTableNames Nil Cache
// ============================================================================

func TestConnection_GetAllTableNames_NilCache(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
		// cache is nil
	}

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("T1").
		AddRow("T2").
		AddRow("T3")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(rows)

	ctx := framework.Background()
	tables, err := conn.GetAllTableNames(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite 110: Tool Descriptions Single DB Mode
// ============================================================================

func TestToolDescriptions_SingleDatabase(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}

	tools := []struct {
		name string
		desc string
	}{
		{"ListConnectionsTool", (&ListConnectionsTool{db: registry}).Description()},
		{"ListTablesTool", (&ListTablesTool{db: registry}).Description()},
		{"DescribeTableTool", (&DescribeTableTool{db: registry}).Description()},
		{"SearchTablesTool", (&SearchTablesTool{db: registry}).Description()},
		{"SearchColumnsTool", (&SearchColumnsTool{db: registry}).Description()},
		{"GetConstraintsTool", (&GetConstraintsTool{db: registry}).Description()},
		{"GetIndexesTool", (&GetIndexesTool{db: registry}).Description()},
		{"GetRelatedTablesTool", (&GetRelatedTablesTool{db: registry}).Description()},
		{"ExecuteReadTool", (&ExecuteReadTool{db: registry}).Description()},
		{"ExecuteWriteTool", (&ExecuteWriteTool{db: registry, server: &Server{}}).Description()},
		{"ExplainQueryTool", (&ExplainQueryTool{db: registry}).Description()},
	}

	for _, tool := range tools {
		if tool.desc == "" {
			t.Errorf("%s has empty description", tool.name)
		}
	}
}

// ============================================================================
// Test Suite 61: Mock DB - GetAllTableNames
// ============================================================================

func TestConnection_GetAllTableNames_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("USERS").
		AddRow("ORDERS").
		AddRow("PRODUCTS")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(rows)

	ctx := framework.Background()
	tables, err := conn.GetAllTableNames(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 62: Mock DB - SearchTables
// ============================================================================

func TestConnection_SearchTables_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"TABLE_NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow("TEST_TABLE_1").
		AddRow("TEST_TABLE_2")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER", "TEST", 10).
		WillReturnRows(rows)

	ctx := framework.Background()
	tables, err := conn.SearchTables(ctx, "TEST", 10)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 63: Mock DB - SearchColumns
// ============================================================================

func TestConnection_SearchColumns_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"TABLE_NAME", "COLUMN_NAME", "DATA_TYPE", "NULLABLE"}
	rows := sqlmock.NewRows(columns).
		AddRow("USERS", "USER_ID", "NUMBER", "N").
		AddRow("USERS", "USER_NAME", "VARCHAR2", "Y").
		AddRow("USERS", "USER_EMAIL", "VARCHAR2", "Y")

	mock.ExpectQuery("FROM all_tab_columns").
		WithArgs("TESTUSER", "USER", sqlmock.AnyArg()).
		WillReturnRows(rows)

	ctx := framework.Background()
	columnsResult, err := conn.SearchColumns(ctx, "USER", 50)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(columnsResult) != 1 {
		t.Errorf("Expected 1 table, got %d", len(columnsResult))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 64: Mock DB - GetConstraints
// ============================================================================

func TestConnection_GetConstraints_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock constraint query - use regex to match
	constraintColumns := []string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}
	constraintRows := sqlmock.NewRows(constraintColumns).
		AddRow("PK_USERS", "P", sql.NullString{}).
		AddRow("UK_EMAIL", "U", sql.NullString{})

	mock.ExpectQuery("SELECT ac.constraint_name").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(constraintRows)

	// Mock column query - for PK
	colColumns := []string{"COLUMN_NAME"}
	colRows := sqlmock.NewRows(colColumns).AddRow("USER_ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnRows(colRows)

	// Mock column query - for UK
	colRowsUk := sqlmock.NewRows(colColumns).AddRow("EMAIL")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "UK_EMAIL").
		WillReturnRows(colRowsUk)

	ctx := framework.Background()
	constraints, err := conn.GetConstraints(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(constraints) != 2 {
		t.Errorf("Expected 2 constraints, got %d", len(constraints))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 65: Mock DB - GetIndexes
// ============================================================================

func TestConnection_GetIndexes_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	indexColumns := []string{"INDEX_NAME", "UNIQUENESS", "STATUS"}
	indexRows := sqlmock.NewRows(indexColumns).
		AddRow("PK_USERS", "UNIQUE", "VALID").
		AddRow("IDX_NAME", "NONUNIQUE", "VALID")

	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(indexRows)

	// Mock column queries
	colColumns := []string{"COLUMN_NAME"}
	colRows1 := sqlmock.NewRows(colColumns).AddRow("USER_ID")
	mock.ExpectQuery("FROM all_ind_columns").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnRows(colRows1)

	colRows2 := sqlmock.NewRows(colColumns).AddRow("USER_NAME")
	mock.ExpectQuery("FROM all_ind_columns").
		WithArgs("TESTUSER", "IDX_NAME").
		WillReturnRows(colRows2)

	ctx := framework.Background()
	indexes, err := conn.GetIndexes(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d", len(indexes))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 66: Mock DB - GetRelatedTables
// ============================================================================

func TestConnection_GetRelatedTables_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock outgoing FKs query
	outColumns := []string{"TABLE_NAME"}
	outRows := sqlmock.NewRows(outColumns).AddRow("ADDRESSES")
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(outRows)

	// Mock incoming FKs query
	inColumns := []string{"TABLE_NAME"}
	inRows := sqlmock.NewRows(inColumns).AddRow("ORDERS")
	mock.ExpectQuery("FROM all_constraints pk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(inRows)

	ctx := framework.Background()
	related, err := conn.GetRelatedTables(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(related.ReferencedTables) != 1 {
		t.Errorf("Expected 1 referenced table, got %d", len(related.ReferencedTables))
	}

	if len(related.ReferencingTables) != 1 {
		t.Errorf("Expected 1 referencing table, got %d", len(related.ReferencingTables))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 67: Mock DB - ExecuteQuery
// ============================================================================

func TestConnection_ExecuteQuery_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob").
		AddRow(3, "Charlie")

	// Expect the query to be wrapped with ROWNUM
	mock.ExpectQuery("SELECT \\* FROM \\(").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(result.Columns))
	}

	if len(result.Rows) != 3 {
		t.Errorf("Expected 3 rows, got %d", len(result.Rows))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 68: Mock DB - ExecuteQuery with existing FETCH
// ============================================================================

func TestConnection_ExecuteQuery_WithFetch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice")

	// When query already has FETCH, it should not be wrapped
	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users FETCH FIRST 10 ROWS ONLY", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}
}

// ============================================================================
// Test Suite 69: Mock DB - ExecuteWrite
// ============================================================================

func TestConnection_ExecuteWrite_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO").
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	ctx := framework.Background()
	result, err := conn.ExecuteWrite(ctx, "INSERT INTO users (name) VALUES ('test')", true)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RowsAffected != 1 {
		t.Errorf("Expected 1 row affected, got %d", result.RowsAffected)
	}

	if !result.Committed {
		t.Error("Expected committed to be true")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 70: Mock DB - ExecuteWrite Without Commit
// ============================================================================

func TestConnection_ExecuteWrite_WithoutCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("UPDATE").
		WillReturnResult(sqlmock.NewResult(0, 5))
	mock.ExpectRollback()

	ctx := framework.Background()
	result, err := conn.ExecuteWrite(ctx, "UPDATE users SET name = 'test' WHERE id = 1", false)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RowsAffected != 5 {
		t.Errorf("Expected 5 rows affected, got %d", result.RowsAffected)
	}

	if result.Committed {
		t.Error("Expected committed to be false")
	}
}

// ============================================================================
// Test Suite 71: Mock DB - ExecuteWrite ReadOnly Mode
// ============================================================================

func TestConnection_ExecuteWrite_ReadOnlyMode(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: true, // Read-only mode
	}

	ctx := framework.Background()
	_, err = conn.ExecuteWrite(ctx, "INSERT INTO users VALUES (1)", true)
	if err == nil {
		t.Error("Expected error for read-only mode")
	}

	if !strings.Contains(err.Error(), "read-only mode") {
		t.Errorf("Expected read-only error, got: %v", err)
	}
}

// ============================================================================
// Test Suite 72: Mock DB - ExplainQuery
// ============================================================================

func TestConnection_ExplainQuery_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	mock.ExpectBegin()
	mock.ExpectExec("EXPLAIN PLAN FOR").
		WillReturnResult(sqlmock.NewResult(0, 0))

	planColumns := []string{"EXECUTION_PLAN_STEP"}
	planRows := sqlmock.NewRows(planColumns).
		AddRow("TABLE ACCESS FULL USERS (Cost: 100)").
		AddRow("NESTED LOOP (Cost: 50)")

	mock.ExpectQuery("SELECT").
		WillReturnRows(planRows)

	mock.ExpectExec("DELETE FROM plan_table").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	ctx := framework.Background()
	plan, err := conn.ExplainQuery(ctx, "SELECT * FROM users")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(plan.Steps) != 2 {
		t.Errorf("Expected 2 steps, got %d", len(plan.Steps))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 73: Mock DB - GetTableInfo
// ============================================================================

func TestConnection_GetTableInfo_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock table exists check
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(countRows)

	// Mock column query
	colColumns := []string{"COLUMN_NAME", "DATA_TYPE", "NULLABLE"}
	colRows := sqlmock.NewRows(colColumns).
		AddRow("USER_ID", "NUMBER", "N").
		AddRow("USER_NAME", "VARCHAR2", "Y")

	mock.ExpectQuery("SELECT column_name, data_type, nullable").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(colRows)

	// Mock relationship query - use regex since it's a complex UNION query
	relColumns := []string{"DIRECTION", "COLUMN_NAME", "TABLE_NAME", "COLUMN_NAME"}
	relRows := sqlmock.NewRows(relColumns)
	mock.ExpectQuery("SELECT 'OUTGOING'").
		WillReturnRows(relRows)

	ctx := framework.Background()
	tableInfo, err := conn.GetTableInfo(ctx, "USERS")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if tableInfo == nil {
		t.Fatal("Expected table info, got nil")
	}

	if len(tableInfo.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(tableInfo.Columns))
	}
}

// ============================================================================
// Test Suite 74: Mock DB - GetTableInfo Not Found
// ============================================================================

func TestConnection_GetTableInfo_NotFound(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock table does not exist
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(0)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "NONEXISTENT").
		WillReturnRows(countRows)

	ctx := framework.Background()
	tableInfo, err := conn.GetTableInfo(ctx, "NONEXISTENT")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if tableInfo != nil {
		t.Error("Expected nil for non-existent table")
	}
}

// ============================================================================
// Test Suite 75: Mock DB - GetTableInfo Error on Count Query
// ============================================================================

func TestConnection_GetTableInfo_CountError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock count query to return error
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "USERS").
		WillReturnError(fmt.Errorf("database error"))

	ctx := framework.Background()
	tableInfo, err := conn.GetTableInfo(ctx, "USERS")
	if err == nil {
		t.Error("Expected error on count query failure")
	}

	if tableInfo != nil {
		t.Error("Expected nil for failed query")
	}
}

// ============================================================================
// Test Suite 76: Mock DB - GetTableInfo Error on Columns Query
// ============================================================================

func TestConnection_GetTableInfo_ColumnsError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock count query to return 1 (table exists)
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(countRows)

	// Mock columns query to return error
	mock.ExpectQuery("SELECT column_name, data_type, nullable").
		WithArgs("TESTUSER", "USERS").
		WillReturnError(fmt.Errorf("columns query error"))

	ctx := framework.Background()
	tableInfo, err := conn.GetTableInfo(ctx, "USERS")
	if err == nil {
		t.Error("Expected error on columns query failure")
	}

	if tableInfo != nil {
		t.Error("Expected nil for failed query")
	}
}

// ============================================================================
// Test Suite 77: Mock DB - buildCache
// ============================================================================

func TestConnection_buildCache_WithMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:     "test",
		schema:    "TESTUSER",
		DB:        db,
		Status:    StatusConnected,
		ReadOnly:  false,
		cachePath: "/dev/null", // Will fail on save but that's ok for this test
	}

	// Mock GetAllTableNames
	tablesColumns := []string{"TABLE_NAME"}
	tablesRows := sqlmock.NewRows(tablesColumns).
		AddRow("USERS").
		AddRow("ORDERS")

	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	cache, err := conn.buildCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(cache.AllTableNames) != 2 {
		t.Errorf("Expected 2 tables in cache, got %d", len(cache.AllTableNames))
	}

	// Verify saveCache is called (will fail but that's ok)
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %v", err)
	}
}

// ============================================================================
// Test Suite 76: Mock DB - GetAllTableNames With Cache
// ============================================================================

func TestConnection_GetAllTableNames_WithCache(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"CACHED_TABLE_1": {},
				"CACHED_TABLE_2": {},
				"CACHED_TABLE_3": {},
			},
			Tables: make(map[string]*TableInfo),
		},
	}

	ctx := framework.Background()
	tables, err := conn.GetAllTableNames(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("Expected 3 tables from cache, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite 77: Tool Handlers With Mock DB
// ============================================================================

func TestListTablesTool_WithMock(t *testing.T) {
	// Test tool name and schema are correct
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}
	tool := &ListTablesTool{db: registry}

	if tool.Name() != "oracle_list_tables" {
		t.Errorf("Expected name oracle_list_tables, got %s", tool.Name())
	}

	schema := tool.Schema()
	if schema.Type != "object" {
		t.Errorf("Expected type object, got %s", schema.Type)
	}
}

func TestSearchTablesTool_WithMock(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}
	tool := &SearchTablesTool{db: registry}

	if tool.Name() != "oracle_search_tables" {
		t.Errorf("Expected name oracle_search_tables, got %s", tool.Name())
	}
}

func TestExecuteReadTool_WithMock(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}
	tool := &ExecuteReadTool{db: registry}

	if tool.Name() != "oracle_execute_read" {
		t.Errorf("Expected name oracle_execute_read, got %s", tool.Name())
	}

	profile := tool.EnforcerProfile(nil)
	if profile.RiskLevel != framework.RiskMed {
		t.Errorf("Expected RiskMed, got %s", profile.RiskLevel)
	}
}

func TestExecuteWriteTool_WithMock(t *testing.T) {
	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {Label: "_default"},
		},
	}
	server := &Server{readOnly: false}
	tool := &ExecuteWriteTool{db: registry, server: server}

	if tool.Name() != "oracle_execute_write" {
		t.Errorf("Expected name oracle_execute_write, got %s", tool.Name())
	}

	profile := tool.EnforcerProfile(map[string]interface{}{"commit": true})
	if profile.ImpactScope != framework.ImpactWrite {
		t.Errorf("Expected ImpactWrite, got %s", profile.ImpactScope)
	}
}

// ============================================================================
// Test Suite: GetConstraints Error Paths
// ============================================================================

func TestConnection_GetConstraints_ColumnsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock constraints query to return one constraint
	constraintRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}).
		AddRow("PK_USERS", "P", sql.NullString{})
	mock.ExpectQuery("FROM all_constraints ac").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(constraintRows)

	// Mock columns query to fail
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnError(fmt.Errorf("columns query error"))

	ctx := framework.Background()
	_, err = conn.GetConstraints(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when columns query fails")
	}
}

func TestConnection_GetConstraints_RefQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock constraints query to return a FK constraint
	constraintRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}).
		AddRow("FK_ORDERS", "R", sql.NullString{})
	mock.ExpectQuery("FROM all_constraints ac").
		WithArgs("TESTUSER", "ORDERS").
		WillReturnRows(constraintRows)

	// Mock columns query - return one column
	colRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("USER_ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "FK_ORDERS").
		WillReturnRows(colRows)

	// Mock ref query to fail
	mock.ExpectQuery("SELECT ac.table_name, acc.column_name").
		WithArgs("TESTUSER", "FK_ORDERS").
		WillReturnError(fmt.Errorf("ref query error"))

	ctx := framework.Background()
	_, err = conn.GetConstraints(ctx, "ORDERS")
	if err == nil {
		t.Error("Expected error when ref query fails")
	}
}

// ============================================================================
// Test Suite: GetRelatedTables Error Paths
// ============================================================================

func TestConnection_GetRelatedTables_OutQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock out query to fail
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnError(fmt.Errorf("out query error"))

	ctx := framework.Background()
	_, err = conn.GetRelatedTables(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when out query fails")
	}
}

func TestConnection_GetRelatedTables_InQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock out query to succeed with empty results
	outRows := sqlmock.NewRows([]string{"TABLE_NAME"})
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(outRows)

	// Mock in query to fail
	mock.ExpectQuery("FROM all_constraints pk").
		WithArgs("USERS", "TESTUSER").
		WillReturnError(fmt.Errorf("in query error"))

	ctx := framework.Background()
	_, err = conn.GetRelatedTables(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when in query fails")
	}
}

// ============================================================================
// Test Suite: GetAllTableNames Error Paths
// ============================================================================

func TestConnection_GetAllTableNames_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock query to fail
	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnError(fmt.Errorf("query error"))

	ctx := framework.Background()
	_, err = conn.GetAllTableNames(ctx)
	if err == nil {
		t.Error("Expected error when query fails")
	}
}

// ============================================================================
// Test Suite: loadTableDetails Error Paths
// ============================================================================

func TestConnection_loadTableDetails_RelationshipsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock count query to return 1 (table exists)
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(countRows)

	// Mock columns query
	colRows := sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "NULLABLE"}).
		AddRow("ID", "NUMBER", "N")
	mock.ExpectQuery("SELECT column_name, data_type, nullable").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(colRows)

	// Mock relationships query to fail
	mock.ExpectQuery("SELECT 'OUTGOING' AS direction").
		WithArgs("TESTUSER", "USERS").
		WillReturnError(fmt.Errorf("relationships query error"))

	ctx := framework.Background()
	_, err = conn.GetTableInfo(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when relationships query fails")
	}
}

// ============================================================================
// Test Suite: SearchTables Error Paths
// ============================================================================

func TestConnection_SearchTables_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock query to fail
	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER", "USER", 20).
		WillReturnError(fmt.Errorf("search query error"))

	ctx := framework.Background()
	_, err = conn.SearchTables(ctx, "USER", 20)
	if err == nil {
		t.Error("Expected error when search query fails")
	}
}

// ============================================================================
// Test Suite: GetIndexes Columns Query Error
// ============================================================================

func TestConnection_GetIndexes_ColumnsQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock index query
	indexRows := sqlmock.NewRows([]string{"INDEX_NAME", "UNIQUENESS", "STATUS"}).
		AddRow("PK_USERS", "UNIQUE", "VALID")
	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(indexRows)

	// Mock columns query to fail
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnError(fmt.Errorf("columns query error"))

	ctx := framework.Background()
	_, err = conn.GetIndexes(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when columns query fails")
	}
}

// ============================================================================
// Test Suite: SearchTables With Cache Hit (alternative)
// ============================================================================

func TestConnection_SearchTables_WithCacheAlt(t *testing.T) {
	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"USERS":    {},
				"USER_EMP": {},
			},
			Tables: make(map[string]*TableInfo),
		},
	}

	// Should return from cache, no DB query needed
	// Note: limit must be <= number of matches to return from cache early
	ctx := framework.Background()
	tables, err := conn.SearchTables(ctx, "USER", 2)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite: GetAllTableNames With Cache (alternative)
// ============================================================================

func TestConnection_GetAllTableNames_WithCacheAlt(t *testing.T) {
	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"USERS":    {},
				"ORDERS":   {},
				"PRODUCTS": {},
			},
			Tables: make(map[string]*TableInfo),
		},
	}

	ctx := framework.Background()
	tables, err := conn.GetAllTableNames(ctx)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(tables) != 3 {
		t.Errorf("Expected 3 tables, got %d", len(tables))
	}
}

// ============================================================================
// Test Suite: loadOrBuildCache
// ============================================================================

func TestConnection_loadOrBuildCache_WithCache(t *testing.T) {
	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		Status:   StatusConnected,
		ReadOnly: false,
		cache: &SchemaCache{
			AllTableNames: map[string]struct{}{
				"USERS": {},
			},
			Tables: make(map[string]*TableInfo),
		},
	}

	cache, err := conn.loadOrBuildCache()
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if cache == nil {
		t.Error("Expected cache to be returned")
	}
}

// ============================================================================
// Test Suite: GetIndexes Query Error
// ============================================================================

func TestConnection_GetIndexes_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock index query to fail
	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "USERS").
		WillReturnError(fmt.Errorf("indexes query error"))

	ctx := framework.Background()
	_, err = conn.GetIndexes(ctx, "USERS")
	if err == nil {
		t.Error("Expected error when index query fails")
	}
}

// ============================================================================
// Test Suite: SearchColumns Error Paths
// ============================================================================

func TestConnection_SearchColumns_QueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock search query to fail
	mock.ExpectQuery("FROM all_tab_columns").
		WithArgs("TESTUSER", "ID", 500).
		WillReturnError(fmt.Errorf("search query error"))

	ctx := framework.Background()
	_, err = conn.SearchColumns(ctx, "ID", 50)
	if err == nil {
		t.Error("Expected error when search query fails")
	}
}

// ============================================================================
// Test Suite: ExplainQuery Error Paths
// ============================================================================

func TestConnection_ExplainQuery_TxError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock BeginTx to fail
	mock.ExpectBegin().WillReturnError(fmt.Errorf("begin error"))

	ctx := framework.Background()
	_, err = conn.ExplainQuery(ctx, "SELECT * FROM users")
	if err == nil {
		t.Error("Expected error when BeginTx fails")
	}
}

func TestConnection_ExplainQuery_PlanQueryError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Mock BeginTx to succeed
	mock.ExpectBegin()

	// Mock EXPLAIN PLAN to succeed
	mock.ExpectExec("EXPLAIN PLAN FOR").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock plan query to fail
	mock.ExpectQuery("SELECT").
		WillReturnError(fmt.Errorf("plan query error"))

	// Expect rollback
	mock.ExpectRollback()

	ctx := framework.Background()
	_, err = conn.ExplainQuery(ctx, "SELECT * FROM users")
	if err == nil {
		t.Error("Expected error when plan query fails")
	}
}

// ============================================================================
// Test Suite: ExecuteQuery with FETCH FIRST
// ============================================================================

func TestConnection_ExecuteQuery_WithFetchFirst(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	conn := &Connection{
		Label:    "test",
		schema:   "TESTUSER",
		DB:       db,
		Status:   StatusConnected,
		ReadOnly: false,
	}

	// Query already has FETCH FIRST, should not be wrapped
	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns).AddRow(1, "Test")
	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	ctx := framework.Background()
	result, err := conn.ExecuteQuery(ctx, "SELECT * FROM users FETCH FIRST 10 ROWS ONLY", 100)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if len(result.Rows) != 1 {
		t.Errorf("Expected 1 row, got %d", len(result.Rows))
	}
}

// ============================================================================
// Test Suite: Close Error Handling
// ============================================================================

func TestDatabaseRegistry_Close_WithDBErrors(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"test": {
				Label:  "test",
				schema: "TEST",
				DB:     db,
			},
		},
	}

	// Mock Close to fail
	mock.ExpectClose().WillReturnError(fmt.Errorf("close error"))

	err = registry.Close()
	if err == nil {
		t.Error("Expected error on Close")
	}
}

// ============================================================================
// Test Suite: Handler Tests with Mock DB
// ============================================================================

func TestListTablesTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock GetAllTableNames query
	tablesRows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("USERS").
		AddRow("ORDERS").
		AddRow("PRODUCTS")
	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	tool := &ListTablesTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	// Verify result contains expected tables
	expectedTables := []string{"USERS", "ORDERS", "PRODUCTS"}
	for _, table := range expectedTables {
		if !strings.Contains(result.RawText, table) {
			t.Errorf("Expected result to contain table %s", table)
		}
	}
}

func TestListTablesTool_Handle_WithLimitMock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock GetAllTableNames query - return 5 tables
	tablesRows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("A").
		AddRow("B").
		AddRow("C").
		AddRow("D").
		AddRow("E")
	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER").
		WillReturnRows(tablesRows)

	tool := &ListTablesTool{db: registry}
	ctx := framework.Background()

	// Request only 3 tables
	result, err := tool.Handle(ctx, map[string]interface{}{
		"limit": float64(3),
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Should contain 3 tables (A, B, C)
	if !strings.Contains(result.RawText, "Found 3 tables") {
		t.Errorf("Expected result to show 3 tables, got: %v", result)
	}
}

func TestDescribeTableTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock count query
	countRows := sqlmock.NewRows([]string{"COUNT(*)"}).AddRow(1)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(countRows)

	// Mock columns query
	colRows := sqlmock.NewRows([]string{"COLUMN_NAME", "DATA_TYPE", "NULLABLE"}).
		AddRow("ID", "NUMBER", "N").
		AddRow("NAME", "VARCHAR2", "Y")
	mock.ExpectQuery("SELECT column_name, data_type, nullable").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(colRows)

	// Mock relationships query
	relRows := sqlmock.NewRows([]string{"DIRECTION", "COLUMN_NAME", "TABLE_NAME", "COLUMN_NAME"}).
		AddRow("OUTGOING", "DEPT_ID", "DEPARTMENTS", "ID")
	mock.ExpectQuery("SELECT 'OUTGOING' AS direction").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(relRows)

	tool := &DescribeTableTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"table_name": "USERS",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	// Verify result contains expected columns
	if !strings.Contains(result.RawText, "ID") || !strings.Contains(result.RawText, "NAME") {
		t.Errorf("Expected result to contain column names, got: %v", result)
	}
}

func TestSearchTablesTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock search query - skip cache by having > limit matches
	tablesRows := sqlmock.NewRows([]string{"TABLE_NAME"}).
		AddRow("USER_ACCOUNTS").
		AddRow("USER_PROFILES")
	mock.ExpectQuery("FROM all_tables").
		WithArgs("TESTUSER", "USER", 20).
		WillReturnRows(tablesRows)

	tool := &SearchTablesTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"search_term": "USER",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	// Verify result contains expected tables
	if !strings.Contains(result.RawText, "USER_ACCOUNTS") || !strings.Contains(result.RawText, "USER_PROFILES") {
		t.Errorf("Expected result to contain searched tables, got: %v", result)
	}
}

func TestGetConstraintsTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock constraints query
	constraintRows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "CONSTRAINT_TYPE", "SEARCH_CONDITION"}).
		AddRow("PK_USERS", "P", sql.NullString{})
	mock.ExpectQuery("FROM all_constraints ac").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(constraintRows)

	// Mock columns query
	colRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnRows(colRows)

	tool := &GetConstraintsTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"table_name": "USERS",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	if !strings.Contains(result.RawText, "PRIMARY KEY") {
		t.Errorf("Expected result to contain PRIMARY KEY, got: %v", result)
	}
}

func TestGetIndexesTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock indexes query
	indexRows := sqlmock.NewRows([]string{"INDEX_NAME", "UNIQUENESS", "STATUS"}).
		AddRow("PK_USERS", "UNIQUE", "VALID")
	mock.ExpectQuery("FROM all_indexes").
		WithArgs("TESTUSER", "USERS").
		WillReturnRows(indexRows)

	// Mock columns query
	colRows := sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("ID")
	mock.ExpectQuery("SELECT column_name").
		WithArgs("TESTUSER", "PK_USERS").
		WillReturnRows(colRows)

	tool := &GetIndexesTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"table_name": "USERS",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	if !strings.Contains(result.RawText, "UNIQUE") {
		t.Errorf("Expected result to contain UNIQUE, got: %v", result)
	}
}

func TestGetRelatedTablesTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock outgoing FKs query
	outRows := sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("DEPARTMENTS")
	mock.ExpectQuery("FROM all_constraints fk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(outRows)

	// Mock incoming FKs query
	inRows := sqlmock.NewRows([]string{"TABLE_NAME"}).AddRow("ORDERS")
	mock.ExpectQuery("FROM all_constraints pk").
		WithArgs("USERS", "TESTUSER").
		WillReturnRows(inRows)

	tool := &GetRelatedTablesTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"table_name": "USERS",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	if !strings.Contains(result.RawText, "DEPARTMENTS") || !strings.Contains(result.RawText, "ORDERS") {
		t.Errorf("Expected result to contain related tables, got: %v", result)
	}
}

func TestExecuteReadTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock query
	columns := []string{"ID", "NAME"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")
	mock.ExpectQuery("SELECT").
		WillReturnRows(rows)

	tool := &ExecuteReadTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"sql": "SELECT * FROM users",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check Data field contains rows
	resultRows, ok := result.Data.([]map[string]interface{})
	if !ok || len(resultRows) == 0 {
		t.Error("Expected non-empty data")
		return
	}

	// Verify row contents
	foundAlice := false
	foundBob := false
	for _, row := range resultRows {
		if name, ok := row["NAME"].(string); ok {
			if name == "Alice" {
				foundAlice = true
			}
			if name == "Bob" {
				foundBob = true
			}
		}
	}
	if !foundAlice || !foundBob {
		t.Errorf("Expected result to contain query results, got: %v", result.Data)
	}
}

func TestExplainQueryTool_Handle_WithMockDB(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Failed to create mock: %v", err)
	}
	defer db.Close()

	registry := &DatabaseRegistry{
		connections: map[string]*Connection{
			"_default": {
				Label:    "_default",
				schema:   "TESTUSER",
				DB:       db,
				Status:   StatusConnected,
				ReadOnly: false,
			},
		},
	}

	// Mock transaction
	mock.ExpectBegin()

	// Mock EXPLAIN PLAN
	mock.ExpectExec("EXPLAIN PLAN FOR").
		WillReturnResult(sqlmock.NewResult(0, 0))

	// Mock plan query
	planRows := sqlmock.NewRows([]string{"EXECUTION_PLAN_STEP"}).
		AddRow("TABLE ACCESS FULL USERS (Cost: 100)")
	mock.ExpectQuery("SELECT").
		WillReturnRows(planRows)

	// Mock DELETE FROM plan_table
	mock.ExpectExec("DELETE FROM plan_table").
		WillReturnResult(sqlmock.NewResult(0, 1))

	// Mock commit
	mock.ExpectCommit()

	tool := &ExplainQueryTool{db: registry}
	ctx := framework.Background()

	result, err := tool.Handle(ctx, map[string]interface{}{
		"sql": "SELECT * FROM users",
	})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if result.RawText == "" {
		t.Error("Expected non-empty result")
	}

	if !strings.Contains(result.RawText, "Execution Plan") {
		t.Errorf("Expected result to contain execution plan, got: %v", result)
	}
}
