# Code Quality Review: Interlock Advisory Timeout + Idempotency Fixes

**Reviewer**: Flux-drive Quality & Style Reviewer
**Date**: 2026-02-15
**Scope**: Go (client, tools) + Python (structural tests)
**Files**: internal/client/client.go, internal/client/client_test.go, internal/tools/tools.go, tests/structural/test_structure.py, .claude-plugin/plugin.json

---

## Summary

This diff transitions negotiation timeout enforcement from automatic force-release to advisory-only mode and adds idempotent 404 handling to `ReleaseByPattern`. The changes improve protocol correctness and eliminate race-condition errors in concurrent release scenarios. Code quality is high across all languages: Go idioms are followed, tests are comprehensive, and Python structural tests correctly validate the architectural shift.

**Key Strengths**:
- Proper idiomatic error handling with `errors.As` for typed error checks
- Exported constants eliminate magic number duplication between layers
- Comprehensive test coverage including race-condition simulation
- Structural tests enforce protocol invariants at the implementation level
- Clear documentation in comments and test names

**Findings**: 2 minor naming consistency issues, 1 comment clarity improvement opportunity.

---

## Universal Review

### Naming Consistency

**FINDING 1: Constant naming inconsistency across package boundaries**

**Location**: `internal/tools/tools.go:20-24`

```go
const (
	normalTimeoutMinutes    = client.NormalTimeoutMinutes
	urgentTimeoutMinutes    = client.UrgentTimeoutMinutes
	negotiationPollInterval = client.NegotiationPollInterval
)
```

**Issue**: Tools layer aliases use `camelCase` while client layer exports use `PascalCase`. This creates unnecessary cognitive friction when reading cross-package references. The aliasing adds no value since the constants are only used in one location (line 354 description formatting).

**Impact**: Moderate — reduces code clarity without benefit.

**Fix**: Remove the aliasing constants and reference `client.NormalTimeoutMinutes` directly at the usage site (line 354). This eliminates the naming inconsistency and makes the source of truth explicit.

```go
// Remove lines 20-24
// Update line 354:
mcp.WithDescription(fmt.Sprintf("Urgency level: 'normal' (%d minute timeout) or 'urgent' (%d minute timeout). Default: normal",
    client.NormalTimeoutMinutes, client.UrgentTimeoutMinutes)),
```

---

### File Organization

**FINDING 2: Orphaned goroutine cleanup left incomplete**

**Location**: `internal/tools/tools.go` (diff context)

```diff
-	"sync"
-var (timeoutCheckerOnce sync.Once; timeoutCheckerStop chan struct{})
-func StopTimeoutChecker() { ... } // removed
```

**Issue**: The diff removes the goroutine-based timeout checker but leaves the import cleanup and structural evidence in the diff without context. The actual file should be verified to confirm `sync` import is removed if no longer needed (likely used by `atomic.Uint64` at line 674).

**Impact**: Low — import hygiene only, no runtime effect.

**Fix**: Confirm `sync` import removal if `atomic` is the only remaining concurrency primitive. Run `goimports` to auto-clean unused imports.

---

### Error Handling Patterns

**STRENGTH**: idiomatic error wrapping and typed error checks

**Location**: `internal/client/client.go:358-363`

```go
if err := c.DeleteReservation(ctx, r.ID); err != nil {
	if !isNotFound(err) {
		return released, fmt.Errorf("delete reservation %q: %w", r.ID, err)
	}
	// 404 = already deleted by another goroutine/session, count as success.
}
released++
```

This is exemplary Go error handling:
- Uses `%w` for error chain preservation (allows upstream `errors.Is`/`errors.As` checks)
- Applies typed error detection via `isNotFound` helper (uses `errors.As` for `*IntermuteError` at line 574-578)
- Documents the idempotency decision inline
- Unconditionally increments `released` counter after the error check, treating 404 as success

The corresponding test (`TestReleaseByPattern_404Idempotent`) validates this behavior with a simulated race condition (lines 218-268 in client_test.go).

---

### Test Strategy

