package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
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

func TestFormatReservationReason_AppendsActiveBeadID(t *testing.T) {
	t.Parallel()

	got := FormatReservationReason("Wave 3 file edit", ReservationCorrelation{ActiveBeadID: "sylveste-kgfi.3"})

	if !strings.Contains(got, "Wave 3 file edit") {
		t.Fatalf("reason = %q, want original reason preserved", got)
	}
	if !strings.Contains(got, "active_bead_id=sylveste-kgfi.3") {
		t.Fatalf("reason = %q, want active_bead_id metadata", got)
	}
}

func TestCreateReservationWithCorrelation_SendsBeadMetadataInReason(t *testing.T) {
	t.Parallel()

	var received map[string]any
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/reservations" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		return jsonResponse(http.StatusCreated, map[string]any{
			"id":           "res-1",
			"agent_id":     "a1",
			"project":      "p1",
			"path_pattern": "internal/http/*",
			"exclusive":    true,
			"reason":       received["reason"],
			"is_active":    true,
		}), nil
	})

	res, err := c.CreateReservationWithCorrelation(context.Background(), "internal/http/*", "Wave 3 edit", 15, true, ReservationCorrelation{ActiveBeadID: "sylveste-kgfi.3"})
	if err != nil {
		t.Fatalf("CreateReservationWithCorrelation() error = %v", err)
	}
	if !strings.Contains(fmt.Sprint(received["reason"]), "active_bead_id=sylveste-kgfi.3") {
		t.Fatalf("reason payload = %v, want active_bead_id metadata", received["reason"])
	}
	if res.Reason != received["reason"] {
		t.Fatalf("response reason = %q, want payload reason %q", res.Reason, received["reason"])
	}
}

func TestCollisionSummary_ProjectsBeadKeyedCards(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		if r.Method != http.MethodGet {
			t.Fatalf("unexpected method %s", r.Method)
		}
		switch r.URL.Path {
		case "/api/reservations":
			if got := r.URL.Query().Get("project"); got != "p1" {
				t.Fatalf("project query = %q, want p1", got)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []map[string]any{
					{
						"id":           "res-same",
						"agent_id":     "holder-a",
						"project":      "p1",
						"path_pattern": "internal/http/*",
						"exclusive":    true,
						"reason":       "Wave 3 active_bead_id=sylveste-kgfi.3",
						"expires_at":   time.Now().Add(10 * time.Minute).Format(time.RFC3339),
						"is_active":    true,
					},
					{
						"id":           "res-stale",
						"agent_id":     "holder-b",
						"project":      "p1",
						"path_pattern": "internal/http/*",
						"exclusive":    true,
						"reason":       "prior work active_bead_id=sylveste-old.1",
						"expires_at":   "2026-04-30T03:00:00Z",
						"is_active":    false,
					},
				},
			}), nil
		case "/api/reservations/check":
			return jsonResponse(http.StatusOK, map[string]any{
				"conflicts": []map[string]any{
					{
						"reservation_id": "res-same",
						"agent_id":       "holder-a",
						"held_by":        "holder-a",
						"pattern":        "internal/http/*",
						"reason":         "Wave 3 active_bead_id=sylveste-kgfi.3",
						"expires_at":     "2026-04-30T05:00:00Z",
					},
				},
			}), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return nil, nil
		}
	})

	summary, err := c.CollisionSummary(context.Background(), []string{"internal/http/*"}, "sylveste-kgfi.3")
	if err != nil {
		t.Fatalf("CollisionSummary() error = %v", err)
	}
	if summary.Project != "p1" {
		t.Fatalf("Project = %q, want p1", summary.Project)
	}
	if len(summary.Cards) != 2 {
		t.Fatalf("len(Cards) = %d, want 2", len(summary.Cards))
	}

	same := summary.Cards[0]
	if same.ActiveBeadID != "sylveste-kgfi.3" || same.Confidence != "reported" {
		t.Fatalf("same bead card correlation = (%q,%q), want (sylveste-kgfi.3,reported)", same.ActiveBeadID, same.Confidence)
	}
	if same.HardBlocker {
		t.Fatalf("same bead card HardBlocker = true, want false")
	}
	if same.SuggestedAction != "coordinate_same_bead" {
		t.Fatalf("same bead action = %q, want coordinate_same_bead", same.SuggestedAction)
	}

	stale := summary.Cards[1]
	if stale.State != "stale" || stale.HardBlocker {
		t.Fatalf("stale card state/blocker = (%q,%v), want (stale,false)", stale.State, stale.HardBlocker)
	}
	if stale.SuggestedAction != "wait_for_expiry_or_release" {
		t.Fatalf("stale action = %q, want wait_for_expiry_or_release", stale.SuggestedAction)
	}
}

