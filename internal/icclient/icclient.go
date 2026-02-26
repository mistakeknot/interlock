// Package icclient wraps the `ic coordination` CLI for use as a reservation backend.
// When the `ic` binary is available, Interlock routes reservation operations through
// Intercore's unified coordination_locks table instead of Intermute's HTTP API.
package icclient

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
)

// Client wraps calls to the `ic` binary.
type Client struct {
	binary  string
	workDir string // working directory for ic process (ic finds DB by walking up from CWD)
}

// New creates a client, auto-discovering the `ic` binary on PATH.
func New() *Client {
	path, _ := exec.LookPath("ic")
	return &Client{binary: path}
}

// Available returns true if the `ic` binary was found.
func (c *Client) Available() bool {
	return c.binary != ""
}

// ReserveResult is the JSON output from `ic coordination reserve`.
type ReserveResult struct {
	Lock     *LockInfo     `json:"lock,omitempty"`
	Conflict *ConflictInfo `json:"conflict,omitempty"`
}

// LockInfo is a lock entry from coordination_locks.
type LockInfo struct {
	ID        string `json:"id"`
	Owner     string `json:"owner"`
	Scope     string `json:"scope"`
	Pattern   string `json:"pattern"`
	Exclusive bool   `json:"exclusive"`
	Reason    string `json:"reason,omitempty"`
}

// ConflictInfo describes a blocking reservation.
type ConflictInfo struct {
	BlockerOwner   string `json:"blocker_owner"`
	BlockerPattern string `json:"blocker_pattern"`
	BlockerReason  string `json:"blocker_reason"`
}

// Reserve calls `ic coordination reserve`. Returns the result and whether a
// conflict was detected (exit code 1).
func (c *Client) Reserve(ctx context.Context, owner, scope, pattern, reason string, ttlSec int, exclusive bool) (*ReserveResult, error) {
	args := []string{"--json", "coordination", "reserve",
		"--owner=" + owner, "--scope=" + scope, "--pattern=" + pattern,
		fmt.Sprintf("--ttl=%d", ttlSec)}
	if reason != "" {
		args = append(args, "--reason="+reason)
	}
	if !exclusive {
		args = append(args, "--exclusive=false")
	}

	out, err := c.run(ctx, args...)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Conflict — parse the output for conflict details.
			var result ReserveResult
			if jsonErr := json.Unmarshal(out, &result); jsonErr == nil {
				return &result, nil
			}
			return nil, fmt.Errorf("reserve conflict (parse failed): %s", out)
		}
		return nil, fmt.Errorf("ic coordination reserve: %w", err)
	}

	var result ReserveResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse reserve result: %w", err)
	}
	return &result, nil
}

// Release calls `ic coordination release` by lock ID.
func (c *Client) Release(ctx context.Context, id string) error {
	_, err := c.run(ctx, "--json", "coordination", "release", id)
	return err
}

// ReleaseAll calls `ic coordination release --owner --scope`.
func (c *Client) ReleaseAll(ctx context.Context, owner, scope string) (int64, error) {
	out, err := c.run(ctx, "--json", "coordination", "release", "--owner="+owner, "--scope="+scope)
	if err != nil {
		return 0, err
	}
	var result struct {
		Released int64 `json:"released"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return 0, fmt.Errorf("parse release result: %w", err)
	}
	return result.Released, nil
}

// Check calls `ic coordination check`. Returns conflicts (if any) and whether
// the path is clear (no conflicts).
func (c *Client) Check(ctx context.Context, scope, pattern, excludeOwner string) ([]byte, bool, error) {
	args := []string{"--json", "coordination", "check", "--scope=" + scope, "--pattern=" + pattern}
	if excludeOwner != "" {
		args = append(args, "--exclude-owner="+excludeOwner)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return out, true, nil // has conflicts
		}
		return nil, false, fmt.Errorf("ic coordination check: %w", err)
	}
	return out, false, nil // clear
}

// List calls `ic coordination list` with optional filters.
func (c *Client) List(ctx context.Context, owner, scope string) (json.RawMessage, error) {
	args := []string{"--json", "coordination", "list", "--active"}
	if owner != "" {
		args = append(args, "--owner="+owner)
	}
	if scope != "" {
		args = append(args, "--scope="+scope)
	}
	out, err := c.run(ctx, args...)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(out), nil
}

// SetWorkDir sets the working directory for ic subprocesses.
// The ic binary finds its database by walking up from CWD.
func (c *Client) SetWorkDir(dir string) {
	c.workDir = dir
}

func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.binary, args...)
	if c.workDir != "" {
		cmd.Dir = c.workDir
	}
	out, err := cmd.Output()
	if err != nil {
		// Preserve stderr in the error for exit errors.
		if exitErr, ok := err.(*exec.ExitError); ok {
			return out, exitErr
		}
		return nil, err
	}
	return out, nil
}
