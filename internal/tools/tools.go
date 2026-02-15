// Package tools defines the 9 MCP tools that wrap the intermute HTTP API.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interlock/internal/client"
)

// RegisterAll registers all 9 MCP tools with the server.
func RegisterAll(s *server.MCPServer, c *client.Client) {
	s.AddTools(
		reserveFiles(c),
		releaseFiles(c),
		releaseAll(c),
		checkConflicts(c),
		myReservations(c),
		sendMessage(c),
		fetchInbox(c),
		listAgents(c),
		requestRelease(c),
	)
}

// --- Reserve tools ---

func reserveFiles(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("reserve_files",
			mcp.WithDescription("Reserve file patterns before editing. Prevents other agents from modifying these files."),
			mcp.WithArray("patterns",
				mcp.Description("Glob patterns for files to reserve (e.g. 'src/router.go', 'internal/http/*.go')"),
				mcp.Required(),
				mcp.WithStringItems(),
			),
			mcp.WithString("reason",
				mcp.Description("Why you're reserving these files"),
				mcp.Required(),
			),
			mcp.WithNumber("ttl_minutes",
				mcp.Description("Reservation duration in minutes (default: 15)"),
			),
			mcp.WithBoolean("exclusive",
				mcp.Description("Whether the reservation is exclusive (default: true)"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			patterns := toStringSlice(args["patterns"])
			reason, _ := args["reason"].(string)
			ttl := intOr(args["ttl_minutes"], 15)
			exclusive := boolOr(args["exclusive"], true)

			if len(patterns) == 0 {
				return mcp.NewToolResultError("patterns is required"), nil
			}

			type result struct {
				Reservations []any    `json:"reservations"`
				Errors       []string `json:"errors,omitempty"`
			}
			var res result
			for _, p := range patterns {
				r, err := c.CreateReservation(ctx, p, reason, ttl, exclusive)
				if err != nil {
					if ce, ok := err.(*client.ConflictError); ok {
						res.Errors = append(res.Errors, fmt.Sprintf("%s: conflict with %v", p, ce.Conflicts))
					} else {
						res.Errors = append(res.Errors, fmt.Sprintf("%s: %v", p, err))
					}
					continue
				}
				res.Reservations = append(res.Reservations, r)
				emitSignal("reserve", fmt.Sprintf("reserved %s", p))
			}
			return jsonResult(res)
		},
	}
}

func releaseFiles(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("release_files",
			mcp.WithDescription("Release specific file reservations by reservation ID."),
			mcp.WithArray("reservation_ids",
				mcp.Description("IDs of reservations to release"),
				mcp.Required(),
				mcp.WithStringItems(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			ids := toStringSlice(args["reservation_ids"])
			if len(ids) == 0 {
				return mcp.NewToolResultError("reservation_ids is required"), nil
			}

			type result struct {
				Released []string `json:"released"`
				Errors   []any    `json:"errors,omitempty"`
			}
			var res result
			for _, id := range ids {
				if err := c.DeleteReservation(ctx, id); err != nil {
					res.Errors = append(res.Errors, map[string]string{"id": id, "error": err.Error()})
				} else {
					res.Released = append(res.Released, id)
					emitSignal("release", fmt.Sprintf("released %s", id))
				}
			}
			return jsonResult(res)
		},
	}
}

func releaseAll(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("release_all",
			mcp.WithDescription("Release all your active reservations at once."),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			reservations, err := c.ListReservations(ctx, map[string]string{
				"agent":   c.AgentID(),
				"project": c.Project(),
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("list reservations: %v", err)), nil
			}

			count := 0
			for _, r := range reservations {
				if r.IsActive {
					if err := c.DeleteReservation(ctx, r.ID); err == nil {
						count++
					}
				}
			}
			emitSignal("release", fmt.Sprintf("released all (%d reservations)", count))
			return jsonResult(map[string]int{"released_count": count})
		},
	}
}

// --- Conflict tools ---

func checkConflicts(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("check_conflicts",
			mcp.WithDescription("Check if file patterns would conflict with existing reservations (dry run, does not create reservations)."),
			mcp.WithArray("patterns",
				mcp.Description("Glob patterns to check for conflicts"),
				mcp.Required(),
				mcp.WithStringItems(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			patterns := toStringSlice(args["patterns"])
			if len(patterns) == 0 {
				return mcp.NewToolResultError("patterns is required"), nil
			}

			type result struct {
				Conflicts []any    `json:"conflicts"`
				Clear     []string `json:"clear"`
			}
			var res result
			res.Conflicts = make([]any, 0)
			res.Clear = make([]string, 0)
			for _, p := range patterns {
				conflicts, err := c.CheckConflicts(ctx, p)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("check %s: %v", p, err)), nil
				}
				if len(conflicts) > 0 {
					for _, cd := range conflicts {
						res.Conflicts = append(res.Conflicts, cd)
					}
				} else {
					res.Clear = append(res.Clear, p)
				}
			}
			return jsonResult(res)
		},
	}
}

