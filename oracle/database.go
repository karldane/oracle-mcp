package oracle

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/sijms/go-ora/v2"
)

// ConnectionStatus represents the status of a database connection
type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusAvailable    ConnectionStatus = "available"
	StatusError        ConnectionStatus = "error"
	StatusDisconnected ConnectionStatus = "disconnected"
)

// QueryExecutor defines the interface for executing queries against a database
// This allows for easy mocking in tests
type QueryExecutor interface {
	GetAllTableNames(ctx context.Context) ([]string, error)
	GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error)
	SearchTables(ctx context.Context, searchTerm string, limit int) ([]string, error)
	SearchColumns(ctx context.Context, searchTerm string, limit int) (map[string][]ColumnInfo, error)
	GetConstraints(ctx context.Context, tableName string) ([]ConstraintInfo, error)
	GetIndexes(ctx context.Context, tableName string) ([]IndexInfo, error)
	GetRelatedTables(ctx context.Context, tableName string) (*RelatedTables, error)
	ExecuteQuery(ctx context.Context, sql string, maxRows int) (*QueryResult, error)
	ExecuteWrite(ctx context.Context, sql string, commit bool) (*WriteResult, error)
	ExplainQuery(ctx context.Context, sql string) (*ExplainPlan, error)
	Schema() string
	IsReadOnly() bool
}

// Connection represents a single Oracle database connection
type Connection struct {
	Label      string
	ConnString string
	DB         *sql.DB
	schema     string
	Status     ConnectionStatus
	ErrorMsg   string
	ReadOnly   bool

	// Schema cache
	cache      *SchemaCache
	cacheMutex sync.RWMutex
	cachePath  string
}

// Schema returns the database schema name
func (c *Connection) Schema() string {
	return c.schema
}

// IsReadOnly returns whether this connection is read-only
func (c *Connection) IsReadOnly() bool {
	return c.ReadOnly
}

// DatabaseRegistry manages multiple Oracle database connections
type DatabaseRegistry struct {
	connections map[string]*Connection
	readOnly    bool
	mu          sync.RWMutex

	// Shared cache directory
	cacheDir string
}

// NewDatabaseRegistry creates a new database registry from connection strings
// env vars. It only parses env vars and does not connect at startup.
func NewDatabaseRegistry(readOnly bool) (*DatabaseRegistry, error) {
	registry := &DatabaseRegistry{
		connections: make(map[string]*Connection),
		readOnly:    readOnly,
		cacheDir:    os.Getenv("CACHE_DIR"),
	}

	if registry.cacheDir == "" {
		registry.cacheDir = ".cache"
	}

	// Find all connection string env vars
	var unnamedConn string
	namedConns := make(map[string]string) // lowercase label -> original label + conn string

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]

		if key == "ORACLE_CONNECTION_STRING" {
			unnamedConn = value
		} else if strings.HasPrefix(key, "ORACLE_CONNECTION_STRING_") {
			label := strings.ToLower(strings.TrimPrefix(key, "ORACLE_CONNECTION_STRING_"))
			if label == "" {
				continue
			}
			// Check for collision after lowercasing
			if existingLabel, exists := namedConns[label]; exists {
				return nil, fmt.Errorf("connection label collision after case normalization: %s (from %s) and %s both normalize to %s",
					existingLabel, existingLabel, key, label)
			}
			namedConns[label] = value
		}
	}

	// Detect conflict: both unnamed and named connections
	if unnamedConn != "" && len(namedConns) > 0 {
		return nil, fmt.Errorf("cannot use both ORACLE_CONNECTION_STRING and ORACLE_CONNECTION_STRING_* together; choose one or the other")
	}

	// Create connections (in-memory structs only, no I/O)
	if unnamedConn != "" {
		registry.connections["_default"] = registry.newConnection("_default", unnamedConn)
	} else if len(namedConns) > 0 {
		for label, connString := range namedConns {
			registry.connections[label] = registry.newConnection(label, connString)
		}
	}

	return registry, nil
}

