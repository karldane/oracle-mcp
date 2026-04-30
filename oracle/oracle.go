// Package oracle provides an MCP backend for Oracle databases
// This is a Go-native port of the oracle-mcp-server with EnforcerProfile safety metadata
package oracle

import (
	"fmt"
	"strings"

	"github.com/karldane/mcp-framework/framework"
	"github.com/mark3labs/mcp-go/mcp"
)

// Server provides the Oracle MCP server functionality
type Server struct {
	*framework.Server
	db       *DatabaseRegistry
	readOnly bool
}

// NewServer creates a new Oracle MCP server
func NewServer(readOnly bool) (*Server, error) {
	db, err := NewDatabaseRegistry(readOnly)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database registry: %w", err)
	}

	multiDB := db.IsMultiDatabase()
	instructions := "Oracle MCP Server with multi-database support. "
	if multiDB {
		instructions += "Multiple database connections are configured. "
		instructions += "Use oracle_connections to see available databases. "
		instructions += "All tools require a 'database' parameter."
	} else {
		instructions += "Use oracle_connections to check connection status."
	}

	// Configure PII pipeline if HMAC key is provided via env
	piiConfig := buildPIIConfig()
	piiEnabled := piiConfig != nil

	config := &framework.Config{
		Name:           "oracle-mcp",
		Version:        "2.0.0",
		Instructions:   instructions,
		PIIScanEnabled: piiEnabled,
		PIIConfig:      piiConfig,
	}

	s := &Server{
		Server:   framework.NewServerWithConfig(config),
		db:       db,
		readOnly: readOnly,
	}

	s.registerTools()
	return s, nil
}

// Initialize initializes the schema cache for connected databases
func (s *Server) Initialize() error {
	if s.db == nil {
		return nil
	}

	// Initialize cache for all connected databases
	connections := s.db.ListConnections()
	for _, conn := range connections {
		if conn.Connected {
			c, err := s.db.GetConnection(conn.Label)
			if err != nil {
				continue
			}
			c.InitializeSchemaCache()
		}
	}

	s.Server.Initialize()
	return nil
}

// Close closes the database connections
func (s *Server) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Server) registerTools() {
	// Meta-tool for listing connections
	s.RegisterTool(&ListConnectionsTool{db: s.db})

	// Read-only schema introspection tools
	s.RegisterTool(&ListTablesTool{db: s.db})
	s.RegisterTool(&DescribeTableTool{db: s.db})
	s.RegisterTool(&SearchTablesTool{db: s.db})
	s.RegisterTool(&SearchColumnsTool{db: s.db})
	s.RegisterTool(&GetConstraintsTool{db: s.db})
	s.RegisterTool(&GetIndexesTool{db: s.db})
	s.RegisterTool(&GetRelatedTablesTool{db: s.db})
	s.RegisterTool(&ExplainQueryTool{db: s.db})

	// Query execution tools
	s.RegisterTool(&ExecuteReadTool{db: s.db})

	// Write tool - disabled by default
	s.RegisterTool(&ExecuteWriteTool{db: s.db, server: s})
}

// IsReadOnly returns whether the server is in read-only mode
func (s *Server) IsReadOnly() bool {
	return s.readOnly
}

// RebuildSchemaCache rebuilds the schema cache
func (s *Server) RebuildSchemaCache() error {
	if s.db == nil {
		return nil
	}
	connections := s.db.ListConnections()
	for _, conn := range connections {
		c, err := s.db.GetConnection(conn.Label)
		if err != nil {
			continue
		}
		c.RebuildSchemaCache()
	}
	return nil
}

// ListConnections returns all configured connections
func (s *Server) ListConnections() []ConnectionInfo {
	if s.db == nil {
		return nil
	}
	return s.db.ListConnections()
}

// --- Meta-tool for listing connections ---

// ListConnectionsTool lists all configured database connections
type ListConnectionsTool struct {
	db *DatabaseRegistry
}

func (t *ListConnectionsTool) Name() string {
	return "oracle_connections"
}

func (t *ListConnectionsTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "List all configured Oracle database connections with their status. " +
			"All other tools require a 'database' parameter specifying which connection to use."
	}
	return "List Oracle database connection status and configured connections."
}

