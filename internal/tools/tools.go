// Package tools defines the MCP tools that wrap the intermute HTTP API.
package tools

import (
	"context"
	crand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/mistakeknot/interbase/mcputil"
	"github.com/mistakeknot/interbase/toolerror"
	"github.com/mistakeknot/interlock/internal/client"
)

// Use client-exported constants to avoid duplication.
const (
	normalTimeoutMinutes    = client.NormalTimeoutMinutes
	urgentTimeoutMinutes    = client.UrgentTimeoutMinutes
	negotiationPollInterval = client.NegotiationPollInterval
)

// RegisterAll registers all 16 MCP tools with the server.
func RegisterAll(s *server.MCPServer, c *client.Client) {
	s.AddTools(
		reserveFiles(c),
		releaseFiles(c),
		releaseAll(c),
		checkConflicts(c),
		myReservations(c),
		sendMessage(c),
		broadcastMessage(c),
		fetchInbox(c),
		listTopicMessages(c),
		listAgents(c),
		requestRelease(c),
		negotiateRelease(c),
		respondToRelease(c),
		forceReleaseNegotiation(c),
		setContactPolicy(c),
		getContactPolicy(c),
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
				return mcputil.ValidationError("patterns is required")
			}

			type resError struct {
				Pattern string `json:"pattern"`
				Error   string `json:"error"`
				Type    string `json:"type"`
			}
			type result struct {
				Reservations []any      `json:"reservations"`
				Errors       []resError `json:"errors,omitempty"`
			}
			var res result
			for _, p := range patterns {
				r, err := c.CreateReservation(ctx, p, reason, ttl, exclusive)
				if err != nil {
					te := toolerror.Wrap(err)
					var ce *client.ConflictError
					if errors.As(err, &ce) {
						te = toolerror.New(toolerror.ErrConflict, "%s: conflict with %v", p, ce.Conflicts).WithRecoverable(true)
					}
					res.Errors = append(res.Errors, resError{Pattern: p, Error: te.Message, Type: te.Type})
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
				return mcputil.ValidationError("reservation_ids is required")
			}

			type releaseError struct {
				ID    string `json:"id"`
				Error string `json:"error"`
				Type  string `json:"type"`
			}
			type result struct {
				Released []string       `json:"released"`
				Errors   []releaseError `json:"errors,omitempty"`
			}
			var res result
			for _, id := range ids {
				if err := c.DeleteReservation(ctx, id); err != nil {
					te := toolerror.Wrap(err)
					res.Errors = append(res.Errors, releaseError{ID: id, Error: te.Message, Type: te.Type})
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
				return toToolError(err), nil
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
				return mcputil.ValidationError("patterns is required")
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
					return toToolError(err), nil
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
				return toToolError(err), nil
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
			mcp.WithString("topic",
				mcp.Description("Optional topic for cross-cutting discovery (e.g., 'build', 'review', 'deploy'). Lowercased at write time."),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			to, _ := args["to"].(string)
			body, _ := args["body"].(string)
			topic, _ := args["topic"].(string)
			if to == "" || body == "" {
				return mcputil.ValidationError("to and body are required")
			}
			opts := client.MessageOptions{Topic: topic}
			if err := c.SendMessageFull(ctx, to, body, opts); err != nil {
				return toToolError(err), nil
			}
			emitSignal("message", fmt.Sprintf("sent message to %s", to))
			return jsonResult(map[string]any{"sent": true, "to": to})
		},
	}
}

func broadcastMessage(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("broadcast_message",
			mcp.WithDescription("Send a message to ALL agents in the project. "+
				"Recipients are resolved server-side; agents with block_all or contacts_only "+
				"(if sender is not in contacts) are excluded. Rate limited to 10/min."),
			mcp.WithString("topic",
				mcp.Description("Topic tag for the broadcast (required). Lowercased at write time."),
				mcp.Required(),
			),
			mcp.WithString("body",
				mcp.Description("Message body"),
				mcp.Required(),
			),
			mcp.WithString("subject",
				mcp.Description("Optional subject line"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			topic, _ := args["topic"].(string)
			body, _ := args["body"].(string)
			subject, _ := args["subject"].(string)
			if topic == "" || body == "" {
				return mcputil.ValidationError("topic and body are required")
			}
			result, err := c.BroadcastMessage(ctx, topic, body, subject)
			if err != nil {
				return toToolError(err), nil
			}
			emitSignal("message", fmt.Sprintf("broadcast to %d agent(s) on topic %q", result.Delivered, topic))
			return jsonResult(map[string]any{
				"message_id": result.MessageID,
				"delivered":  result.Delivered,
				"denied":     result.Denied,
			})
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
				return toToolError(err), nil
			}
			timeouts, timeoutErr := c.CheckExpiredNegotiations(ctx)
			if messages == nil {
				messages = make([]client.Message, 0)
			}
			if len(messages) > 0 {
				emitSignal("message", fmt.Sprintf("received %d message(s)", len(messages)))
			}
			result := map[string]any{
				"messages":    messages,
				"next_cursor": nextCursor,
			}
			if timeoutErr != nil {
				result["negotiation_timeout_error"] = timeoutErr.Error()
			} else if len(timeouts) > 0 {
				result["negotiation_timeouts"] = timeouts
			}
			return jsonResult(result)
		},
	}
}

func requestRelease(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("request_release",
			mcp.WithDescription("[DEPRECATED: use negotiate_release] Ask another agent to release their file reservation."),
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
				return mcputil.ValidationError("agent_name, pattern, and reason are required")
			}
			body, _ := json.Marshal(map[string]string{
				"type":      "release-request",
				"pattern":   pattern,
				"reason":    reason,
				"requester": c.AgentName(),
			})
			if err := c.SendMessage(ctx, agentName, string(body)); err != nil {
				return toToolError(err), nil
			}
			return jsonResult(map[string]any{
				"sent": true,
				"to":   agentName,
				"type": "release-request",
			})
		},
	}
}