func TestCollisionSummary_UsesConflictEndpointForHardBlockers(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/reservations":
			return jsonResponse(http.StatusOK, map[string]any{"reservations": []map[string]any{}}), nil
		case "/api/reservations/check":
			if got := r.URL.Query().Get("pattern"); got != "internal/http/foo.go" {
				t.Fatalf("pattern query = %q, want internal/http/foo.go", got)
			}
			return jsonResponse(http.StatusOK, map[string]any{
				"conflicts": []map[string]any{
					{
						"reservation_id": "res-server",
						"agent_id":       "holder-server",
						"held_by":        "holder-server",
						"pattern":        "internal/http/*.go",
						"reason":         "server-side glob conflict",
						"expires_at":     time.Now().Add(10 * time.Minute).Format(time.RFC3339),
					},
				},
			}), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return nil, nil
		}
	})

	summary, err := c.CollisionSummary(context.Background(), []string{"internal/http/foo.go"}, "sylveste-kgfi.3")
	if err != nil {
		t.Fatalf("CollisionSummary() error = %v", err)
	}
	if len(summary.Conflicts) != 1 {
		t.Fatalf("len(Conflicts) = %d, want 1", len(summary.Conflicts))
	}
	card := summary.Conflicts[0]
	if !card.HardBlocker || card.SuggestedAction != "negotiate_release" {
		t.Fatalf("hard/action = (%v,%q), want (true,negotiate_release)", card.HardBlocker, card.SuggestedAction)
	}
	if card.Pattern != "internal/http/*.go" || card.PathPattern != "internal/http/*.go" {
		t.Fatalf("pattern aliases = (%q,%q), want internal/http/*.go", card.Pattern, card.PathPattern)
	}
}

func TestCollisionSummary_ListOnlyOverlapIsAdvisory(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.Path {
		case "/api/reservations":
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []map[string]any{
					{
						"id":           "res-list-only",
						"agent_id":     "holder-list",
						"project":      "p1",
						"path_pattern": "internal/http/*",
						"exclusive":    true,
						"reason":       "list-only advisory overlap",
						"expires_at":   time.Now().Add(10 * time.Minute).Format(time.RFC3339),
						"is_active":    true,
					},
				},
			}), nil
		case "/api/reservations/check":
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
			return nil, nil
		}
	})

	summary, err := c.CollisionSummary(context.Background(), []string{"internal/http/*"}, "sylveste-kgfi.3")
	if err != nil {
		t.Fatalf("CollisionSummary() error = %v", err)
	}
	if len(summary.Conflicts) != 0 {
		t.Fatalf("len(Conflicts) = %d, want 0 when server endpoint is clear", len(summary.Conflicts))
	}
	if len(summary.Cards) != 1 {
		t.Fatalf("len(Cards) = %d, want advisory card", len(summary.Cards))
	}
	if summary.Cards[0].HardBlocker {
		t.Fatalf("list-derived card HardBlocker = true, want false")
	}
	if summary.Cards[0].SuggestedAction != "coordinate_with_holder" {
		t.Fatalf("list-derived action = %q, want coordinate_with_holder", summary.Cards[0].SuggestedAction)
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

func TestReleaseByPattern_404Idempotent(t *testing.T) {
	t.Parallel()

	// Simulate race: another goroutine already deleted the reservation.
	// First DELETE succeeds, second DELETE returns 404.
	deleteCalls := 0
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("holder-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/reservations":
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []map[string]any{
					{
						"id":           "r1",
						"agent_id":     "holder-1",
						"path_pattern": "internal/tools/tools.go",
						"is_active":    true,
					},
					{
						"id":           "r2",
						"agent_id":     "holder-1",
						"path_pattern": "internal/client/client.go",
						"is_active":    true,
					},
				},
			}), nil
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/reservations/"):
			deleteCalls++
			if strings.HasSuffix(r.URL.Path, "/r2") {
				// Simulate concurrent deletion: 404 on second reservation
				return jsonResponse(http.StatusNotFound, map[string]any{
					"error": "not found",
				}), nil
			}
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		default:
			return jsonResponse(http.StatusNotFound, map[string]any{"error": "not found"}), nil
		}
	})

	released, err := c.ReleaseByPattern(context.Background(), "holder-1", "internal/*")
	if err != nil {
		t.Fatalf("ReleaseByPattern() error = %v, want nil (404 should be treated as success)", err)
	}
	if released != 2 {
		t.Fatalf("released = %d, want 2 (both counted including 404)", released)
	}
	if deleteCalls != 2 {
		t.Fatalf("deleteCalls = %d, want 2", deleteCalls)
	}
}