// newConnection creates a Connection struct without connecting to the DB.
func (r *DatabaseRegistry) newConnection(label, connString string) *Connection {
	// Derive a schema name from the conn string for display purposes before connection.
	// user/pass@host:port/service -> user
	var schema string
	if parts := strings.Split(connString, "@"); len(parts) > 0 {
		if userPart := strings.Split(parts[0], "/"); len(userPart) > 0 {
			schema = userPart[0]
		}
	}

	cachePath := filepath.Join(r.cacheDir, fmt.Sprintf("%s_%s.json", strings.ToLower(schema), label))

	return &Connection{
		Label:      label,
		ConnString: connString,
		Status:     StatusAvailable,
		ReadOnly:   r.readOnly,
		schema:     strings.ToUpper(schema), // Temporary schema
		cachePath:  cachePath,
	}
}

// connect establishes a database connection for a Connection struct.
// It is intended to be called lazily by GetConnection.
func (c *Connection) connect() error {
	if c.ConnString == "" {
		c.Status = StatusDisconnected
		return fmt.Errorf("no connection string provided")
	}

	// Open connection
	db, err := sql.Open("oracle", c.ConnString)
	if err != nil {
		c.Status = StatusError
		c.ErrorMsg = err.Error()
		return fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		c.Status = StatusError
		c.ErrorMsg = err.Error()
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Get current schema
	var schema string
	row := db.QueryRowContext(ctx, "SELECT USER FROM DUAL")
	if err := row.Scan(&schema); err != nil {
		db.Close()
		c.Status = StatusError
		c.ErrorMsg = err.Error()
		return fmt.Errorf("failed to get schema: %w", err)
	}

	// Update connection state
	c.DB = db
	c.schema = strings.ToUpper(schema)
	c.Status = StatusConnected
	c.ErrorMsg = ""

	return nil
}

// ListConnections returns all configured connections with their status
func (r *DatabaseRegistry) ListConnections() []ConnectionInfo {
	r.mu.RLock()
	defer r.mu.RUnlock()

	infos := make([]ConnectionInfo, 0, len(r.connections))
	for label, conn := range r.connections {
		info := ConnectionInfo{
			Label:     label,
			Schema:    conn.schema,
			Connected: conn.Status == StatusConnected,
		}
		if conn.Status == StatusError {
			info.Schema = conn.ErrorMsg
		}
		infos = append(infos, info)
	}
	return infos
}

// GetConnection returns a connection by label, connecting lazily if necessary.
func (r *DatabaseRegistry) GetConnection(label string) (*Connection, error) {
	r.mu.RLock()
	conn, exists := r.connections[label]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unknown database connection: %s (use oracle_connections to see available connections)", label)
	}

	// If already connected, return it
	if conn.Status == StatusConnected {
		return conn, nil
	}

	// If permanently failed or disconnected, return error
	if conn.Status == StatusError || conn.Status == StatusDisconnected {
		return nil, fmt.Errorf("database %s is unavailable (status: %s, error: %s)", label, conn.Status, conn.ErrorMsg)
	}

	// Try to connect (with a write lock to prevent concurrent connection attempts)
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock, in case another goroutine just connected.
	if conn.Status == StatusConnected {
		return conn, nil
	}

	// Perform the actual connection
	if err := conn.connect(); err != nil {
		return nil, err // conn.connect() updates status and error message
	}

	return conn, nil
}

// Close closes all database connections
func (r *DatabaseRegistry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var lastErr error
	for label, conn := range r.connections {
		if conn.DB != nil {
			if err := conn.DB.Close(); err != nil {
				lastErr = err
				fmt.Fprintf(os.Stderr, "Error closing connection %s: %v\n", label, err)
			}
		}
	}
	return lastErr
}

// IsMultiDatabase returns true if multiple named connections are configured
func (r *DatabaseRegistry) IsMultiDatabase() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.connections) > 1
}

