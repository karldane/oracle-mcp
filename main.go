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

	// Check read-only mode
	readOnly := true
	if val := os.Getenv("ORACLE_READ_ONLY"); val != "" {
		readOnly = val != "false"
	}

	// Create server - connection strings are now parsed from environment in NewServer
	server, err := oracle.NewServer(readOnly)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Failed to create server: %v\n", err)
		os.Exit(1)
	}
	defer server.Close()

	// Initialize schema cache for all connected databases
	connections := server.ListConnections()
	connectedCount := 0
	for _, conn := range connections {
		if conn.Connected {
			connectedCount++
		}
	}

	if connectedCount > 0 {
		fmt.Fprintf(os.Stderr, "Initializing schema cache for %d connection(s)...\n", connectedCount)
		if err := server.Initialize(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize schema cache: %v\n", err)
		}
		fmt.Fprintln(os.Stderr, "Oracle MCP Server initialized")
	} else {
		fmt.Fprintln(os.Stderr, "Oracle MCP Server started in disconnected mode (no database connections)")
		fmt.Fprintln(os.Stderr, "Note: Configure ORACLE_CONNECTION_STRING for single connection operation or - ")
		fmt.Fprintln(os.Stderr, "ORACLE_CONNECTION_STRING_* environment variables for multiple named database connections")
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