func TestCheckExpiredNegotiations_AdvisoryOnly(t *testing.T) {
	t.Parallel()

	// Verify that CheckExpiredNegotiations does NOT delete reservations.
	// It should return timeout-eligible negotiations with Released=0.
	deleteCalls := 0
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("a1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/inbox/"):
			return jsonResponse(http.StatusOK, map[string]any{
				"messages": []map[string]any{
					{
						"id":         "m1",
						"from":       "a2",
						"body":       `{"type":"release-request","file":"src/router.go","urgency":"urgent","thread_id":"t1"}`,
						"subject":    "release-request",
						"thread_id":  "t1",
						"created_at": "2020-01-01T00:00:00Z",
					},
				},
				"next_cursor": "",
			}), nil
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/threads/"):
			return jsonResponse(http.StatusOK, map[string]any{
				"messages": []map[string]any{
					{
						"id":      "m1",
						"subject": "release-request",
						"body":    `{"type":"release-request"}`,
					},
				},
			}), nil
		case r.Method == http.MethodDelete:
			deleteCalls++
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		default:
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		}
	})

	timeouts, err := c.CheckExpiredNegotiations(context.Background())
	if err != nil {
		t.Fatalf("CheckExpiredNegotiations() error = %v", err)
	}
	if len(timeouts) != 1 {
		t.Fatalf("len(timeouts) = %d, want 1", len(timeouts))
	}
	if timeouts[0].Released != 0 {
		t.Fatalf("timeouts[0].Released = %d, want 0 (advisory-only)", timeouts[0].Released)
	}
	if deleteCalls != 0 {
		t.Fatalf("deleteCalls = %d, want 0 (advisory-only should not delete)", deleteCalls)
	}
}

// makeExpiredThread returns thread messages for a release-request that is
// past the timeout window. The createdAt timestamp is set far in the past.
func makeExpiredThread(holderID string) []map[string]any {
	expired := time.Now().Add(-20 * time.Minute).Format(time.RFC3339)
	return []map[string]any{
		{
			"id":         "m1",
			"from":       "requester-1",
			"to":         []string{holderID},
			"body":       fmt.Sprintf(`{"type":"release-request","file":"src/router.go","urgency":"normal","thread_id":"t1","holder":"%s"}`, holderID),
			"subject":    "release-request",
			"thread_id":  "t1",
			"created_at": expired,
		},
	}
}

func TestForceReleaseNegotiation_HappyPath(t *testing.T) {
	t.Parallel()

	deleteCalls := 0
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/threads/"):
			return jsonResponse(http.StatusOK, map[string]any{
				"messages": makeExpiredThread("holder-1"),
			}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/reservations":
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []map[string]any{
					{
						"id":           "r1",
						"agent_id":     "holder-1",
						"path_pattern": "src/router.go",
						"is_active":    true,
					},
				},
			}), nil
		case r.Method == http.MethodDelete:
			deleteCalls++
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		default:
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		}
	})

	result, err := c.ForceReleaseNegotiation(context.Background(), "t1", "src/router.go", "need to fix critical bug")
	if err != nil {
		t.Fatalf("ForceReleaseNegotiation() error = %v", err)
	}
	if result.HolderID != "holder-1" {
		t.Fatalf("HolderID = %q, want holder-1", result.HolderID)
	}
	if result.Released != 1 {
		t.Fatalf("Released = %d, want 1", result.Released)
	}
	if deleteCalls != 1 {
		t.Fatalf("deleteCalls = %d, want 1", deleteCalls)
	}
}