func (t *ListConnectionsTool) Schema() mcp.ToolInputSchema {
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: map[string]interface{}{},
	}
}

func (t *ListConnectionsTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	connections := t.db.ListConnections()

	if len(connections) == 0 {
		result := "No database connections configured."
		return framework.TextResult(result), nil
	}

	var result strings.Builder
	if t.db.IsMultiDatabase() {
		result.WriteString(fmt.Sprintf("Configured Database Connections (%d total):\n\n", len(connections)))
	} else {
		result.WriteString("Database Connection Status:\n\n")
	}

	result.WriteString("| Database | Schema | Status |\n")
	result.WriteString("|----------|--------|--------|\n")

	for _, conn := range connections {
		status := "connected"
		if conn.Status == "error" {
			status = fmt.Sprintf("error: %s", conn.Schema)
		} else if conn.Status == "disconnected" {
			status = "disconnected"
		} else if !conn.Connected {
			status = "available"
		}

		displayLabel := conn.Label
		if displayLabel == "_default" {
			displayLabel = "(default)"
		}

		result.WriteString(fmt.Sprintf("| %s | %s | %s |\n", displayLabel, conn.Schema, status))
	}

	if t.db.IsMultiDatabase() {
		result.WriteString("\nNote: All tools require a 'database' parameter. Use this tool to see available connection labels.")
	}

	return framework.TextResult(result.String()), nil
}

func (t *ListConnectionsTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(1),
		framework.WithPII(true),
	)
}

// --- Schema introspection tools ---

// ListTablesTool lists all tables in a database
type ListTablesTool struct {
	db *DatabaseRegistry
}

func (t *ListTablesTool) Name() string {
	return "oracle_list_tables"
}

func (t *ListTablesTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "List all tables in the specified Oracle database schema. " +
			"Requires 'database' parameter."
	}
	return "List all tables in the Oracle database schema"
}

func (t *ListTablesTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of tables to return (default: 100)",
			"default":     100,
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
	}
}

func (t *ListTablesTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	limit := 100
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	tables, err := executor.GetAllTableNames(ctx)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("failed to list tables: %w", err)
	}

	if len(tables) > limit {
		tables = tables[:limit]
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d tables in %s:\n\n", len(tables), executor.Schema()))
	for _, table := range tables {
		result.WriteString(fmt.Sprintf("- %s\n", table))
	}

	return framework.TextResult(result.String()), nil
}

func (t *ListTablesTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(2),
		framework.WithPII(true),
	)
}

// DescribeTableTool returns detailed schema information for a table
type DescribeTableTool struct {
	db *DatabaseRegistry
}

func (t *DescribeTableTool) Name() string {
	return "oracle_describe_table"
}

func (t *DescribeTableTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Get detailed schema information for a specific table in the specified Oracle database " +
			"(columns, relationships). Requires 'database' parameter."
	}
	return "Get detailed schema information for a specific table (columns, relationships)"
}

func (t *DescribeTableTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"table_name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the table to describe",
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"table_name"},
	}
}

func (t *DescribeTableTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	tableName, ok := args["table_name"].(string)
	if !ok || tableName == "" {
		return framework.TextResult(""), fmt.Errorf("table_name is required")
	}

	tableInfo, err := executor.GetTableInfo(ctx, tableName)
	if err != nil {
		return framework.TextResult(""), err
	}

	if tableInfo == nil {
		return framework.TextResult(fmt.Sprintf("Table '%s' not found in the schema.", tableName)), nil
	}

	return framework.TextResult(tableInfo.FormatSchema()), nil
}

func (t *DescribeTableTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(true),
	)
}

// SearchTablesTool searches for tables by name pattern
type SearchTablesTool struct {
	db *DatabaseRegistry
}

func (t *SearchTablesTool) Name() string {
	return "oracle_search_tables"
}

func (t *SearchTablesTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Search for tables by name pattern (case-insensitive substring match) " +
			"in the specified Oracle database. Requires 'database' parameter."
	}
	return "Search for tables by name pattern (case-insensitive substring match)"
}

func (t *SearchTablesTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"search_term": map[string]interface{}{
			"type":        "string",
			"description": "Search term to match against table names",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of results (default: 20)",
			"default":     20,
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"search_term"},
	}
}

