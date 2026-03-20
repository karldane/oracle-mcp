package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/karldane/oracle-mcp/oracle"
)

func main() {
	// Define command-line flags
	writeEnabled := flag.Bool("write-enabled", false, "Enable write tools (disabled by default for safety)")
	flag.Parse()

	// Get connection string from environment
	connString := os.Getenv("ORACLE_CONNECTION_STRING")

	// Check read-only mode
	readOnly := true
	if val := os.Getenv("ORACLE_READ_ONLY"); val != "" {
		readOnly = val != "false"
	}

	// Create server (will start without DB connection if connString is empty)
	server, err := oracle.NewServer(connString, readOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	// Set write enabled flag
	server.SetWriteEnabled(*writeEnabled)

	// Initialize schema cache (only if connected)
	if connString != "" {
		fmt.Fprintln(os.Stderr, "Initializing schema cache...")
		if err := server.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize schema cache: %v\n", err)
		}
		fmt.Fprintln(os.Stderr, "Oracle MCP Server initialized")
	} else {
		fmt.Fprintln(os.Stderr, "Oracle MCP Server started in disconnected mode (no database connection)")
		fmt.Fprintln(os.Stderr, "Note: Tools require ORACLE_CONNECTION_STRING to execute DB operations")
		fmt.Fprintln(os.Stderr, "All tools are registered and available for self-reporting.")
	}

	if readOnly {
		fmt.Fprintln(os.Stderr, "Read-only mode: enabled")
	} else {
		fmt.Fprintln(os.Stderr, "Read-only mode: disabled")
	}
	if *writeEnabled {
		fmt.Fprintln(os.Stderr, "Write tools: ENABLED")
	} else {
		fmt.Fprintln(os.Stderr, "Write tools: disabled (use -write-enabled to enable)")
	}

	fmt.Fprintf(os.Stderr, "Registered tools: %v\n", server.ListTools())
	fmt.Fprintln(os.Stderr, "Ready to serve requests via stdio...")

	// Start serving MCP requests via stdio (blocking)
	if err := server.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