func TestForceReleaseNegotiation_NotExpired(t *testing.T) {
	t.Parallel()

	recent := time.Now().Add(-1 * time.Minute).Format(time.RFC3339)
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]any{
			"messages": []map[string]any{
				{
					"id":         "m1",
					"from":       "requester-1",
					"to":         []string{"holder-1"},
					"body":       `{"type":"release-request","file":"src/router.go","urgency":"normal","thread_id":"t1"}`,
					"subject":    "release-request",
					"thread_id":  "t1",
					"created_at": recent,
				},
			},
		}), nil
	})

	_, err := c.ForceReleaseNegotiation(context.Background(), "t1", "src/router.go", "impatient")
	if err == nil {
		t.Fatal("ForceReleaseNegotiation() expected error for non-expired negotiation, got nil")
	}
	if !strings.Contains(err.Error(), "not exceeded") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForceReleaseNegotiation_AlreadyAcked(t *testing.T) {
	t.Parallel()

	expired := time.Now().Add(-20 * time.Minute).Format(time.RFC3339)
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]any{
			"messages": []map[string]any{
				{
					"id":         "m1",
					"from":       "requester-1",
					"to":         []string{"holder-1"},
					"body":       `{"type":"release-request","file":"src/router.go","urgency":"normal"}`,
					"subject":    "release-request",
					"thread_id":  "t1",
					"created_at": expired,
				},
				{
					"id":        "m2",
					"from":      "holder-1",
					"body":      `{"type":"release-ack","released":true}`,
					"subject":   "release-ack",
					"thread_id": "t1",
				},
			},
		}), nil
	})

	_, err := c.ForceReleaseNegotiation(context.Background(), "t1", "src/router.go", "already handled")
	if err == nil {
		t.Fatal("ForceReleaseNegotiation() expected error for already-acked negotiation, got nil")
	}
	if !strings.Contains(err.Error(), "already has a release-ack") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForceReleaseNegotiation_EmptyThread(t *testing.T) {
	t.Parallel()

	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		return jsonResponse(http.StatusOK, map[string]any{
			"messages": []map[string]any{},
		}), nil
	})

	_, err := c.ForceReleaseNegotiation(context.Background(), "t1", "src/router.go", "phantom thread")
	if err == nil {
		t.Fatal("ForceReleaseNegotiation() expected error for empty thread, got nil")
	}
	if !strings.Contains(err.Error(), "no messages") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestForceReleaseNegotiation_ZeroReleased(t *testing.T) {
	t.Parallel()

	// Holder already released their reservation (0 matches) — should succeed, not error.
	c := NewClient(WithBaseURL("http://intermute.local"), WithAgentID("requester-1"), WithProject("p1"))
	c.http.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/threads/"):
			return jsonResponse(http.StatusOK, map[string]any{
				"messages": makeExpiredThread("holder-1"),
			}), nil
		case r.Method == http.MethodGet && r.URL.Path == "/api/reservations":
			// No active reservations — holder already released
			return jsonResponse(http.StatusOK, map[string]any{
				"reservations": []map[string]any{},
			}), nil
		default:
			return jsonResponse(http.StatusOK, map[string]any{}), nil
		}
	})

	result, err := c.ForceReleaseNegotiation(context.Background(), "t1", "src/router.go", "cleanup")
	if err != nil {
		t.Fatalf("ForceReleaseNegotiation() error = %v (should succeed even with 0 released)", err)
	}
	if result.Released != 0 {
		t.Fatalf("Released = %d, want 0", result.Released)
	}
	if result.HolderID != "holder-1" {
		t.Fatalf("HolderID = %q, want holder-1", result.HolderID)
	}
}