func (t *SearchTablesTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	searchTerm, ok := args["search_term"].(string)
	if !ok || searchTerm == "" {
		return framework.TextResult(""), fmt.Errorf("search_term is required")
	}

	limit := 20
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	tables, err := executor.SearchTables(ctx, searchTerm, limit)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("failed to search tables: %w", err)
	}

	if len(tables) == 0 {
		return framework.TextResult(fmt.Sprintf("No tables found matching '%s'", searchTerm)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found %d tables matching '%s' in %s:\n\n", len(tables), searchTerm, executor.Schema()))
	for _, table := range tables {
		result.WriteString(fmt.Sprintf("- %s\n", table))
	}

	return framework.TextResult(result.String()), nil
}

func (t *SearchTablesTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(true),
	)
}

// SearchColumnsTool searches for columns across all tables
type SearchColumnsTool struct {
	db *DatabaseRegistry
}

func (t *SearchColumnsTool) Name() string {
	return "oracle_search_columns"
}

func (t *SearchColumnsTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Search for columns by name pattern across all tables " +
			"in the specified Oracle database. Requires 'database' parameter."
	}
	return "Search for columns by name pattern across all tables"
}

func (t *SearchColumnsTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"search_term": map[string]interface{}{
			"type":        "string",
			"description": "Search term to match against column names",
		},
		"limit": map[string]interface{}{
			"type":        "number",
			"description": "Maximum number of tables to return (default: 50)",
			"default":     50,
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"search_term"},
	}
}

func (t *SearchColumnsTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	searchTerm, ok := args["search_term"].(string)
	if !ok || searchTerm == "" {
		return framework.TextResult(""), fmt.Errorf("search_term is required")
	}

	limit := 50
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	columns, err := executor.SearchColumns(ctx, searchTerm, limit)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("failed to search columns: %w", err)
	}

	if len(columns) == 0 {
		return framework.TextResult(fmt.Sprintf("No columns found matching '%s'", searchTerm)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Found columns matching '%s' in %d tables in %s:\n\n", searchTerm, len(columns), executor.Schema()))

	count := 0
	for tableName, cols := range columns {
		if count >= limit {
			break
		}
		result.WriteString(fmt.Sprintf("Table: %s\n", tableName))
		for _, col := range cols {
			nullable := "NOT NULL"
			if col.Nullable {
				nullable = "NULL"
			}
			result.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", col.Name, col.DataType, nullable))
		}
		result.WriteString("\n")
		count++
	}

	return framework.TextResult(result.String()), nil
}

func (t *SearchColumnsTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(4),
		framework.WithPII(true),
	)
}

// GetConstraintsTool returns constraints for a table
type GetConstraintsTool struct {
	db *DatabaseRegistry
}

func (t *GetConstraintsTool) Name() string {
	return "oracle_get_constraints"
}

func (t *GetConstraintsTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Get all constraints (PK, FK, UNIQUE, CHECK) for a table " +
			"in the specified Oracle database. Requires 'database' parameter."
	}
	return "Get all constraints (PK, FK, UNIQUE, CHECK) for a table"
}

func (t *GetConstraintsTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"table_name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the table",
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"table_name"},
	}
}

func (t *GetConstraintsTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	tableName, ok := args["table_name"].(string)
	if !ok || tableName == "" {
		return framework.TextResult(""), fmt.Errorf("table_name is required")
	}

	constraints, err := executor.GetConstraints(ctx, tableName)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("failed to get constraints: %w", err)
	}

	if len(constraints) == 0 {
		return framework.TextResult(fmt.Sprintf("No constraints found for table '%s'", tableName)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Constraints for table '%s' in %s:\n\n", tableName, executor.Schema()))

	for _, c := range constraints {
		result.WriteString(fmt.Sprintf("%s Constraint: %s\n", c.Type, c.Name))
		if len(c.Columns) > 0 {
			result.WriteString(fmt.Sprintf("  Columns: %s\n", strings.Join(c.Columns, ", ")))
		}
		if c.References != nil {
			result.WriteString(fmt.Sprintf("  References: %s(%s)\n", c.References.Table, strings.Join(c.References.Columns, ", ")))
		}
		if c.Condition != "" {
			result.WriteString(fmt.Sprintf("  Condition: %s\n", c.Condition))
		}
		result.WriteString("\n")
	}

	return framework.TextResult(result.String()), nil
}

func (t *GetConstraintsTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(true),
	)
}

