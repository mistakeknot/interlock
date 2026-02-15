package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func jsonResponse(status int, payload any) *http.Response {
	data, _ := json.Marshal(payload)
	return &http.Response{
		StatusCode: status,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body: io.NopCloser(bytes.NewReader(data)),
	}
}

func TestSendMessageFull(t *testing.T) {
	t.Parallel()

	var received map[string]any
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/messages" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		return jsonResponse(http.StatusOK, map[string]any{"message_id": "m1"}), nil
	})

	err := c.SendMessageFull(context.Background(), "a2", "hello", MessageOptions{
		ThreadID:    "t1",
		Subject:     "release-request",
		Importance:  "urgent",
		AckRequired: true,
	})
	if err != nil {
		t.Fatalf("SendMessageFull() error = %v", err)
	}

	if got := received["thread_id"]; got != "t1" {
		t.Fatalf("thread_id = %v, want t1", got)
	}
	if got := received["subject"]; got != "release-request" {
		t.Fatalf("subject = %v, want release-request", got)
	}
	if got := received["importance"]; got != "urgent" {
		t.Fatalf("importance = %v, want urgent", got)
	}
	if got := received["ack_required"]; got != true {
		t.Fatalf("ack_required = %v, want true", got)
	}
}

func TestFetchThread(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.URL.Path, "/api/threads/") {
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		}
		return jsonResponse(http.StatusOK, map[string]any{
			"messages": []map[string]any{
				{
					"id":        "m1",
					"from":      "a1",
					"body":      `{"type":"release-request"}`,
					"subject":   "release-request",
					"thread_id": "t1",
				},
				{
					"id":        "m2",
					"from":      "a2",
					"body":      `{"type":"release-ack"}`,
					"subject":   "release-ack",
					"thread_id": "t1",
				},
			},
		}), nil
	})

	msgs, err := c.FetchThread(context.Background(), "t1")
	if err != nil {
		t.Fatalf("FetchThread() error = %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("len(msgs) = %d, want 2", len(msgs))
	}
	if msgs[0].Subject != "release-request" {
		t.Fatalf("msgs[0].Subject = %q, want release-request", msgs[0].Subject)
	}
}

func TestFetchThread_NotFound(t *testing.T) {
	t.Parallel()

	threadCalls := 0
	inboxCalls := 0
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/threads/"):
			threadCalls++
			return jsonResponse(http.StatusNotFound, map[string]any{
				"error": "not found",
			}), nil
		case r.URL.Path == "/api/inbox/a1":
			inboxCalls++
			return jsonResponse(http.StatusOK, map[string]any{
				"messages": []map[string]any{
					{
						"id":        "m1",
						"from":      "a2",
						"body":      `{"type":"release-ack"}`,
						"thread_id": "t1",
					},
					{
						"id":        "m2",
						"from":      "a3",
						"body":      `{"type":"release-request"}`,
						"thread_id": "t2",
					},
				},
				"next_cursor": "",
			}), nil
		default:
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		}
	})

	msgs, err := c.FetchThread(context.Background(), "t1")
	if err != nil {
		t.Fatalf("FetchThread() error = %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("len(msgs) = %d, want 1", len(msgs))
	}
	if msgs[0].ThreadID != "t1" {
		t.Fatalf("msgs[0].ThreadID = %q, want t1", msgs[0].ThreadID)
	}
	if threadCalls == 0 {
		t.Fatal("expected thread endpoint to be called before fallback")
	}
	if inboxCalls == 0 {
		t.Fatal("expected inbox fallback to be called")
	}
}

func TestFetchThread_EmptyMessages(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if !strings.HasPrefix(r.URL.Path, "/api/threads/") {
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		}
		return jsonResponse(http.StatusOK, map[string]any{}), nil
	})

	msgs, err := c.FetchThread(context.Background(), "t1")
	if err != nil {
		t.Fatalf("FetchThread() error = %v", err)
	}
	if msgs == nil {
		t.Fatal("FetchThread() returned nil slice, want empty slice")
	}
	if len(msgs) != 0 {
		t.Fatalf("len(msgs) = %d, want 0", len(msgs))
	}
}

func TestReleaseByPattern_Idempotent(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("holder-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/reservations":
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []any{},
			}), nil
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/reservations/"):
			deleteCalls++
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		default:
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		}
	})

	released, err := c.ReleaseByPattern(context.Background(), "holder-1", "internal/*")
	if err != nil {
		t.Fatalf("ReleaseByPattern() error = %v", err)
	}
	if released != 0 {
		t.Fatalf("released = %d, want 0", released)
	}
	if deleteCalls != 0 {
		t.Fatalf("deleteCalls = %d, want 0", deleteCalls)
	}
}
