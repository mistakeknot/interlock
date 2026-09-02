package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interbase/go/mcputil"
	"github.com/mistakeknot/interlock/internal/client"
	"github.com/mistakeknot/interlock/internal/tools"
)

// version is the release version; set at build time with
//
//	go build -ldflags "-X main.version=x.y.z" ./cmd/interlock-mcp
var version = "0.2.19"

func main() {
	c := client.NewClient(
		client.WithSocketPath(os.Getenv("INTERMUTE_SOCKET")),
		client.WithBaseURL(os.Getenv("INTERMUTE_URL")),
		client.WithAgentID(getAgentID()),
		client.WithProject(getProject()),
		client.WithAgentName(getAgentName()),
	)

	// Register with intermute so this agent is listable and so every later
	// request carries the token intermute bound to its ID. A raw-MCP install
	// has no session hook to do this for it. Registration failing must not
	// stop the server: with intermute down the tools still answer, they just
	// report the coordination loss.
	registerSelf(c)

	metrics := mcputil.NewMetrics()
	s := server.NewMCPServer(
		"interlock",
		version,
		server.WithToolCapabilities(true),
		server.WithToolHandlerMiddleware(metrics.Instrument()),
	)

	tools.RegisterAll(s, c)

	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "interlock-mcp: %v\n", err)
		os.Exit(1)
	}
}

// registerSelf registers this process as an agent unless the environment
// already carries an intermute-issued ID (the session-start hook path), in
// which case that identity is kept as is.
func registerSelf(c *client.Client) {
	if os.Getenv("INTERLOCK_AGENT_ID") != "" || os.Getenv("INTERMUTE_AGENT_ID") != "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	agent, err := c.RegisterAgent(ctx)
	if err != nil {
		log.Printf("interlock-mcp: registration skipped: %v", err)
		return
	}
	log.Printf("interlock-mcp: registered as %s (%s)", agent.AgentID, agent.Name)
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