// GetIndexesTool returns indexes for a table
type GetIndexesTool struct {
	db *DatabaseRegistry
}

func (t *GetIndexesTool) Name() string {
	return "oracle_get_indexes"
}

func (t *GetIndexesTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Get all indexes for a table " +
			"in the specified Oracle database. Requires 'database' parameter."
	}
	return "Get all indexes for a table"
}

func (t *GetIndexesTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"table_name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the table",
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"table_name"},
	}
}

func (t *GetIndexesTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	tableName, ok := args["table_name"].(string)
	if !ok || tableName == "" {
		return framework.TextResult(""), fmt.Errorf("table_name is required")
	}

	indexes, err := executor.GetIndexes(ctx, tableName)
	if err != nil {
		return framework.ToolResult{}, fmt.Errorf("failed to get indexes: %w", err)
	}

	if len(indexes) == 0 {
		return framework.TextResult(fmt.Sprintf("No indexes found for table '%s'", tableName)), nil
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Indexes for table '%s' in %s:\n\n", tableName, executor.Schema()))

	for _, idx := range indexes {
		unique := ""
		if idx.Unique {
			unique = "UNIQUE "
		}
		result.WriteString(fmt.Sprintf("%sIndex: %s\n", unique, idx.Name))
		result.WriteString(fmt.Sprintf("  Columns: %s\n", strings.Join(idx.Columns, ", ")))
		if idx.Status != "" {
			result.WriteString(fmt.Sprintf("  Status: %s\n", idx.Status))
		}
		result.WriteString("\n")
	}

	return framework.TextResult(result.String()), nil
}

func (t *GetIndexesTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(true),
	)
}

// GetRelatedTablesTool returns tables related by foreign keys
type GetRelatedTablesTool struct {
	db *DatabaseRegistry
}

func (t *GetRelatedTablesTool) Name() string {
	return "oracle_get_related_tables"
}

func (t *GetRelatedTablesTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Get tables related to the specified table through foreign keys " +
			"in the specified Oracle database. Requires 'database' parameter."
	}
	return "Get tables related to the specified table through foreign keys"
}

func (t *GetRelatedTablesTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"table_name": map[string]interface{}{
			"type":        "string",
			"description": "Name of the table",
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"table_name"},
	}
}

func (t *GetRelatedTablesTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	tableName, ok := args["table_name"].(string)
	if !ok || tableName == "" {
		return framework.TextResult(""), fmt.Errorf("table_name is required")
	}

	related, err := executor.GetRelatedTables(ctx, tableName)
	if err != nil {
		return framework.TextResult(""), err
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Tables related to '%s' in %s:\n\n", tableName, executor.Schema()))

	if len(related.ReferencedTables) > 0 {
		result.WriteString("Tables referenced by this table (outgoing foreign keys):\n")
		for _, table := range related.ReferencedTables {
			result.WriteString(fmt.Sprintf("  - %s\n", table))
		}
		result.WriteString("\n")
	}

	if len(related.ReferencingTables) > 0 {
		result.WriteString("Tables that reference this table (incoming foreign keys):\n")
		for _, table := range related.ReferencingTables {
			result.WriteString(fmt.Sprintf("  - %s\n", table))
		}
	}

	if len(related.ReferencedTables) == 0 && len(related.ReferencingTables) == 0 {
		result.WriteString("No related tables found.\n")
	}

	return framework.TextResult(result.String()), nil
}

func (t *GetRelatedTablesTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(3),
		framework.WithPII(true),
	)
}

// ExecuteReadTool executes SELECT queries
type ExecuteReadTool struct {
	db *DatabaseRegistry
}

func (t *ExecuteReadTool) Name() string {
	return "oracle_execute_read"
}

func (t *ExecuteReadTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Execute a read-only SELECT query on the specified Oracle database " +
			"(limited to 100 rows by default). Requires 'database' parameter."
	}
	return "Execute a read-only SELECT query (limited to 100 rows by default)"
}