**STRENGTH**: multi-layered test coverage across Go unit tests and Python structural tests

#### Go Unit Tests

**Location**: `internal/client/client_test.go:218-268, 270-324`

Two new test functions cover the advisory timeout behavior:

1. **`TestReleaseByPattern_404Idempotent`** (lines 218-268):
   - Simulates concurrent deletion race: first DELETE succeeds, second returns 404
   - Verifies both reservations counted as released (idempotency guarantee)
   - Uses table-driven mock responses via `roundTripFunc` pattern (lines 13-17)
   - Asserts `deleteCalls == 2` and `released == 2` with clear failure messages

2. **`TestCheckExpiredNegotiations_AdvisoryOnly`** (lines 270-324):
   - Verifies no DELETE calls when checking expired negotiations
   - Asserts `Released=0` in timeout result (advisory-only, not force-release)
   - Uses message timestamps from 2020 to guarantee timeout (6-year age vs 5-minute threshold)
   - Mocks thread fetch to return request-only (no ack), triggering timeout detection

Both tests use parallel execution (`t.Parallel()`) and follow Go table-test conventions.

#### Python Structural Tests

**Location**: `tests/structural/test_structure.py:379-432`

New test class `TestNegotiationProtocol` with 7 tests enforcing protocol-level invariants:

- **`test_pre_edit_has_auto_release_flag`** (line 382): Verifies feature flag check exists
- **`test_pre_edit_has_advisory_release`** (line 387): Confirms `additionalContext` mode (not auto-delete)
- **`test_pre_edit_has_negotiation_throttle`** (line 393): Validates throttle flag file usage
- **`test_lib_has_negotiation_check_path`** (line 398): Checks helper function exists
- **`test_lib_has_fast_curl`** (line 403): Ensures circuit-breaker timeout wrapper
- **`test_tools_have_exported_constants`** (line 409): Validates constant exports
- **`test_advisory_timeout_no_force_release`** (line 416): **Critical test** — parses `CheckExpiredNegotiations` function line-by-line to assert `ReleaseByPattern` is never called, confirming advisory-only behavior

The last test is particularly valuable: it enforces the architectural invariant at the source code level, preventing future regressions from reintroducing force-release logic.

**Test quality**: High. Assertions are precise, failure messages include context, and the line-by-line parsing approach in `test_advisory_timeout_no_force_release` is a creative solution for validating function behavior without executing it.

---

### Complexity Budget

**STRENGTH**: removed goroutine complexity in favor of polling in caller's context

**Location**: `internal/tools/tools.go` (diff context)

```diff
-var (timeoutCheckerOnce sync.Once; timeoutCheckerStop chan struct{})
-func StopTimeoutChecker() { ... } // removed
-// In negotiateRelease: goroutine removed
```

The original design launched a background goroutine to enforce timeouts. The new design:
- Moves timeout checking into `CheckExpiredNegotiations` (called from `fetch_inbox` tool)
- Returns advisory information instead of taking action
- Eliminates lifecycle management complexity (no `sync.Once`, no stop channel)

This is a net win for maintainability: fewer moving parts, easier to reason about, and aligns with the project's multi-agent coordination model (requester decides when to escalate, holder sees advisory context on next edit).

---

## Go-Specific Review

### Exported Symbol Naming

**STRENGTH**: exported constants follow 5-second naming rule

**Location**: `internal/client/client.go:369-375`

```go
// Negotiation timeout constants. Exported so tools layer can reference them
// in descriptions without duplicating magic numbers.
const (
	NormalTimeoutMinutes    = 10
	UrgentTimeoutMinutes    = 5
	NegotiationPollInterval = 2 * time.Second
)
```

These names are clear, self-documenting, and align with Go naming conventions:
- `NormalTimeoutMinutes` — unambiguous intent (timeout value for normal urgency)
- `UrgentTimeoutMinutes` — parallel structure to Normal variant
- `NegotiationPollInterval` — duration type suffix `Interval` signals `time.Duration`

The comment explains the export rationale (tools layer needs them for descriptions), which is helpful for future maintainers who might question why these are exported from an internal package.

