// Package client provides an HTTP client for the intermute coordination service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client communicates with intermute via HTTP (Unix socket or TCP).
type Client struct {
	http      *http.Client
	baseURL   string
	agentID   string
	project   string
	agentName string
}

// Option configures a Client.
type Option func(*Client)

func WithSocketPath(path string) Option {
	return func(c *Client) {
		if path == "" || !fileExists(path) {
			return
		}
		c.http.Transport = &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				return net.DialTimeout("unix", path, 5*time.Second)
			},
		}
		c.baseURL = "http://localhost"
	}
}

func WithBaseURL(u string) Option {
	return func(c *Client) {
		if u != "" {
			c.baseURL = strings.TrimRight(u, "/")
		}
	}
}

func WithAgentID(id string) Option   { return func(c *Client) { c.agentID = id } }
func WithProject(name string) Option { return func(c *Client) { c.project = name } }
func WithAgentName(n string) Option  { return func(c *Client) { c.agentName = n } }

// NewClient creates a new intermute client. Socket option is applied first;
// if it succeeds, it overrides any base URL.
func NewClient(opts ...Option) *Client {
	c := &Client{
		http:    &http.Client{Timeout: 10 * time.Second},
		baseURL: "http://127.0.0.1:7338",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// AgentID returns the configured agent ID.
func (c *Client) AgentID() string { return c.agentID }

// Project returns the configured project name.
func (c *Client) Project() string { return c.project }

// AgentName returns the configured agent name.
func (c *Client) AgentName() string { return c.agentName }

// Reservation represents an active file reservation.
type Reservation struct {
	ID          string `json:"id"`
	AgentID     string `json:"agent_id"`
	Project     string `json:"project"`
	PathPattern string `json:"path_pattern"`
	Exclusive   bool   `json:"exclusive"`
	Reason      string `json:"reason"`
	ExpiresAt   string `json:"expires_at"`
	IsActive    bool   `json:"is_active"`
}

// ConflictDetail describes a single conflict.
type ConflictDetail struct {
	ReservationID string `json:"reservation_id"`
	AgentID       string `json:"agent_id"`
	HeldBy        string `json:"held_by"`
	Pattern       string `json:"pattern"`
	Reason        string `json:"reason"`
	ExpiresAt     string `json:"expires_at"`
}

// Agent represents a registered agent.
type Agent struct {
	AgentID      string   `json:"agent_id"`
	Name         string   `json:"name"`
	Project      string   `json:"project"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
	LastSeen     string   `json:"last_seen"`
}

// Message represents an inbox message.
type Message struct {
	ID          string   `json:"id,omitempty"`
	MessageID   string   `json:"message_id,omitempty"`
	From        string   `json:"from,omitempty"`
	To          []string `json:"to,omitempty"`
	Body        string   `json:"body,omitempty"`
	ThreadID    string   `json:"thread_id,omitempty"`
	Subject     string   `json:"subject,omitempty"`
	Importance  string   `json:"importance,omitempty"`
	AckRequired bool     `json:"ack_required,omitempty"`
	Timestamp   string   `json:"timestamp,omitempty"`
	CreatedAt   string   `json:"created_at,omitempty"`
	Read        bool     `json:"read,omitempty"`
}

// MessageOptions provides optional fields for SendMessageFull.
type MessageOptions struct {
	ThreadID    string
	Subject     string
	Importance  string
	AckRequired bool
}

// NegotiationTimeout describes an expired negotiation that was auto-enforced.
type NegotiationTimeout struct {
	ThreadID   string `json:"thread_id"`
	File       string `json:"file"`
	Holder     string `json:"holder"`
	Urgency    string `json:"urgency"`
	AgeMinutes int    `json:"age_minutes"`
	Released   int    `json:"released"`
}

// IntermuteError is a structured error from the intermute API.
type IntermuteError struct {
	Code       int    `json:"code"`
	Message    string `json:"error"`
	RetryAfter int    `json:"retry_after"`
}

func (e *IntermuteError) Error() string {
	return fmt.Sprintf("intermute %d: %s", e.Code, e.Message)
}

// ConflictError wraps a 409 conflict response.
type ConflictError struct {
	Conflicts []ConflictDetail `json:"conflicts"`
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("reservation conflict: %d conflicts", len(e.Conflicts))
}

// --- API Methods ---

// CreateReservation reserves a file pattern.
func (c *Client) CreateReservation(ctx context.Context, pattern, reason string, ttlMinutes int, exclusive bool) (*Reservation, error) {
	body := map[string]any{
		"agent_id":     c.agentID,
		"project":      c.project,
		"path_pattern": pattern,
		"exclusive":    exclusive,
		"reason":       reason,
	}
	if ttlMinutes > 0 {
		body["ttl_minutes"] = ttlMinutes
	}
	var res Reservation
	if err := c.doJSON(ctx, "POST", "/api/reservations", body, &res); err != nil {
		return nil, err
	}
	return &res, nil
}

// DeleteReservation releases a reservation by ID.
func (c *Client) DeleteReservation(ctx context.Context, id string) error {
	return c.doJSON(ctx, "DELETE", "/api/reservations/"+id, nil, nil)
}

// ListReservations fetches reservations with optional filters.
func (c *Client) ListReservations(ctx context.Context, filters map[string]string) ([]Reservation, error) {
	q := url.Values{}
	for k, v := range filters {
		q.Set(k, v)
	}
	path := "/api/reservations?" + q.Encode()
	var result struct {
		Reservations []Reservation `json:"reservations"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Reservations, nil
}

// CheckConflicts checks if patterns conflict with existing reservations.
func (c *Client) CheckConflicts(ctx context.Context, pattern string) ([]ConflictDetail, error) {
	q := url.Values{}
	q.Set("project", c.project)
	q.Set("pattern", pattern)
	q.Set("exclusive", "true")
	path := "/api/reservations/check?" + q.Encode()
	var result struct {
		Conflicts []ConflictDetail `json:"conflicts"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		// Fallback: if endpoint not found, use list + client-side filter
		if isNotFound(err) {
			return c.checkConflictsFallback(ctx, pattern)
		}
		return nil, err
	}
	return result.Conflicts, nil
}

// RegisterAgent registers this agent with intermute.
func (c *Client) RegisterAgent(ctx context.Context) (*Agent, error) {
	body := map[string]any{
		"id":      c.agentID,
		"name":    c.agentName,
		"project": c.project,
	}
	var agent Agent
	if err := c.doJSON(ctx, "POST", "/api/agents", body, &agent); err != nil {
		return nil, err
	}
	return &agent, nil
}

// ListAgents lists agents for the configured project.
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	path := "/api/agents?project=" + url.QueryEscape(c.project)
	var result struct {
		Agents []Agent `json:"agents"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Agents, nil
}

// DiscoverAgents lists agents filtered by capability tags.
// Capabilities uses OR matching — agents with any of the given capabilities are returned.
// Pass nil or empty slice to list all agents (same as ListAgents).
func (c *Client) DiscoverAgents(ctx context.Context, capabilities []string) ([]Agent, error) {
	path := "/api/agents?project=" + url.QueryEscape(c.project)
	if len(capabilities) > 0 {
		path += "&capability=" + url.QueryEscape(strings.Join(capabilities, ","))
	}
	var result struct {
		Agents []Agent `json:"agents"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	return result.Agents, nil
}

// SendMessage sends a message to another agent.
func (c *Client) SendMessage(ctx context.Context, to, body string) error {
	return c.SendMessageFull(ctx, to, body, MessageOptions{})
}

// SendMessageFull sends a message with optional thread/priority metadata.
func (c *Client) SendMessageFull(ctx context.Context, to, body string, opts MessageOptions) error {
	msg := map[string]any{
		"project": c.project,
		"from":    c.agentID,
		"to":      []string{to},
		"body":    body,
	}
	if opts.ThreadID != "" {
		msg["thread_id"] = opts.ThreadID
	}
	if opts.Subject != "" {
		msg["subject"] = opts.Subject
	}
	if opts.Importance != "" {
		msg["importance"] = opts.Importance
	}
	if opts.AckRequired {
		msg["ack_required"] = true
	}
	return c.doJSON(ctx, "POST", "/api/messages", msg, nil)
}

// FetchInbox fetches inbox messages for this agent.
func (c *Client) FetchInbox(ctx context.Context, cursor string) ([]Message, string, error) {
	q := url.Values{}
	q.Set("project", c.project)
	if cursor != "" {
		q.Set("cursor", cursor)
	}
	path := "/api/inbox/" + url.PathEscape(c.agentID) + "?" + q.Encode()
	var result struct {
		Messages   []Message `json:"messages"`
		NextCursor string    `json:"next_cursor"`
	}
	if err := c.doJSON(ctx, "GET", path, nil, &result); err != nil {
		return nil, "", err
	}
	return result.Messages, result.NextCursor, nil
}

// FetchThread fetches all messages in a thread.
// Falls back to inbox filtering when thread endpoints are unavailable.
func (c *Client) FetchThread(ctx context.Context, threadID string) ([]Message, error) {
	if threadID == "" {
		return make([]Message, 0), nil
	}

	q := url.Values{}
	q.Set("project", c.project)
	path := "/api/threads/" + url.PathEscape(threadID) + "?" + q.Encode()
	var result struct {
		Messages []Message `json:"messages"`
	}
	err := c.doJSON(ctx, "GET", path, nil, &result)
	if err == nil {
		if result.Messages == nil {
			return make([]Message, 0), nil
		}
		return result.Messages, nil
	}
	if !isNotFound(err) {
		return nil, fmt.Errorf("fetch thread %q: %w", threadID, err)
	}

	// Fallback for older intermute versions: page through inbox and filter.
	filtered := make([]Message, 0)
	cursor := ""
	for {
		messages, nextCursor, fetchErr := c.FetchInbox(ctx, cursor)
		if fetchErr != nil {
			return nil, fmt.Errorf("fetch thread fallback %q: %w", threadID, fetchErr)
		}
		for _, msg := range messages {
			if msg.ThreadID == threadID {
				filtered = append(filtered, msg)
			}
		}
		if nextCursor == "" || nextCursor == cursor {
			break
		}
		cursor = nextCursor
	}
	return filtered, nil
}

// ReleaseByPattern releases active reservations held by agentID that overlap pattern.
// Idempotent: returns 0 when nothing matches.
func (c *Client) ReleaseByPattern(ctx context.Context, agentID, pattern string) (int, error) {
	reservations, err := c.ListReservations(ctx, map[string]string{
		"agent":   agentID,
		"project": c.project,
	})
	if err != nil {
		return 0, fmt.Errorf("list reservations for %q: %w", agentID, err)
	}

	released := 0
	for _, r := range reservations {
		if !r.IsActive || !PatternsOverlap(r.PathPattern, pattern) {
			continue
		}
		if err := c.DeleteReservation(ctx, r.ID); err != nil {
			if !isNotFound(err) {
				return released, fmt.Errorf("delete reservation %q: %w", r.ID, err)
			}
			// 404 = already deleted by another goroutine/session, count as success.
		}
		released++
	}
	return released, nil
}

// Negotiation timeout constants. Exported so tools layer can reference them
// in descriptions without duplicating magic numbers.
const (
	NormalTimeoutMinutes    = 10
	UrgentTimeoutMinutes    = 5
	NegotiationPollInterval = 2 * time.Second
)

// CheckExpiredNegotiations finds expired release requests that have no
// release-ack in their thread. Returns advisory information about
// timeout-eligible negotiations. Does NOT force-release reservations —
// the requester agent decides whether to act on timeout information.
func (c *Client) CheckExpiredNegotiations(ctx context.Context) ([]NegotiationTimeout, error) {
	messages := make([]Message, 0)
	cursor := ""
	for {
		page, nextCursor, err := c.FetchInbox(ctx, cursor)
		if err != nil {
			return nil, fmt.Errorf("fetch inbox for negotiation timeout: %w", err)
		}
		messages = append(messages, page...)
		if nextCursor == "" || nextCursor == cursor {
			break
		}
		cursor = nextCursor
	}

	now := time.Now()
	timeouts := make([]NegotiationTimeout, 0)
	for _, msg := range messages {
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Body), &payload); err != nil {
			continue
		}
		if stringOr(payload["type"], "") != "release-request" {
			continue
		}

		urgency := stringOr(payload["urgency"], msg.Importance)
		if urgency == "" {
			continue
		}
		if urgency != "urgent" && urgency != "normal" {
			urgency = "normal"
		}

		createdAt := msg.CreatedAt
		if createdAt == "" {
			createdAt = msg.Timestamp
		}
		if createdAt == "" {
			continue
		}
		requestTime, err := parseMessageTime(createdAt)
		if err != nil {
			continue
		}

		timeoutMinutes := NormalTimeoutMinutes
		if urgency == "urgent" {
			timeoutMinutes = UrgentTimeoutMinutes
		}
		age := now.Sub(requestTime)
		if age < time.Duration(timeoutMinutes)*time.Minute {
			continue
		}

		threadID := msg.ThreadID
		if threadID == "" {
			threadID = stringOr(payload["thread_id"], "")
		}
		if threadID != "" {
			threadMessages, threadErr := c.FetchThread(ctx, threadID)
			if threadErr != nil {
				return nil, fmt.Errorf("check thread %q for timeout: %w", threadID, threadErr)
			}
			if hasReleaseAck(threadMessages) {
				continue
			}
		}

		file := stringOr(payload["file"], "")
		if file == "" {
			file = stringOr(payload["pattern"], "")
		}
		if file == "" {
			continue
		}

		// Advisory only: report the timeout-eligible negotiation.
		// The requester agent can call respond_to_release to force-release
		// or the holder agent will see advisory context on next edit.
		timeouts = append(timeouts, NegotiationTimeout{
			ThreadID:   threadID,
			File:       file,
			Holder:     c.agentID,
			Urgency:    urgency,
			AgeMinutes: int(age.Minutes()),
			Released:   0,
		})
	}

	return timeouts, nil
}

// --- Internal ---

func (c *Client) doJSON(ctx context.Context, method, path string, body any, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.agentID != "" {
		req.Header.Set("X-Agent-ID", c.agentID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("intermute unavailable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		var ce struct {
			Error     string           `json:"error"`
			Conflicts []ConflictDetail `json:"conflicts"`
		}
		if json.NewDecoder(resp.Body).Decode(&ce) == nil && len(ce.Conflicts) > 0 {
			return &ConflictError{Conflicts: ce.Conflicts}
		}
		return &IntermuteError{Code: 409, Message: "reservation conflict"}
	}

	if resp.StatusCode >= 400 {
		var ie IntermuteError
		ie.Code = resp.StatusCode
		if json.NewDecoder(resp.Body).Decode(&ie) != nil {
			ie.Message = http.StatusText(resp.StatusCode)
		}
		return &ie
	}

	if out != nil {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("read response: %w", err)
		}
		if len(data) > 0 {
			if err := json.Unmarshal(data, out); err != nil {
				return fmt.Errorf("decode response: %w", err)
			}
		}
	}
	return nil
}

func (c *Client) checkConflictsFallback(ctx context.Context, pattern string) ([]ConflictDetail, error) {
	reservations, err := c.ListReservations(ctx, map[string]string{
		"project": c.project,
	})
	if err != nil {
		return nil, err
	}
	var conflicts []ConflictDetail
	for _, r := range reservations {
		if r.AgentID == c.agentID || !r.IsActive || !r.Exclusive {
			continue
		}
		if PatternsOverlap(r.PathPattern, pattern) {
			conflicts = append(conflicts, ConflictDetail{
				ReservationID: r.ID,
				AgentID:       r.AgentID,
				Pattern:       r.PathPattern,
				Reason:        r.Reason,
				ExpiresAt:     r.ExpiresAt,
			})
		}
	}
	return conflicts, nil
}

// PatternsOverlap does a simple prefix/glob overlap check.
func PatternsOverlap(existing, candidate string) bool {
	e := strings.TrimSuffix(existing, "*")
	c := strings.TrimSuffix(candidate, "*")
	return strings.HasPrefix(e, c) || strings.HasPrefix(c, e)
}

// patternsOverlap is kept as an internal alias for legacy callers.
func patternsOverlap(existing, candidate string) bool {
	return PatternsOverlap(existing, candidate)
}

func isNotFound(err error) bool {
	var ie *IntermuteError
	if errors.As(err, &ie) {
		return ie.Code == http.StatusNotFound
	}
	return false
}

func parseMessageTime(ts string) (time.Time, error) {
	if t, err := time.Parse(time.RFC3339Nano, ts); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, ts)
}

func hasReleaseAck(messages []Message) bool {
	for _, msg := range messages {
		if msg.Subject == "release-ack" {
			return true
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(msg.Body), &payload); err != nil {
			continue
		}
		if stringOr(payload["type"], "") == "release-ack" {
			return true
		}
	}
	return false
}

func stringOr(v any, def string) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return def
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