func negotiateRelease(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("negotiate_release",
			mcp.WithDescription("Request another agent to release their file reservation, with urgency and optional blocking wait."),
			mcp.WithString("agent_name",
				mcp.Description("Name or ID of the agent holding the reservation"),
				mcp.Required(),
			),
			mcp.WithString("file",
				mcp.Description("The file pattern you need released"),
				mcp.Required(),
			),
			mcp.WithString("reason",
				mcp.Description("Why you need the file"),
				mcp.Required(),
			),
			mcp.WithString("urgency",
				mcp.Description(fmt.Sprintf("Urgency level: 'normal' (%d minute timeout) or 'urgent' (%d minute timeout). Default: normal", normalTimeoutMinutes, urgentTimeoutMinutes)),
			),
			mcp.WithNumber("wait_seconds",
				mcp.Description("If >0, poll the thread for a response up to this many seconds"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			agentName, _ := args["agent_name"].(string)
			file, _ := args["file"].(string)
			reason, _ := args["reason"].(string)
			urgency := stringOr(args["urgency"], "normal")
			waitSeconds := intOr(args["wait_seconds"], 0)

			if agentName == "" || file == "" || reason == "" {
				return mcputil.ValidationError("agent_name, file, and reason are required")
			}
			if urgency != "normal" && urgency != "urgent" {
				return mcputil.ValidationError("urgency must be 'normal' or 'urgent'")
			}

			conflicts, err := c.CheckConflicts(ctx, file)
			if err != nil {
				return toToolError(err), nil
			}

			holderID := ""
			for _, conflict := range conflicts {
				if conflict.AgentID == agentName || conflict.HeldBy == agentName {
					holderID = conflict.AgentID
					break
				}
			}
			if holderID == "" {
				return mcputil.NotFoundError("agent %q does not hold a reservation matching %q", agentName, file)
			}

			threadID := generateNegotiateID()
			if threadID == "" {
				return mcputil.WrapError(errors.New("failed to generate negotiation thread ID"))
			}
			bodyBytes, err := json.Marshal(map[string]any{
				"type":      "release-request",
				"file":      file,
				"reason":    reason,
				"requester": c.AgentName(),
				"urgency":   urgency,
				"thread_id": threadID,
			})
			if err != nil {
				return mcputil.WrapError(fmt.Errorf("marshal release request: %w", err))
			}

			importance := "normal"
			ackRequired := false
			if urgency == "urgent" {
				importance = "urgent"
				ackRequired = true
			}

			if err := c.SendMessageFull(ctx, holderID, string(bodyBytes), client.MessageOptions{
				ThreadID:    threadID,
				Subject:     "release-request",
				Importance:  importance,
				AckRequired: ackRequired,
			}); err != nil {
				return toToolError(err), nil
			}

			if waitSeconds <= 0 {
				return jsonResult(map[string]any{
					"status":    "pending",
					"thread_id": threadID,
					"to":        holderID,
					"urgency":   urgency,
				})
			}

			deadline := time.Now().Add(time.Duration(waitSeconds) * time.Second)
			consecutiveErrors := 0
			const maxConsecutiveErrors = 3
			var lastPollErr error
			for time.Now().Before(deadline) {
				status, payload, pollErr := pollNegotiationThread(ctx, c, threadID)
				if pollErr != nil {
					consecutiveErrors++
					lastPollErr = pollErr
					if consecutiveErrors >= maxConsecutiveErrors {
						return mcputil.TransientError("poll thread %q: %d consecutive errors, last: %v", threadID, consecutiveErrors, lastPollErr)
					}
				} else {
					consecutiveErrors = 0
					if status != "" {
						result := map[string]any{
							"status":    status,
							"thread_id": threadID,
						}
						for k, v := range payload {
							result[k] = v
						}
						return jsonResult(result)
					}
				}

				remaining := time.Until(deadline)
				if remaining <= 0 {
					break
				}
				sleepFor := negotiationPollInterval
				if remaining < sleepFor {
					sleepFor = remaining
				}
				time.Sleep(sleepFor)
			}

			// Final check to avoid lost wakeups near the deadline.
			status, payload, err := pollNegotiationThread(ctx, c, threadID)
			if err != nil && consecutiveErrors+1 >= maxConsecutiveErrors {
				return mcputil.TransientError("final poll thread %q: %v", threadID, err)
			}
			if status != "" {
				result := map[string]any{
					"status":    status,
					"thread_id": threadID,
				}
				for k, v := range payload {
					result[k] = v
				}
				return jsonResult(result)
			}

			return jsonResult(map[string]any{
				"status":          "timeout",
				"thread_id":       threadID,
				"waited":          waitSeconds,
				"can_escalate":    true,
				"escalation_hint": "Call force_release_negotiation with this thread_id to force-release the reservation",
			})
		},
	}
}

func respondToRelease(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("respond_to_release",
			mcp.WithDescription("Respond to a release negotiation by releasing now or deferring with an ETA."),
			mcp.WithString("thread_id",
				mcp.Description("Negotiation thread ID"),
				mcp.Required(),
			),
			mcp.WithString("requester",
				mcp.Description("Requester agent ID"),
				mcp.Required(),
			),
			mcp.WithString("action",
				mcp.Description("Response action: 'release' or 'defer'"),
				mcp.Required(),
			),
			mcp.WithString("file",
				mcp.Description("The file pattern being negotiated"),
				mcp.Required(),
			),
			mcp.WithNumber("eta_minutes",
				mcp.Description("For defer only: estimated minutes (max 60)"),
			),
			mcp.WithString("reason",
				mcp.Description("For defer only: why you need more time"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			threadID, _ := args["thread_id"].(string)
			requester, _ := args["requester"].(string)
			action, _ := args["action"].(string)
			file, _ := args["file"].(string)
			etaMinutes := intOr(args["eta_minutes"], 0)
			reason, _ := args["reason"].(string)

			if threadID == "" || requester == "" || action == "" || file == "" {
				return mcputil.ValidationError("thread_id, requester, action, and file are required")
			}
			if action != "release" && action != "defer" {
				return mcputil.ValidationError("action must be 'release' or 'defer'")
			}

			if action == "release" {
				released, err := c.ReleaseByPattern(ctx, c.AgentID(), file)
				if err != nil {
					return nil, fmt.Errorf("release reservations by pattern: %w", err)
				}

				bodyBytes, err := json.Marshal(map[string]any{
					"type":         "release-ack",
					"file":         file,
					"released":     true,
					"released_by":  c.AgentName(),
					"released_cnt": released,
				})
				if err != nil {
					return nil, fmt.Errorf("marshal release ack: %w", err)
				}
				if err := c.SendMessageFull(ctx, requester, string(bodyBytes), client.MessageOptions{
					ThreadID: threadID,
					Subject:  "release-ack",
				}); err != nil {
					return nil, fmt.Errorf("send release ack: %w", err)
				}

				return jsonResult(map[string]any{
					"action":    "release",
					"thread_id": threadID,
					"file":      file,
					"released":  released,
				})
			}

			if etaMinutes < 0 {
				etaMinutes = 0
			}
			if etaMinutes > 60 {
				etaMinutes = 60
			}

			bodyBytes, err := json.Marshal(map[string]any{
				"type":        "release-defer",
				"file":        file,
				"eta_minutes": etaMinutes,
				"reason":      reason,
				"released":    false,
			})
			if err != nil {
				return nil, fmt.Errorf("marshal release defer: %w", err)
			}
			if err := c.SendMessageFull(ctx, requester, string(bodyBytes), client.MessageOptions{
				ThreadID: threadID,
				Subject:  "release-defer",
			}); err != nil {
				return nil, fmt.Errorf("send release defer: %w", err)
			}

			return jsonResult(map[string]any{
				"action":      "defer",
				"thread_id":   threadID,
				"file":        file,
				"eta_minutes": etaMinutes,
				"reason":      reason,
			})
		},
	}
}

func forceReleaseNegotiation(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("force_release_negotiation",
			mcp.WithDescription("Force-release a reservation after a negotiation has timed out. Use when negotiate_release returns status 'timeout' and you need the file. Validates that the timeout window has elapsed and no response was received."),
			mcp.WithString("thread_id",
				mcp.Description("Negotiation thread ID from the timed-out negotiate_release call"),
				mcp.Required(),
			),
			mcp.WithString("file",
				mcp.Description("The file pattern being negotiated"),
				mcp.Required(),
			),
			mcp.WithString("reason",
				mcp.Description("Why you are force-releasing (audit trail)"),
				mcp.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			threadID, _ := args["thread_id"].(string)
			file, _ := args["file"].(string)
			reason, _ := args["reason"].(string)

			if threadID == "" || file == "" || reason == "" {
				return mcputil.ValidationError("thread_id, file, and reason are required")
			}

			result, err := c.ForceReleaseNegotiation(ctx, threadID, file, reason)
			if err != nil {
				return toToolError(err), nil
			}

			// Only send release-ack if we actually released something.
			// Sending acks when released == 0 causes thread spam (correctness review C1).
			if result.Released > 0 {
				ackBody, _ := json.Marshal(map[string]any{
					"type":         "release-ack",
					"file":         file,
					"released":     true,
					"forced":       true,
					"reason":       "escalation-timeout",
					"released_by":  c.AgentName(),
					"released_cnt": result.Released,
				})
				_ = c.SendMessageFull(ctx, result.HolderID, string(ackBody), client.MessageOptions{
					ThreadID:   threadID,
					Subject:    "release-ack",
					Importance: "urgent",
				})
			}

			status := "force_released"
			if result.Released == 0 {
				status = "already_released"
			}

			emitSignal("escalation", fmt.Sprintf("force-released %s from %s (thread %s, released=%d)", file, result.HolderID, threadID, result.Released))

			return jsonResult(map[string]any{
				"status":    status,
				"thread_id": threadID,
				"file":      file,
				"holder":    result.HolderID,
				"released":  result.Released,
				"reason":    reason,
			})
		},
	}
}

// --- Agent tools ---

func listAgents(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("list_agents",
			mcp.WithDescription("List agents registered with intermute. Optionally filter by capability tag (e.g. 'review:architecture'). Comma-separated capabilities use OR matching."),
			mcp.WithString("capability",
				mcp.Description("Capability tag to filter by (e.g. 'review:architecture'). Comma-separated for OR matching. Omit to list all agents."),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			capability, _ := args["capability"].(string)
			var agents []client.Agent
			var err error
			if capability != "" {
				var caps []string
				for _, c := range strings.Split(capability, ",") {
					if c = strings.TrimSpace(c); c != "" {
						caps = append(caps, c)
					}
				}
				agents, err = c.DiscoverAgents(ctx, caps)
			} else {
				agents, err = c.ListAgents(ctx)
			}
			if err != nil {
				return toToolError(err), nil
			}
			if agents == nil {
				agents = make([]client.Agent, 0)
			}
			return jsonResult(agents)
		},
	}
}

// --- Helpers ---

// toToolError converts a client error to a structured ToolError MCP result.
// It maps IntermuteError HTTP codes and ConflictError to the appropriate ToolError types.
func toToolError(err error) *mcp.CallToolResult {
	// ConflictError → ErrConflict (recoverable — agent can retry after release)
	var ce *client.ConflictError
	if errors.As(err, &ce) {
		te := toolerror.New(toolerror.ErrConflict, "%v", ce).WithRecoverable(true)
		te.Data = map[string]any{"conflicts": ce.Conflicts}
		return mcp.NewToolResultError(te.JSON())
	}

	// IntermuteError → map HTTP status to error type
	var ie *client.IntermuteError
	if errors.As(err, &ie) {
		switch {
		case ie.Code == 404:
			return mcp.NewToolResultError(toolerror.New(toolerror.ErrNotFound, "%s", ie.Message).JSON())
		case ie.Code == 403:
			return mcp.NewToolResultError(toolerror.New(toolerror.ErrPermission, "%s", ie.Message).JSON())
		case ie.Code == 422 || ie.Code == 400:
			return mcp.NewToolResultError(toolerror.New(toolerror.ErrValidation, "%s", ie.Message).JSON())
		case ie.Code == 429 || ie.Code >= 500:
			te := toolerror.New(toolerror.ErrTransient, "%s", ie.Message)
			if ie.RetryAfter > 0 {
				te.Data = map[string]any{"retry_after": ie.RetryAfter}
			}
			return mcp.NewToolResultError(te.JSON())
		default:
			return mcp.NewToolResultError(toolerror.Wrap(ie).JSON())
		}
	}

	// Connection errors → ErrTransient
	if isConnError(err) {
		return mcp.NewToolResultError(toolerror.New(toolerror.ErrTransient, "intermute unavailable: %v", err).JSON())
	}

	// Everything else → ErrInternal
	return mcp.NewToolResultError(toolerror.Wrap(err).JSON())
}

// isConnError returns true if err contains a network connection error.
func isConnError(err error) bool {
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}
	return strings.Contains(err.Error(), "intermute unavailable")
}

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

func stringOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

var negotiateIDCounter atomic.Uint64

func generateNegotiateID() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		count := negotiateIDCounter.Add(1)
		return fmt.Sprintf("negotiate-fallback-%d-%d-%d", time.Now().UnixNano(), os.Getpid(), count)
	}
	return fmt.Sprintf("negotiate-%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

func pollNegotiationThread(ctx context.Context, c *client.Client, threadID string) (string, map[string]any, error) {
	messages, err := c.FetchThread(ctx, threadID)
	if err != nil {
		return "", nil, fmt.Errorf("fetch thread: %w", err)
	}
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		msgType := msg.Subject

		var body map[string]any
		if err := json.Unmarshal([]byte(msg.Body), &body); err == nil {
			if t := stringOr(body["type"], ""); t != "" {
				msgType = t
			}
		}

		switch msgType {
		case "release-ack":
			return "released", map[string]any{
				"released_by": stringOr(body["released_by"], msg.From),
				"reason":      stringOr(body["reason"], ""),
			}, nil
		case "release-defer":
			return "deferred", map[string]any{
				"eta_minutes": intOr(body["eta_minutes"], 0),
				"reason":      stringOr(body["reason"], ""),
			}, nil
		}
	}
	return "", nil, nil
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

// --- Topic discovery tools ---

func listTopicMessages(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("list_topic_messages",
			mcp.WithDescription("List messages by topic for cross-cutting discovery. Allows late-joining or oversight agents to find conversations without being original recipients."),
			mcp.WithString("topic",
				mcp.Description("Topic to search for (e.g., 'build', 'review', 'deploy'). Case-insensitive."),
				mcp.Required(),
			),
			mcp.WithString("since_cursor",
				mcp.Description("Pagination cursor — only return messages after this cursor"),
			),
			mcp.WithNumber("limit",
				mcp.Description("Maximum number of messages to return (default: 100, max: 1000)"),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			topic, _ := args["topic"].(string)
			if topic == "" {
				return mcputil.ValidationError("topic is required")
			}
			sinceCursor, _ := args["since_cursor"].(string)
			limit := 0
			if v, ok := args["limit"].(float64); ok {
				limit = int(v)
			}
			messages, err := c.TopicMessages(ctx, topic, sinceCursor, limit)
			if err != nil {
				return toToolError(err), nil
			}
			if messages == nil {
				messages = make([]client.Message, 0)
			}
			return jsonResult(map[string]any{
				"topic":    topic,
				"messages": messages,
				"count":    len(messages),
			})
		},
	}
}

// --- Contact policy tools ---

func setContactPolicy(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("set_contact_policy",
			mcp.WithDescription("Set your agent's contact policy controlling who can send you messages. Levels: open (anyone, default), auto (agents with overlapping file reservations), contacts_only (explicit whitelist), block_all (reject everything). Idempotent: yes."),
			mcp.WithString("policy",
				mcp.Description("Policy level: open, auto, contacts_only, or block_all"),
				mcp.Required(),
			),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			args := req.GetArguments()
			policy, _ := args["policy"].(string)
			if policy == "" {
				return mcputil.ValidationError("policy is required")
			}
			switch policy {
			case "open", "auto", "contacts_only", "block_all":
				// valid
			default:
				return mcputil.ValidationError("policy must be open, auto, contacts_only, or block_all")
			}
			if err := c.SetContactPolicy(ctx, policy); err != nil {
				return toToolError(err), nil
			}
			return jsonResult(map[string]any{"policy": policy, "updated": true})
		},
	}
}

func getContactPolicy(c *client.Client) server.ServerTool {
	return server.ServerTool{
		Tool: mcp.NewTool("get_contact_policy",
			mcp.WithDescription("Get your agent's current contact policy. Returns the policy level controlling who can send you messages. Idempotent: yes."),
		),
		Handler: func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			policy, err := c.GetContactPolicy(ctx)
			if err != nil {
				return toToolError(err), nil
			}
			return jsonResult(map[string]any{"policy": policy})
		},
	}
}