---

### Error Wrapping

**STRENGTH**: consistent use of `%w` for error chain preservation

All error returns in the diff use `fmt.Errorf(..., %w)`:

- Line 360: `fmt.Errorf("delete reservation %q: %w", r.ID, err)`
- Line 350: `fmt.Errorf("list reservations for %q: %w", agentID, err)`
- Line 387: `fmt.Errorf("fetch inbox for negotiation timeout: %w", err)`
- Line 443: `fmt.Errorf("check thread %q for timeout: %w", threadID, threadErr)`

This enables upstream callers to use `errors.Is` and `errors.As` for typed error handling (e.g., the `isNotFound` check at line 359).

---

### Testing: Table-Driven + Parallel Execution

**Location**: `internal/client/client_test.go`

Both new tests use `t.Parallel()` (lines 219, 271) and the `roundTripFunc` mock pattern (lines 13-17, 224-256, 276-309). This is idiomatic Go testing:

- Parallel execution speeds up test suite runs
- `roundTripFunc` avoids heavyweight test server setup (httptest.Server)
- Mock responses use `jsonResponse` helper (lines 19-28) for consistent marshaling

The mock logic in `TestReleaseByPattern_404Idempotent` is particularly well-designed:

```go
case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/reservations/"):
	deleteCalls++
	if strings.HasSuffix(r.URL.Path, "/r2") {
		// Simulate concurrent deletion: 404 on second reservation
		return jsonResponse(http.StatusNotFound, map[string]any{
			"error": "not found",
		}), nil
	}
	return jsonResponse(http.StatusOK, map[string]any{}), nil
```

The path suffix check simulates a race condition where `r2` was deleted by another goroutine between the List and Delete calls. This validates the idempotency fix in a realistic scenario.

---

### Imports Cleanup

**FINDING 3: Verify sync package removal**

**Location**: `internal/tools/tools.go` (diff context)

The diff shows `"sync"` import removal, but line 674 uses `sync/atomic.Uint64`. Verify:
1. If `sync` was replaced by `sync/atomic` in imports (correct)
2. Or if `sync` is still needed for other reasons

Run `goimports` to auto-fix. This is low-priority hygiene.

---

## Python-Specific Review

### Type Hints and Pythonic Constructs

**STRENGTH**: pytest fixtures and parametrize for DRY tests

**Location**: `tests/structural/test_structure.py:379-432`

The new test class uses pytest patterns effectively:
- Fixture `project_root` (defined elsewhere, injected by pytest)
- `Path` objects for filesystem operations (pathlib is more Pythonic than os.path)
- String methods (`.splitlines()`, `in content`) for pattern matching

The line-by-line parser in `test_advisory_timeout_no_force_release` is creative:

```python
in_func = False
found_advisory_comment = False
for line in content.splitlines():
    if "func (c *Client) CheckExpiredNegotiations" in line:
        in_func = True
    elif in_func:
        if line.startswith("func ") or (line.startswith("}") and not line.startswith("})")):
            break
        assert "ReleaseByPattern" not in line, \
            "CheckExpiredNegotiations must not call ReleaseByPattern (advisory-only)"
        if "advisory" in line.lower() or "Advisory" in line:
            found_advisory_comment = True
assert found_advisory_comment, "CheckExpiredNegotiations should have advisory comment"
```

**FINDING 4: Parser could miss edge cases due to formatting assumptions**

**Issue**: The `line.startswith("func ")` check to detect function boundaries assumes standard Go formatting. If code is formatted with comments between functions or has closing braces on the same line as other code, the parser could misidentify boundaries.

**Impact**: Low — gofmt enforces consistent formatting, unlikely to break in practice.

**Improvement**: Use a simple heuristic improvement:

```python
if line.startswith("func ") and not line.startswith("func ("):
    break  # Next standalone function
```

This avoids breaking on inline function literals (rare in client.go but possible).

---

### Naming Conventions

**STRENGTH**: snake_case everywhere, class names follow PEP 8

