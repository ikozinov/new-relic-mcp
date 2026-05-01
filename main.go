package main

import (
	"fmt"
	"os"

	"new-relic-mcp/tools"

	"github.com/joho/godotenv"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Load .env if it exists
	_ = godotenv.Load()

	s := server.NewMCPServer(
		"newrelic-mcp-go",
		"1.0.0",
	)

	tools.RegisterAllTools(s)

	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
		os.Exit(1)
	}
}
