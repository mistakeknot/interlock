package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interlock/internal/client"
	"github.com/mistakeknot/interlock/internal/tools"
)

func main() {
	c := client.NewClient(
		client.WithSocketPath(os.Getenv("INTERMUTE_SOCKET")),
		client.WithBaseURL(os.Getenv("INTERMUTE_URL")),
		client.WithAgentID(getAgentID()),
		client.WithProject(getProject()),
		client.WithAgentName(getAgentName()),
	)

	s := server.NewMCPServer(
		"interlock",
		"0.1.0",
		server.WithToolCapabilities(true),
	)

	tools.RegisterAll(s, c)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interlock-mcp: %v\n", err)
		os.Exit(1)
	}
}

func getAgentID() string {
	if id := os.Getenv("INTERLOCK_AGENT_ID"); id != "" {
		return id
	}
	if id := os.Getenv("INTERMUTE_AGENT_ID"); id != "" {
		return id
	}
	if id := os.Getenv("CLAUDE_SESSION_ID"); id != "" {
		return "claude-" + id[:min(8, len(id))]
	}
	hostname, _ := os.Hostname()
	return fmt.Sprintf("%s-%d", hostname, os.Getpid())
}

func getProject() string {
	if p := os.Getenv("INTERLOCK_PROJECT"); p != "" {
		return p
	}
	dir, _ := os.Getwd()
	return filepath.Base(dir)
}

func getAgentName() string {
	if n := os.Getenv("INTERLOCK_AGENT_NAME"); n != "" {
		return n
	}
	if n := os.Getenv("INTERMUTE_AGENT_NAME"); n != "" {
		return n
	}
	return getAgentID()
}

// min returns the smaller of a and b.
// Needed for Go < 1.21; safe to keep for clarity.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Ensure main uses mcp package (for tool definitions via tools package).
var _ = mcp.NewTool