- `test_advisory_timeout_no_force_release` — descriptive, under 80 chars
- `TestNegotiationProtocol` — PascalCase class name per pytest convention
- No abbreviations that sacrifice clarity (e.g., `found_advisory_comment` not `found_adv`)

---

### Test Assertions and Coverage

**STRENGTH**: assertions include helpful failure messages

Every assertion in the new tests includes a custom message:

```python
assert found_advisory_comment, "CheckExpiredNegotiations should have advisory comment"
```

This is critical for CI debugging: when a test fails in a remote pipeline, the message explains *what* was expected, not just *where* it failed.

---

## Shell-Specific Review

**Not applicable**: No shell changes in this diff. The `.claude-plugin/plugin.json` change updates the MCP command path from direct binary to `launch-mcp.sh` wrapper, but the wrapper script itself is not in this diff.

---

## JSON Manifest Change

**Location**: `.claude-plugin/plugin.json:20`

```diff
-      "command": "${CLAUDE_PLUGIN_ROOT}/bin/interlock-mcp",
+      "command": "${CLAUDE_PLUGIN_ROOT}/bin/launch-mcp.sh",
```

**Context**: This change wraps the MCP server binary in a launcher script. The script likely handles environment setup (socket path, env vars) before exec'ing the binary.

**Risk**: Low, assuming `launch-mcp.sh` exists and is executable. The structural tests verify this indirectly:

```python
def test_mcp_server_declared(self, plugin_json):
    assert "interlock-mcp" in srv["command"] or "launch-mcp" in srv["command"]
```

This test was updated to accept either path, confirming the change is intentional and tested.

---

## Findings Summary

| # | Severity | Type | Location | Summary |
|---|----------|------|----------|---------|
| 1 | Minor | Naming | tools.go:20-24 | Remove redundant constant aliasing |
| 2 | Minor | Import | tools.go | Verify `sync` import removal via goimports |
| 3 | Info | Comment | client.go:458-460 | Advisory comment could be more explicit about requester escalation |
| 4 | Minor | Test | test_structure.py:426 | Line parser could be more robust to formatting edge cases |

---

## Recommendations

### Immediate (before merge)

1. **Remove constant aliasing in tools.go**: Reference `client.NormalTimeoutMinutes` directly at line 354.
2. **Run goimports on tools.go**: Confirm `sync` import is correctly removed.

### Future improvements

1. **Extract line parser to a shared test utility**: The Go function parser in `test_advisory_timeout_no_force_release` could be reused for other structural invariants (e.g., "hook X must not call Y"). Consider extracting to a `tests/structural/parse_go.py` module.

2. **Add integration test for 404 race condition**: The unit test simulates the race with mocks. A live integration test against a real intermute instance (with concurrent sessions) would validate the fix end-to-end.

3. **Document negotiation timeout escalation in AGENTS.md**: The protocol shift from auto-release to advisory-only is architectural. Add a "Negotiation Protocol" section to AGENTS.md explaining timeout escalation, advisory vs blocking enforcement, and when to use `respond_to_release`.

---

## Positive Callouts

1. **Idempotency fix is well-tested**: The 404 race condition test validates a real concurrency bug with a minimal, focused mock.

2. **Exported constants eliminate duplication**: Moving timeout values to client layer and exporting them prevents magic number drift between client/tools layers.

3. **Structural tests enforce protocol invariants**: The `test_advisory_timeout_no_force_release` test is a creative use of Python to validate Go code structure, preventing future regressions.

4. **Error wrapping is consistent**: All new error paths use `%w` for chain preservation, enabling typed error checks upstream.

5. **Comment quality is high**: Comments explain *why* decisions were made (e.g., "404 = already deleted by another goroutine/session, count as success"), not just *what* the code does.

---

## Conclusion

This diff is production-ready with minor cleanup. The architectural shift from auto-release to advisory-only negotiation is well-implemented: Go code follows idioms, tests are comprehensive, and Python structural tests validate the protocol invariants. The idempotency fix eliminates a race condition that would cause spurious errors in multi-session workflows.

**Approval status**: APPROVED with minor non-blocking improvements (constant aliasing removal + goimports cleanup).