// RequireConnection returns an error if the database is not connected.
// The database parameter is required for multi-database connections.
func (r *DatabaseRegistry) RequireConnection(database string) (QueryExecutor, error) {
	// Default to _default for single-database mode
	if database == "" {
		database = "_default"
	}

	conn, err := r.GetConnection(database)
	if err != nil {
		return nil, err
	}

	if conn.Status != StatusConnected || conn.DB == nil {
		return nil, fmt.Errorf("database %s is not connected (status: %s)", database, conn.Status)
	}

	return conn, nil
}

// --- Connection-level methods ---

// IsConnected returns true if the connection is established
func (c *Connection) IsConnected() bool {
	return c.DB != nil
}

// InitializeSchemaCache loads or builds the schema cache
func (c *Connection) InitializeSchemaCache() error {
	cache, err := c.loadOrBuildCache()
	if err != nil {
		return err
	}

	c.cacheMutex.Lock()
	c.cache = cache
	c.cacheMutex.Unlock()

	return nil
}

// RebuildSchemaCache forces a rebuild of the schema cache
func (c *Connection) RebuildSchemaCache() error {
	cache, err := c.buildCache()
	if err != nil {
		return err
	}

	c.cacheMutex.Lock()
	c.cache = cache
	c.cacheMutex.Unlock()

	return c.saveCache()
}