func (t *ExecuteReadTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"sql": map[string]interface{}{
			"type":        "string",
			"description": "SELECT SQL query to execute",
		},
		"max_rows": map[string]interface{}{
			"type":        "number",
			"description": "Maximum rows to return (default: 100, max: 1000)",
			"default":     100,
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"sql"},
	}
}

func (t *ExecuteReadTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	sql, ok := args["sql"].(string)
	if !ok || sql == "" {
		return framework.TextResult(""), fmt.Errorf("sql is required")
	}

	maxRows := 100
	if mr, ok := args["max_rows"].(float64); ok {
		maxRows = int(mr)
		if maxRows > 1000 {
			maxRows = 1000
		}
	}

	// Ensure it's a SELECT query
	if !isSelectQuery(sql) {
		return framework.TextResult(""), fmt.Errorf("only SELECT queries are allowed with oracle_execute_read. Use oracle_execute_write for DML statements.")
	}

	result, err := executor.ExecuteQuery(ctx, sql, maxRows)
	if err != nil {
		return framework.TextResult(""), err
	}

	// Extract table name and build column hints for PII pipeline
	hints := buildHintsFromQuery(ctx, result, sql, executor)

	return framework.ToolResult{
		Data:        result.Rows,
		ColumnHints: hints,
	}, nil
}

func (t *ExecuteReadTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskMed),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(8),
		framework.WithPII(true),
		framework.WithApprovalReq(true),
	)
}

// ExecuteWriteTool executes DML queries
type ExecuteWriteTool struct {
	db     *DatabaseRegistry
	server *Server
}

func (t *ExecuteWriteTool) Name() string {
	return "oracle_execute_write"
}

func (t *ExecuteWriteTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Execute a write query (INSERT, UPDATE, DELETE) on the specified Oracle database. " +
			"Disabled without --write-enabled flag. Requires 'database' parameter."
	}
	return "Execute a write query (INSERT, UPDATE, DELETE). Disabled without --write-enabled flag."
}

func (t *ExecuteWriteTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"sql": map[string]interface{}{
			"type":        "string",
			"description": "DML SQL query to execute (INSERT, UPDATE, DELETE)",
		},
		"commit": map[string]interface{}{
			"type":        "boolean",
			"description": "Whether to commit the transaction (default: false for safety)",
			"default":     false,
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"sql"},
	}
}

func (t *ExecuteWriteTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	sql, ok := args["sql"].(string)
	if !ok || sql == "" {
		return framework.TextResult(""), fmt.Errorf("sql is required")
	}

	commit := false
	if c, ok := args["commit"].(bool); ok {
		commit = c
	}

	// Check if it's a write operation
	if !isWriteQuery(sql) {
		return framework.TextResult(""), fmt.Errorf("only INSERT, UPDATE, DELETE queries are allowed with oracle_execute_write")
	}

	// Check read-only mode
	if t.server.readOnly {
		return framework.TextResult(""), fmt.Errorf("server is in read-only mode. Set ORACLE_READ_ONLY=false to enable write operations.")
	}

	result, err := executor.ExecuteWrite(ctx, sql, commit)
	if err != nil {
		return framework.TextResult(""), err
	}

	hints := buildHintsFromWriteQuery(ctx, sql, executor)

	return framework.ToolResult{
		RawText:     formatWriteResult(result, commit),
		Data:        result.RowsAffected,
		ColumnHints: hints,
	}, nil
}

func (t *ExecuteWriteTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	commit := false
	if args != nil {
		commit, _ = args["commit"].(bool)
	}
	if commit {
		return framework.NewEnforcerProfile(
			framework.WithRisk(framework.RiskHigh),
			framework.WithImpact(framework.ImpactWrite),
			framework.WithResourceCost(8),
			framework.WithPII(true),
			framework.WithApprovalReq(true),
		)
	}
	// Dry-run / rollback-only mode
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskMed),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(8),
		framework.WithPII(true),
		framework.WithApprovalReq(false),
	)
}

// ExplainQueryTool explains a query execution plan
type ExplainQueryTool struct {
	db *DatabaseRegistry
}

func (t *ExplainQueryTool) Name() string {
	return "oracle_explain_query"
}

func (t *ExplainQueryTool) Description() string {
	if t.db.IsMultiDatabase() {
		return "Get the execution plan for a SELECT query " +
			"on the specified Oracle database. Requires 'database' parameter."
	}
	return "Get the execution plan for a SELECT query"
}

