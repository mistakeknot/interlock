// Package client provides an HTTP client for the intermute coordination service.
package client

import (
	"bytes"
	"context"
	"encoding/json"
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

func WithAgentID(id string) Option    { return func(c *Client) { c.agentID = id } }
func WithProject(name string) Option  { return func(c *Client) { c.project = name } }
func WithAgentName(n string) Option   { return func(c *Client) { c.agentName = n } }

// NewClient creates a new intermute client. Socket option is applied first;
// if it succeeds, it overrides any base URL.
func NewClient(opts ...Option) *Client {
	c := &Client{
		http: &http.Client{Timeout: 10 * time.Second},
		baseURL: "http://127.0.0.1:7338",
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// AgentID returns the configured agent ID.
func (c *Client) AgentID() string  { return c.agentID }
// Project returns the configured project name.
func (c *Client) Project() string  { return c.project }
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
	AgentID string `json:"agent_id"`
	Name    string `json:"name"`
	Project string `json:"project"`
	Status  string `json:"status"`
}

// Message represents an inbox message.
type Message struct {
	ID        string `json:"message_id"`
	From      string `json:"from"`
	Body      string `json:"body"`
	Timestamp string `json:"timestamp"`
	Read      bool   `json:"read"`
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

// SendMessage sends a message to another agent.
func (c *Client) SendMessage(ctx context.Context, to, body string) error {
	msg := map[string]any{
		"project": c.project,
		"from":    c.agentID,
		"to":      []string{to},
		"body":    body,
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
		if patternsOverlap(r.PathPattern, pattern) {
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

// patternsOverlap does a simple prefix/glob overlap check.
func patternsOverlap(existing, candidate string) bool {
	e := strings.TrimSuffix(existing, "*")
	c := strings.TrimSuffix(candidate, "*")
	return strings.HasPrefix(e, c) || strings.HasPrefix(c, e)
}

func isNotFound(err error) bool {
	var ie *IntermuteError
	if ok := false; err != nil {
		switch e := err.(type) {
		case *IntermuteError:
			ie = e
			ok = true
		default:
			_ = ok
		}
	}
	return ie != nil && ie.Code == 404
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
