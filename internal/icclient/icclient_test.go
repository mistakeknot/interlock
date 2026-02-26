package icclient

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// findIC returns the path to the ic binary with coordination support. It checks:
// 1. IC_TEST_BIN env var (pre-built binary for CI)
// 2. Pre-built binary in intercore source tree (has latest features)
// 3. PATH lookup (may be stale — checked last)
// Skips the test if no binary is found.
func findIC(t *testing.T) string {
	t.Helper()

	// 1. Explicit env var.
	if bin := os.Getenv("IC_TEST_BIN"); bin != "" {
		if _, err := os.Stat(bin); err == nil {
			return bin
		}
	}

	// 2. Check pre-built binary in intercore source tree (preferred — has latest code).
	icSrc := filepath.Join("..", "..", "..", "..", "core", "intercore")
	preBuild := filepath.Join(icSrc, "ic")
	if abs, err := filepath.Abs(preBuild); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return abs
		}
	}

	// 3. Check PATH (may be stale).
	if bin, err := exec.LookPath("ic"); err == nil {
		return bin
	}

	t.Skip("ic binary not found — set IC_TEST_BIN, build core/intercore/ic, or put ic on PATH")
	return ""
}

// setupClient creates a Client with a temp project directory containing .clavain/.
// The ic binary resolves its DB by walking up from CWD.
func setupClient(t *testing.T) *Client {
	t.Helper()
	binPath := findIC(t)

	projDir := t.TempDir()
	clavainDir := filepath.Join(projDir, ".clavain")
	if err := os.MkdirAll(clavainDir, 0755); err != nil {
		t.Fatalf("mkdir .clavain: %v", err)
	}

	c := &Client{binary: binPath}
	c.SetWorkDir(projDir)
	return c
}

func TestClient_Available(t *testing.T) {
	c := &Client{binary: ""}
	if c.Available() {
		t.Error("expected not available with empty binary")
	}

	c = &Client{binary: "/usr/bin/true"}
	if !c.Available() {
		t.Error("expected available with binary set")
	}
}

func TestClient_ReserveAndRelease(t *testing.T) {
	c := setupClient(t)
	ctx := context.Background()

	// Reserve a pattern.
	result, err := c.Reserve(ctx, "agent-test", "/test/project", "*.go", "icclient test", 900, true)
	if err != nil {
		t.Fatalf("reserve: %v", err)
	}
	if result.Lock == nil {
		t.Fatal("expected lock in result")
	}
	if result.Lock.ID == "" {
		t.Error("expected non-empty lock ID")
	}
	if result.Lock.Owner != "agent-test" {
		t.Errorf("owner = %q, want agent-test", result.Lock.Owner)
	}

	// Check — should be clear for the same owner.
	_, hasConflict, err := c.Check(ctx, "/test/project", "*.go", "agent-test")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if hasConflict {
		t.Error("expected no conflict when excluding owner")
	}

	// Check — should conflict for a different owner.
	_, hasConflict, err = c.Check(ctx, "/test/project", "main.go", "other-agent")
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if !hasConflict {
		t.Error("expected conflict for different owner")
	}

	// Release by ID.
	err = c.Release(ctx, result.Lock.ID)
	if err != nil {
		t.Fatalf("release: %v", err)
	}

	// Check again — should be clear now.
	_, hasConflict, err = c.Check(ctx, "/test/project", "main.go", "other-agent")
	if err != nil {
		t.Fatalf("check after release: %v", err)
	}
	if hasConflict {
		t.Error("expected no conflict after release")
	}
}

func TestClient_ReserveConflict(t *testing.T) {
	c := setupClient(t)
	ctx := context.Background()

	// First reserve succeeds.
	result1, err := c.Reserve(ctx, "agent-a", "/proj", "*.go", "first", 900, true)
	if err != nil {
		t.Fatalf("reserve 1: %v", err)
	}
	if result1.Lock == nil {
		t.Fatal("expected lock")
	}

	// Second reserve from different agent should conflict.
	result2, err := c.Reserve(ctx, "agent-b", "/proj", "main.go", "second", 900, true)
	if err != nil {
		t.Fatalf("reserve 2: %v", err)
	}
	if result2.Conflict == nil {
		t.Fatal("expected conflict")
	}
	if result2.Conflict.BlockerOwner != "agent-a" {
		t.Errorf("blocker = %q, want agent-a", result2.Conflict.BlockerOwner)
	}
}

func TestClient_ReleaseAll(t *testing.T) {
	c := setupClient(t)
	ctx := context.Background()

	// Create two reservations for the same agent.
	c.Reserve(ctx, "agent-x", "/proj", "a.go", "r1", 900, true)
	c.Reserve(ctx, "agent-x", "/proj", "b.go", "r2", 900, true)

	n, err := c.ReleaseAll(ctx, "agent-x", "/proj")
	if err != nil {
		t.Fatalf("release all: %v", err)
	}
	if n != 2 {
		t.Errorf("released = %d, want 2", n)
	}
}

func TestClient_List(t *testing.T) {
	c := setupClient(t)
	ctx := context.Background()

	c.Reserve(ctx, "agent-list", "/proj", "pkg/*.go", "listing", 900, true)

	raw, err := c.List(ctx, "agent-list", "/proj")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(raw) == 0 {
		t.Error("expected non-empty list result")
	}
}