func (t *ExplainQueryTool) Schema() mcp.ToolInputSchema {
	props := map[string]interface{}{
		"database": map[string]interface{}{
			"type":        "string",
			"description": "Database connection label (required). Use oracle_connections to see available databases.",
		},
		"sql": map[string]interface{}{
			"type":        "string",
			"description": "SELECT SQL query to explain",
		},
	}
	if !t.db.IsMultiDatabase() {
		delete(props, "database")
	}
	return mcp.ToolInputSchema{
		Type:       "object",
		Properties: props,
		Required:   []string{"sql"},
	}
}

func (t *ExplainQueryTool) Handle(ctx framework.CallContext, args map[string]interface{}) (framework.ToolResult, error) {
	executor, err := t.db.RequireConnection(getDatabaseParam(args, t.db.IsMultiDatabase()))
	if err != nil {
		return framework.TextResult(""), err
	}

	sql, ok := args["sql"].(string)
	if !ok || sql == "" {
		return framework.TextResult(""), fmt.Errorf("sql is required")
	}

	if !isSelectQuery(sql) {
		return framework.TextResult(""), fmt.Errorf("only SELECT queries can be explained")
	}

	plan, err := executor.ExplainQuery(ctx, sql)
	if err != nil {
		return framework.TextResult(""), fmt.Errorf("failed to explain query: %w", err)
	}

	return framework.TextResult(formatExplainPlan(plan)), nil
}

func (t *ExplainQueryTool) EnforcerProfile(args map[string]interface{}) *framework.EnforcerProfile {
	return framework.NewEnforcerProfile(
		framework.WithRisk(framework.RiskLow),
		framework.WithImpact(framework.ImpactRead),
		framework.WithResourceCost(4),
		framework.WithPII(true),
	)
}

// --- Helper functions ---

func getDatabaseParam(args map[string]interface{}, multiDB bool) string {
	if db, ok := args["database"].(string); ok {
		return db
	}
	return ""
}

func isSelectQuery(sql string) bool {
	sql = strings.TrimSpace(strings.ToUpper(sql))
	return strings.HasPrefix(sql, "SELECT") || strings.HasPrefix(sql, "WITH")
}

func isWriteQuery(sql string) bool {
	sql = strings.TrimSpace(strings.ToUpper(sql))
	writePrefixes := []string{"INSERT", "UPDATE", "DELETE", "MERGE", "DROP", "CREATE", "ALTER", "TRUNCATE", "GRANT", "REVOKE"}
	for _, prefix := range writePrefixes {
		if strings.HasPrefix(sql, prefix) {
			return true
		}
	}
	return false
}

func formatQueryResult(result *QueryResult) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Rows: %d\n\n", len(result.Rows)))

	if len(result.Columns) > 0 {
		sb.WriteString(strings.Join(result.Columns, " | "))
		sb.WriteString("\n")
		sb.WriteString(strings.Repeat("-", 60))
		sb.WriteString("\n")
	}

	for _, row := range result.Rows {
		values := make([]string, len(result.Columns))
		for i, col := range result.Columns {
			if val, ok := row[col]; ok && val != nil {
				values[i] = fmt.Sprintf("%v", val)
			} else {
				values[i] = "NULL"
			}
		}
		sb.WriteString(strings.Join(values, " | "))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatWriteResult(result *WriteResult, committed bool) string {
	status := "executed"
	if committed {
		status = "committed"
	} else {
		status = "executed (not committed - use commit=true to persist)"
	}
	return fmt.Sprintf("Query %s successfully. %d row(s) affected.", status, result.RowsAffected)
}

func formatExplainPlan(plan *ExplainPlan) string {
	var sb strings.Builder
	sb.WriteString("Execution Plan:\n\n")

	for _, step := range plan.Steps {
		sb.WriteString(fmt.Sprintf("  %s\n", step))
	}

	if len(plan.Suggestions) > 0 {
		sb.WriteString("\nOptimization Suggestions:\n")
		for _, suggestion := range plan.Suggestions {
			sb.WriteString(fmt.Sprintf("  - %s\n", suggestion))
		}
	}

	return sb.String()
}