// GetAllTableNames returns all table names
func (c *Connection) GetAllTableNames(ctx context.Context) ([]string, error) {
	c.cacheMutex.RLock()
	if c.cache != nil {
		tables := make([]string, 0, len(c.cache.AllTableNames))
		for table := range c.cache.AllTableNames {
			tables = append(tables, table)
		}
		c.cacheMutex.RUnlock()
		return tables, nil
	}
	c.cacheMutex.RUnlock()

	// Fallback to database query
	query := `
		SELECT table_name 
		FROM all_tables 
		WHERE owner = :1
		ORDER BY table_name
	`

	rows, err := c.DB.QueryContext(ctx, query, c.Schema())
	if err != nil {
		return nil, fmt.Errorf("failed to query tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

// GetTableInfo returns schema information for a table
func (c *Connection) GetTableInfo(ctx context.Context, tableName string) (*TableInfo, error) {
	tableName = strings.ToUpper(tableName)

	// Check cache first
	c.cacheMutex.RLock()
	if c.cache != nil {
		if tableInfo, ok := c.cache.Tables[tableName]; ok {
			if tableInfo.FullyLoaded {
				c.cacheMutex.RUnlock()
				return tableInfo, nil
			}
		}
	}
	c.cacheMutex.RUnlock()

	// Load from database
	tableInfo, err := c.loadTableDetails(ctx, tableName)
	if err != nil {
		return nil, err
	}

	// Update cache
	if tableInfo != nil {
		c.cacheMutex.Lock()
		if c.cache != nil {
			c.cache.Tables[tableName] = tableInfo
			c.cache.AllTableNames[tableName] = struct{}{}
			c.saveCache()
		}
		c.cacheMutex.Unlock()
	}

	return tableInfo, nil
}

// SearchTables searches for tables by name pattern
func (c *Connection) SearchTables(ctx context.Context, searchTerm string, limit int) ([]string, error) {
	searchTerm = strings.ToUpper(searchTerm)

	// First check cache
	c.cacheMutex.RLock()
	if c.cache != nil {
		var matches []string
		for tableName := range c.cache.AllTableNames {
			if strings.Contains(tableName, searchTerm) {
				matches = append(matches, tableName)
				if len(matches) >= limit {
					break
				}
			}
		}
		c.cacheMutex.RUnlock()

		if len(matches) >= limit {
			return matches, nil
		}
	} else {
		c.cacheMutex.RUnlock()
	}

	// Query database
	query := `
		SELECT table_name 
		FROM all_tables 
		WHERE owner = :1
		AND UPPER(table_name) LIKE '%' || :2 || '%'
		ORDER BY table_name
		FETCH FIRST :3 ROWS ONLY
	`

	rows, err := c.DB.QueryContext(ctx, query, c.Schema(), searchTerm, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to search tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

// SearchColumns searches for columns by name pattern
func (c *Connection) SearchColumns(ctx context.Context, searchTerm string, limit int) (map[string][]ColumnInfo, error) {
	searchTerm = strings.ToUpper(searchTerm)

	query := `
		SELECT table_name, column_name, data_type, nullable
		FROM all_tab_columns 
		WHERE owner = :1
		AND UPPER(column_name) LIKE '%' || :2 || '%'
		ORDER BY table_name, column_id
		FETCH FIRST :3 ROWS ONLY
	`

	rows, err := c.DB.QueryContext(ctx, query, c.Schema(), searchTerm, limit*10)
	if err != nil {
		return nil, fmt.Errorf("failed to search columns: %w", err)
	}
	defer rows.Close()

	result := make(map[string][]ColumnInfo)
	count := 0

	for rows.Next() {
		var tableName, colName, dataType, nullable string
		if err := rows.Scan(&tableName, &colName, &dataType, &nullable); err != nil {
			return nil, err
		}

		if _, ok := result[tableName]; !ok {
			if count >= limit {
				break
			}
			count++
		}

		result[tableName] = append(result[tableName], ColumnInfo{
			Name:     colName,
			DataType: dataType,
			Nullable: nullable == "Y",
		})
	}

	return result, rows.Err()
}

// GetConstraints returns constraints for a table
func (c *Connection) GetConstraints(ctx context.Context, tableName string) ([]ConstraintInfo, error) {
	tableName = strings.ToUpper(tableName)

	query := `
		SELECT ac.constraint_name, ac.constraint_type, ac.search_condition
		FROM all_constraints ac
		WHERE ac.owner = :1
		AND ac.table_name = :2
	`

	rows, err := c.DB.QueryContext(ctx, query, c.Schema(), tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get constraints: %w", err)
	}
	defer rows.Close()

	var constraints []ConstraintInfo

	for rows.Next() {
		var name, constraintType, condition sql.NullString
		if err := rows.Scan(&name, &constraintType, &condition); err != nil {
			return nil, err
		}

		info := ConstraintInfo{
			Name: name.String,
			Type: mapConstraintType(constraintType.String),
		}

		if condition.Valid {
			info.Condition = condition.String
		}

		// Get columns
		colQuery := `
			SELECT column_name
			FROM all_cons_columns
			WHERE owner = :1
			AND constraint_name = :2
			ORDER BY position
		`
		colRows, err := c.DB.QueryContext(ctx, colQuery, c.Schema(), name.String)
		if err != nil {
			return nil, err
		}

		for colRows.Next() {
			var colName string
			if err := colRows.Scan(&colName); err != nil {
				colRows.Close()
				return nil, err
			}
			info.Columns = append(info.Columns, colName)
		}
		colRows.Close()

		// If FK, get referenced table
		if constraintType.String == "R" {
			refQuery := `
				SELECT ac.table_name, acc.column_name
				FROM all_constraints ac
				JOIN all_cons_columns acc ON ac.constraint_name = acc.constraint_name
					AND ac.owner = acc.owner
				WHERE ac.constraint_name = (
					SELECT r_constraint_name
					FROM all_constraints
					WHERE owner = :1
					AND constraint_name = :2
				)
				AND acc.owner = ac.owner
				ORDER BY acc.position
			`
			refRows, err := c.DB.QueryContext(ctx, refQuery, c.Schema(), name.String)
			if err != nil {
				return nil, err
			}

			if refRows.Next() {
				var refTable, refCol string
				if err := refRows.Scan(&refTable, &refCol); err != nil {
					refRows.Close()
					return nil, err
				}
				info.References = &ReferenceInfo{
					Table:   refTable,
					Columns: []string{refCol},
				}
			}
			refRows.Close()
		}

		constraints = append(constraints, info)
	}

	return constraints, rows.Err()
}

// GetIndexes returns indexes for a table
func (c *Connection) GetIndexes(ctx context.Context, tableName string) ([]IndexInfo, error) {
	tableName = strings.ToUpper(tableName)

	query := `
		SELECT index_name, uniqueness, status
		FROM all_indexes
		WHERE owner = :1
		AND table_name = :2
	`

	rows, err := c.DB.QueryContext(ctx, query, c.Schema(), tableName)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []IndexInfo

	for rows.Next() {
		var name, uniqueness, status string
		if err := rows.Scan(&name, &uniqueness, &status); err != nil {
			return nil, err
		}

		info := IndexInfo{
			Name:   name,
			Unique: uniqueness == "UNIQUE",
			Status: status,
		}

		// Get columns
		colQuery := `
			SELECT column_name
			FROM all_ind_columns
			WHERE index_owner = :1
			AND index_name = :2
			ORDER BY column_position
		`
		colRows, err := c.DB.QueryContext(ctx, colQuery, c.Schema(), name)
		if err != nil {
			return nil, err
		}

		for colRows.Next() {
			var colName string
			if err := colRows.Scan(&colName); err != nil {
				colRows.Close()
				return nil, err
			}
			info.Columns = append(info.Columns, colName)
		}
		colRows.Close()

		indexes = append(indexes, info)
	}

	return indexes, rows.Err()
}

// GetRelatedTables returns tables related by foreign keys
func (c *Connection) GetRelatedTables(ctx context.Context, tableName string) (*RelatedTables, error) {
	tableName = strings.ToUpper(tableName)

	result := &RelatedTables{
		ReferencedTables:  []string{},
		ReferencingTables: []string{},
	}

	// Tables this table references (outgoing FKs)
	outQuery := `
		SELECT DISTINCT parent_cols.table_name
		FROM all_constraints fk
		JOIN all_constraints pk ON pk.constraint_name = fk.r_constraint_name
			AND pk.owner = fk.r_owner
		JOIN all_cons_columns parent_cols ON parent_cols.constraint_name = pk.constraint_name
			AND parent_cols.owner = pk.owner
		WHERE fk.constraint_type = 'R'
		AND fk.table_name = :1
		AND fk.owner = :2
	`

	rows, err := c.DB.QueryContext(ctx, outQuery, tableName, c.Schema())
	if err != nil {
		return nil, fmt.Errorf("failed to get related tables: %w", err)
	}

	for rows.Next() {
		var refTable string
		if err := rows.Scan(&refTable); err != nil {
			rows.Close()
			return nil, err
		}
		result.ReferencedTables = append(result.ReferencedTables, refTable)
	}
	rows.Close()

	// Tables that reference this table (incoming FKs)
	inQuery := `
		SELECT DISTINCT fk.table_name
		FROM all_constraints pk
		JOIN all_constraints fk ON fk.r_constraint_name = pk.constraint_name
			AND fk.r_owner = pk.owner
		WHERE pk.constraint_type IN ('P', 'U')
		AND pk.table_name = :1
		AND pk.owner = :2
		AND fk.constraint_type = 'R'
	`

	rows, err = c.DB.QueryContext(ctx, inQuery, tableName, c.Schema())
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var refTable string
		if err := rows.Scan(&refTable); err != nil {
			rows.Close()
			return nil, err
		}
		result.ReferencingTables = append(result.ReferencingTables, refTable)
	}

	return result, rows.Close()
}

// ExecuteQuery executes a SELECT query
func (c *Connection) ExecuteQuery(ctx context.Context, sql string, maxRows int) (*QueryResult, error) {
	// Add row limiting if not present
	if !strings.Contains(strings.ToUpper(sql), "FETCH FIRST") && !strings.Contains(strings.ToUpper(sql), "ROWNUM") {
		sql = fmt.Sprintf("SELECT * FROM (%s) WHERE ROWNUM <= %d", sql, maxRows)
	}

	rows, err := c.DB.QueryContext(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := &QueryResult{
		Columns: columns,
		Rows:    []map[string]interface{}{},
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		result.Rows = append(result.Rows, row)

		if len(result.Rows) >= maxRows {
			break
		}
	}

	return result, rows.Err()
}

// ExecuteWrite executes a DML query
func (c *Connection) ExecuteWrite(ctx context.Context, sql string, commit bool) (*WriteResult, error) {
	if c.ReadOnly {
		return nil, fmt.Errorf("database is in read-only mode")
	}

	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, sql)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()

	if commit {
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit: %w", err)
		}
	}

	return &WriteResult{
		RowsAffected: rowsAffected,
		Committed:    commit,
	}, nil
}

// ExplainQuery gets the execution plan for a query
func (c *Connection) ExplainQuery(ctx context.Context, sql string) (*ExplainPlan, error) {
	tx, err := c.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Generate explain plan
	_, err = tx.ExecContext(ctx, fmt.Sprintf("EXPLAIN PLAN FOR %s", sql))
	if err != nil {
		return nil, fmt.Errorf("failed to generate explain plan: %w", err)
	}

	// Retrieve plan
	rows, err := tx.QueryContext(ctx, `
		SELECT 
			LPAD(' ', 2*LEVEL-2) || operation || ' ' || 
			options || ' ' || object_name || 
			CASE 
				WHEN cost IS NOT NULL THEN ' (Cost: ' || cost || ')'
				ELSE ''
			END || 
			CASE 
				WHEN cardinality IS NOT NULL THEN ' (Rows: ' || cardinality || ')'
				ELSE ''
			END as execution_plan_step
		FROM plan_table
		START WITH id = 0
		CONNECT BY PRIOR id = parent_id
		ORDER SIBLINGS BY position
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve explain plan: %w", err)
	}
	defer rows.Close()

	plan := &ExplainPlan{
		Steps:       []string{},
		Suggestions: []string{},
	}

	for rows.Next() {
		var step string
		if err := rows.Scan(&step); err != nil {
			return nil, err
		}
		plan.Steps = append(plan.Steps, step)
	}

	// Clean up plan table
	tx.ExecContext(ctx, "DELETE FROM plan_table")
	tx.Commit()

	// Add basic suggestions
	plan.Suggestions = analyzeQueryForOptimization(sql)

	return plan, nil
}

// --- Private helper methods ---

func (c *Connection) loadOrBuildCache() (*SchemaCache, error) {
	// Try to load from disk
	if _, err := os.Stat(c.cachePath); err == nil {
		cache, err := c.loadCacheFromDisk()
		if err == nil {
			return cache, nil
		}
	}

	// Build new cache
	return c.buildCache()
}

func (c *Connection) loadCacheFromDisk() (*SchemaCache, error) {
	// Implementation would load from JSON file
	// For now, return nil to trigger rebuild
	return nil, fmt.Errorf("cache loading not implemented")
}

func (c *Connection) buildCache() (*SchemaCache, error) {
	ctx := context.Background()
	tables, err := c.GetAllTableNames(ctx)
	if err != nil {
		return nil, err
	}

	cache := &SchemaCache{
		Tables:        make(map[string]*TableInfo),
		AllTableNames: make(map[string]struct{}),
		LastUpdated:   time.Now(),
	}

	for _, table := range tables {
		cache.AllTableNames[table] = struct{}{}
		cache.Tables[table] = &TableInfo{
			TableName:     table,
			Columns:       []ColumnInfo{},
			Relationships: make(map[string][]RelationshipInfo),
			FullyLoaded:   false,
		}
	}

	c.saveCache()
	return cache, nil
}

func (c *Connection) saveCache() error {
	// Implementation would save to JSON file
	return nil
}

func (c *Connection) loadTableDetails(ctx context.Context, tableName string) (*TableInfo, error) {
	// Check if table exists
	var count int
	err := c.DB.QueryRowContext(ctx, `
		SELECT COUNT(*) 
		FROM all_tables 
		WHERE owner = :1 AND table_name = :2
	`, c.Schema(), tableName).Scan(&count)

	if err != nil {
		return nil, err
	}

	if count == 0 {
		return nil, nil
	}

	// Get columns
	colRows, err := c.DB.QueryContext(ctx, `
		SELECT column_name, data_type, nullable
		FROM all_tab_columns
		WHERE owner = :1 AND table_name = :2
		ORDER BY column_id
	`, c.Schema(), tableName)

	if err != nil {
		return nil, err
	}
	defer colRows.Close()

	info := &TableInfo{
		TableName:     tableName,
		Columns:       []ColumnInfo{},
		Relationships: make(map[string][]RelationshipInfo),
		FullyLoaded:   true,
	}

	for colRows.Next() {
		var col ColumnInfo
		var nullable string
		if err := colRows.Scan(&col.Name, &col.DataType, &nullable); err != nil {
			return nil, err
		}
		col.Nullable = nullable == "Y"
		col.ScanPolicy, col.MaxScanLength = scanPolicyForColumn(col.Name, col.DataType)
		info.Columns = append(info.Columns, col)
	}

	// Get relationships
	relRows, err := c.DB.QueryContext(ctx, `
		SELECT 'OUTGOING' AS direction, acc.column_name, rcc.table_name, rcc.column_name
		FROM all_constraints ac
		JOIN all_cons_columns acc ON acc.constraint_name = ac.constraint_name AND acc.owner = ac.owner
		JOIN all_cons_columns rcc ON rcc.constraint_name = ac.r_constraint_name AND rcc.owner = ac.r_owner
		WHERE ac.constraint_type = 'R'
		AND ac.owner = :1
		AND ac.table_name = :2

		UNION ALL

		SELECT 'INCOMING' AS direction, rcc.column_name, ac.table_name, acc.column_name
		FROM all_constraints ac
		JOIN all_cons_columns acc ON acc.constraint_name = ac.constraint_name AND acc.owner = ac.owner
		JOIN all_cons_columns rcc ON rcc.constraint_name = ac.r_constraint_name AND rcc.owner = ac.r_owner
		WHERE ac.constraint_type = 'R'
		AND ac.r_owner = :1
		AND ac.r_constraint_name IN (
			SELECT constraint_name 
			FROM all_constraints
			WHERE owner = :1
			AND table_name = :2
			AND constraint_type IN ('P', 'U')
		)
	`, c.Schema(), tableName)

	if err != nil {
		return nil, err
	}
	defer relRows.Close()

	for relRows.Next() {
		var direction, localCol, refTable, refCol string
		if err := relRows.Scan(&direction, &localCol, &refTable, &refCol); err != nil {
			return nil, err
		}

		info.Relationships[refTable] = append(info.Relationships[refTable], RelationshipInfo{
			LocalColumn:   localCol,
			ForeignColumn: refCol,
			Direction:     direction,
		})
	}

	return info, nil
}

func mapConstraintType(t string) string {
	switch t {
	case "P":
		return "PRIMARY KEY"
	case "R":
		return "FOREIGN KEY"
	case "U":
		return "UNIQUE"
	case "C":
		return "CHECK"
	default:
		return t
	}
}

func analyzeQueryForOptimization(sql string) []string {
	sql = strings.ToUpper(sql)
	suggestions := []string{}

	if strings.Contains(sql, "SELECT *") {
		suggestions = append(suggestions, "Consider selecting only needed columns instead of SELECT *")
	}

	if strings.Contains(sql, " LIKE '%") {
		suggestions = append(suggestions, "Leading wildcards in LIKE predicates prevent index usage")
	}

	if strings.Contains(sql, " IN (SELECT ") && !strings.Contains(sql, " EXISTS") {
		suggestions = append(suggestions, "Consider using EXISTS instead of IN with subqueries for better performance")
	}

	if strings.Contains(sql, " OR ") {
		suggestions = append(suggestions, "OR conditions may prevent index usage. Consider UNION ALL of separated queries")
	}

	joinCount := strings.Count(sql, " JOIN ")
	if joinCount > 2 {
		suggestions = append(suggestions, fmt.Sprintf("Query joins %d tables - consider reviewing join order and conditions", joinCount+1))
	}

	return suggestions
}

// environ returns all environment variables as key=value pairs
func environ() []string {
	return os.Environ()
}
