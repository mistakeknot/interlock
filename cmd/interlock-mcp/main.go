package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
var version = "0.2.20"

// handleVersionFlag answers --version/-v and rejects other flags so that
// `interlock-mcp --version` prints and exits instead of starting a server that
// registers itself with intermute (issue #7).
func handleVersionFlag(args []string, out io.Writer, errOut io.Writer) (exitCode int, handled bool) {
	for _, a := range args {
		switch a {
		case "--version", "-v", "version":
			fmt.Fprintf(out, "interlock-mcp %s\n", version)
			return 0, true
		default:
			if strings.HasPrefix(a, "-") {
				fmt.Fprintf(errOut, "interlock-mcp: unknown flag %q (only --version is accepted; the server takes no arguments)\n", a)
				return 2, true
			}
		}
	}
	return 0, false
}

func main() {
	if code, handled := handleVersionFlag(os.Args[1:], os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
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
	// One identity per session (issue #4): if the session-start hook already
	// registered this name, adopt that row instead of creating a second one.
	if adopted, err := c.AdoptAgentByName(ctx); err != nil {
		log.Printf("interlock-mcp: adoption lookup failed, registering fresh: %v", err)
	} else if adopted != nil {
		log.Printf("interlock-mcp: adopted existing agent %s (%s)", adopted.AgentID, adopted.Name)
		return
	}
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
	// Inside tmux, the pane title is what the session-start hook falls back
	// to as well, so both halves of a session land on the same name (issue #4).
	if os.Getenv("TMUX") != "" {
		if out, err := exec.Command("tmux", "display-message", "-p", "#T").Output(); err == nil {
			if t := strings.TrimSpace(string(out)); t != "" {
				return t
			}
		}
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