func myReservations(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("my_reservations",
			mcp.WithDescription("List your current active reservations."),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			reservations, err := c.ListReservations(ctx, map[string]string{
				"agent":   c.AgentID(),
				"project": c.Project(),
			})
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("list reservations: %v", err)), nil
			}
			if reservations == nil {
				reservations = make([]client.Reservation, 0)
			}
			return jsonResult(reservations)
		},
	}
}

// --- Messaging tools ---

func sendMessage(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("send_message",
			mcp.WithDescription("Send a message to another agent in the project."),
			mcp.WithString("to",
				mcp.Description("Agent ID or name to send to"),
				mcp.Required(),
			),
			mcp.WithString("body",
				mcp.Description("Message body"),
				mcp.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			to, _ := args["to"].(string)
			body, _ := args["body"].(string)
			if to == "" || body == "" {
				return mcp.NewToolResultError("to and body are required"), nil
			}
			if err := c.SendMessage(ctx, to, body); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("send message: %v", err)), nil
			}
			emitSignal("message", fmt.Sprintf("sent message to %s", to))
			return jsonResult(map[string]any{"sent": true, "to": to})
		},
	}
}

func fetchInbox(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("fetch_inbox",
			mcp.WithDescription("Check your inbox for messages from other agents."),
			mcp.WithString("cursor",
				mcp.Description("Pagination cursor from a previous fetch"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			cursor, _ := args["cursor"].(string)
			messages, nextCursor, err := c.FetchInbox(ctx, cursor)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("fetch inbox: %v", err)), nil
			}
			if messages == nil {
				messages = make([]client.Message, 0)
			}
			if len(messages) > 0 {
				emitSignal("message", fmt.Sprintf("received %d message(s)", len(messages)))
			}
			return jsonResult(map[string]any{
				"messages":    messages,
				"next_cursor": nextCursor,
			})
		},
	}
}

func requestRelease(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("request_release",
			mcp.WithDescription("Ask another agent to release their file reservation."),
			mcp.WithString("agent_name",
				mcp.Description("Name or ID of the agent holding the reservation"),
				mcp.Required(),
			),
			mcp.WithString("pattern",
				mcp.Description("The file pattern you need released"),
				mcp.Required(),
			),
			mcp.WithString("reason",
				mcp.Description("Why you need the files"),
				mcp.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			agentName, _ := args["agent_name"].(string)
			pattern, _ := args["pattern"].(string)
			reason, _ := args["reason"].(string)
			if agentName == "" || pattern == "" || reason == "" {
				return mcp.NewToolResultError("agent_name, pattern, and reason are required"), nil
			}
			body, _ := json.Marshal(map[string]string{
				"type":      "release-request",
				"pattern":   pattern,
				"reason":    reason,
				"requester": c.AgentName(),
			})
			if err := c.SendMessage(ctx, agentName, string(body)); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("send release request: %v", err)), nil
			}
			return jsonResult(map[string]any{
				"sent": true,
				"to":   agentName,
				"type": "release-request",
			})
		},
	}
}

// --- Agent tools ---

func listAgents(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("list_agents",
			mcp.WithDescription("List all active agents in the project."),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			agents, err := c.ListAgents(ctx)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("list agents: %v", err)), nil
			}
			if agents == nil {
				agents = make([]client.Agent, 0)
			}
			return jsonResult(agents)
		},
	}
}

// --- Helpers ---

func jsonResult(v any) (*mcp.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("marshal: %v", err)), nil
	}
	return mcp.NewToolResultText(string(data)), nil
}

func toStringSlice(v any) []string {
	if v == nil {
		return nil
	}
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func intOr(v any, def int) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return def
}

func boolOr(v any, def bool) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

// emitSignal fires the signal script in the background (fire-and-forget).
func emitSignal(eventType, text string) {
	// Look for the signal script relative to the binary
	script := findSignalScript()
	if script == "" {
		return
	}
	cmd := exec.Command("bash", script, eventType, text)
	_ = cmd.Start() // fire-and-forget
}

func findSignalScript() string {
	candidates := []string{
		"scripts/interlock-signal.sh",
		"../scripts/interlock-signal.sh",
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
		// Check if file exists
		if fi, err := exec.LookPath("bash"); err == nil {
			_ = fi
			// Just check the file directly
		}
	}
	return ""
}
