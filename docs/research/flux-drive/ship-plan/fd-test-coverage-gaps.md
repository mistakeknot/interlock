# fd-test-coverage-gaps — Ship Plan Findings

**Reviewer:** fd-test-coverage-gaps (test strategy / failure-mode coverage lens)
**Target:** `docs/research/flux-review/interlock-ship-plan-review/2026-04-30-ship-plan.md`
**Date:** 2026-04-30
**Project:** interlock — 6 proposed tests for v0.2.14 hotfix and v0.2.16 xfail
**Lens:** Failure-mode coverage (not line coverage); concurrency; upgrade paths; xfail adequacy

---

## Summary

Four findings.
- One **P0**: No concurrent-process test exists. CF-5 (TOCTOU init guard) and CF-1 (pre-commit reconciliation data loss) are *concurrency bugs by definition*. A single-process test cannot reproduce them. The plan's 6 tests are all single-process — none of them exercise the actual race that produced the bug. The test labeled `test_concurrent_commit_loss_xfail` is described as a "failing test" but if it's single-process, it is not testing concurrent commit loss; it's testing some serialized approximation.
- One **P1**: The xfail test for CF-1 is the only signal of an open data-loss bug for users who get v0.2.14 but never reach v0.2.15/v0.2.16. Users do not read xfail annotations; they read runtime warnings. xfail without a runtime nag provides zero protection.
- One **P1**: No upgrade-path test. The orphan sweeper is verified on synthetic fixtures (`test_orphan_sweeper_classifies_before_delete`, line 88) but never against real v0.2.13-accumulated orphans with real edge cases (filenames with colons or weird charsets, extreme age, partial writes from killed sessions).
- One **P2**: Test 4 (`test_env_var_bypass_skips_isolation`) is too narrow. The plan's CF-4 fix in v0.2.15 strips `GIT_DIR`/`GIT_WORK_TREE` at the top of the wrapped function. The test should cover all three env-var forms (`GIT_DIR=`, `GIT_WORK_TREE=`, `GIT_INDEX_FILE=`) plus combinations and plus the negative case ("user legitimately wants GIT_DIR set; verify isolation correctly skips").

---

## P0: No concurrent-process test — the proposed `test_concurrent_commit_loss_xfail` cannot exercise the race it claims to test

**Location:** Ship plan §"Test additions" (lines 86-95), specifically:
- Test 6: `test_concurrent_commit_loss_xfail` — failing test for v0.2.16 design note (line 92).
- Tests 1-5 listed at lines 87-91 are all single-session.

**Failure scenario:** sma runs `pytest`. All 6 tests pass (test 6 xfails as expected). sma ships v0.2.14. A user runs two Claude Code sessions concurrently in the same repo (the use case interlock is *designed for*, per CLAUDE.md "MCP server for file reservation and agent coordination"). The two sessions hit CF-1 (pre-commit reconciliation) or CF-5 (TOCTOU init guard) and silently lose work.

The test suite passed. The bug shipped anyway. Why? Because:

1. **CF-5 TOCTOU on `[[ ! -f $SESSION_INDEX ]]`** is a TIME-OF-CHECK / TIME-OF-USE race between two processes that both pass the `! -f` check before either writes. A single-process test cannot reproduce this; the check passes once and the test proceeds linearly.

2. **CF-1 pre-commit reconciliation data loss** requires two `git commit` invocations interleaved by the kernel scheduler. The `mkdir`-based lockfile in `scripts/interlock-precommit-hook` (per ship plan line 99: "mkdir-based lockfile, not flock(1)") is the synchronization primitive being tested. A single-process test cannot exercise lockfile contention.

The plan's `test_concurrent_commit_loss_xfail` (line 92) is described as "failing test (xfail) that exercises the loss-of-work scenario" (line 60). But "exercises" how? If the test is in `tests/integration/test_concurrent_commit_loss.py` and uses `pytest`'s default single-process model, it is at best simulating concurrency via mocked-out clock or sequential calls — which doesn't catch the kernel-scheduler race that produces real-world CF-1 loss.

**Why P0:** This is the root failure-mode coverage gap. The plan ships a fix while explicitly choosing not to test the bug class that motivated the fix. If v0.2.15's wrapper hardening introduces a new race in the env-var-strip-then-rev-parse sequence (line 51), the test suite will not detect it.

**Concrete gap in plan:**
- No use of `multiprocessing`, `subprocess.Popen` parallel pairs, or `pytest-xdist` with shared fixtures.
- No use of a barrier/event primitive to synchronize two processes at the critical section.
- No description of how `test_concurrent_commit_loss_xfail` actually models concurrency.

**Smallest viable fix:** Replace test 6 with two tests:

```python
# test_concurrent_pre_commit_loss
def test_concurrent_pre_commit_loss(tmp_repo):
    """Two `git commit` processes race on the same repo. Both stage different
    files. Without CF-1 fix, one commit's staged file silently disappears."""
    barrier = multiprocessing.Barrier(2)
    p1 = multiprocessing.Process(target=commit_one, args=(tmp_repo, barrier, "a.txt"))
    p2 = multiprocessing.Process(target=commit_two, args=(tmp_repo, barrier, "b.txt"))
    p1.start(); p2.start()
    p1.join(); p2.join()
    log = subprocess.check_output(["git", "-C", tmp_repo, "log", "--name-only"])
    assert b"a.txt" in log, "CF-1: a.txt was silently lost"
    assert b"b.txt" in log, "CF-1: b.txt was silently lost"

# test_session_init_toctou
def test_session_init_toctou(tmp_repo):
    """Two SessionStart hook invocations race on `[[ ! -f $SESSION_INDEX ]]`.
    Without CF-5 fix, one session's index init silently overwrites the other's."""
    # similar multiprocessing pattern
```

Mark both as xfail until the fix lands. *This* is what "test the failure mode that produced the bug" looks like.

**Question for sma:** Is the existing `tests/structural/test_structure.py` Python or shell? Is there a test harness for spinning up multiple bash subprocesses against a shared fixture? If not, this is meaningful new infrastructure (worth doing, but not zero-cost).

---

## P1: xfail without a runtime warning provides zero protection for stuck-at-v0.2.14 users

**Location:** Ship plan line 60 (xfail test for v0.2.16) and line 63 (open question F: "should v0.2.14 add a stderr nag — 'interlock pre-commit reconciliation has known data-loss bug; concurrent commits may overwrite each other; see issue #N' — to match honesty principles?").

**Failure scenario:** A user installs interlock 0.2.14. They never run `pytest` against the plugin (most plugin users don't). They never see the xfail. They use the plugin for a month. Their team starts running concurrent commits (a workflow interlock is supposed to support). Work is silently lost.

The xfail is a signal *to the maintainer and to test-running CI*. It is not a signal to the runtime user.

**Why P1:** The xfail is the right test infrastructure but the wrong communication channel for users. CF-1 is described in the original review as a P0 silent-data-loss bug. The plan ships v0.2.14 *without fixing it* and chooses to document it via xfail in v0.2.16. Between the v0.2.14 ship and the v0.2.16 ship (whenever that is), users have no signal.

The plan's open question F asks whether to add a stderr nag. The answer should be **yes**, and it should be in v0.2.14, not deferred to "TBD."

**Smallest viable fix:** Add to the v0.2.14 file list (currently at lines 33-40):

- `scripts/interlock-precommit-hook` — add a one-time-per-session stderr message: `[interlock] WARNING: concurrent commits may produce data loss in this version. See https://github.com/mistakeknot/interlock/issues/N. Set INTERLOCK_SUPPRESS_CF1_WARN=1 to silence.`

This converts "xfail in test suite" (visible to maintainer only) into "stderr warning at commit time" (visible to every user every time they hit the dangerous path).

The xfail test should also annotate with the GitHub issue URL, e.g., `@pytest.mark.xfail(reason="CF-1 data loss; tracked at #N")` so when it unexpectedly passes (after the fix), the test output points the maintainer to the close-the-loop action.

---

## P1: No upgrade-path test — orphan sweeper is verified only on synthetic fixtures

**Location:** Ship plan line 90: `test_orphan_sweeper_classifies_before_delete — empty → delete; non-empty → quarantine.`

**Failure scenario:** The test creates synthetic orphan files in a tmpdir (presumably a small handful — 2 or 3 files) with controlled empty/non-empty states. The sweeper passes the test. sma runs the sweeper on real elf-revel orphans (638 files). The sweeper hits an edge case the synthetic fixtures didn't cover:

Plausible edge cases the synthetic test will not catch:
- An orphan file with a UUID containing a Unicode character (very unlikely from the plugin's own UUID generator, but possible if the repo was edited by other tools).
- An orphan file mid-write (truncated index format) — `git ls-files --cached --others` may error or treat the partial file as "non-empty" when it's actually corrupt.
- An orphan file owned by a different user (sma running the sweeper, the orphan written by an earlier process running as root or in a container).
- An orphan file on a filesystem with case-insensitive paths (macOS APFS default) where `index-Abc` and `index-abc` collide.
- An orphan timestamp older than the >7-day TTL by 30+ days, accumulated from before a clock change or NTP correction.
- A canonical `.git/index` that has staged work the user wants to keep, then `git reset --mixed HEAD` (per line 84) discards it.

**Why P1:** The sweeper *will* be run on 638 real files (per the Repair existing damage section). Synthetic-only test coverage is not adequate validation for that operation. fd-repair-safety-protocol covers the safety procedure; this finding is specifically about the test's representativeness.

**Smallest viable fix:** Add a test:

```python
def test_orphan_sweeper_on_real_corpus_sample():
    """Validate sweeper against a copy of N real orphans drawn from elf-revel.
    Captures filename quirks, timestamp distribution, and content shape that
    synthetic fixtures cannot anticipate."""
    sample = copy_real_orphans_to_tmp("/path/to/elf-revel/.git/index-*", n=20)
    # run sweeper in dry-run mode
    classification = run_sweeper_dry(sample)
    # assertions about classification distribution
```

This is essentially "fuzz-test against real-world data." For a one-time repair operation on 638 files, the marginal cost is 1 test; the marginal benefit is "we won't be surprised by a filename we never imagined."

**Question for sma:** Is there ethical concern about copying 20 real orphans (which may contain the user's own staged work) into a test fixture? If yes, the test should normalize/anonymize, or be a manual "dry-run review" rather than an automated test.

---

## P2: Test 4 (env-var bypass) is too narrow — should cover all three env-var forms plus negative case

**Location:** Ship plan line 91: `test_env_var_bypass_skips_isolation — GIT_DIR=/other/repo command git ... doesn't write to session index.`

**Failure scenario:** Test 4 covers `GIT_DIR=`. CF-4 fix per ship plan line 53 strips `GIT_DIR` AND `GIT_WORK_TREE`. Per ship plan line 35-36, `stop.sh` reads `GIT_INDEX_FILE`. So the env-var bypass surface is at least three variables. The test as scoped only covers one.

If the v0.2.15 implementation strips `GIT_DIR` correctly but forgets to strip `GIT_WORK_TREE` (a plausible implementation oversight given line 53 mentions both but the test only asserts one), the test passes and the bug ships.

Additionally, there is no negative test ("user legitimately wants `GIT_DIR=/some/repo` set in their shell because they're using git in a multi-worktree setup; verify the wrapper does the right thing — either honors GIT_DIR safely or warns clearly").

**Why P2:** Coverage gap — narrower than the fix surface. Not catastrophic because the ship plan's other CF-4 mitigation (env-strip at top of function, line 53) is structural, but the test should match the structural change.

**Smallest viable fix:** Expand test 4 into three or four tests:

```python
def test_env_var_bypass_GIT_DIR()        # current test
def test_env_var_bypass_GIT_WORK_TREE()  # new
def test_env_var_bypass_GIT_INDEX_FILE() # new — verifies stop.sh path-reconstruction
def test_env_var_combinations()          # GIT_DIR + GIT_WORK_TREE simultaneously
```

---

## P3: Structural tests as regression guard — verify they catch the new files

**Location:** Ship plan line 95: "Plus existing structural tests in `tests/structural/test_structure.py` must continue to pass (assertions for `git()`, `command git`, `env -u GIT_INDEX_FILE`, `GIT_INDEX_FILE`, `index-${SESSION_ID}`, `git read-tree HEAD`)."

**Concern:** v0.2.14 adds a new file (`scripts/interlock-orphan-sweep`, line 38) and a new helper (`session_index_is_empty()` in `hooks/lib.sh`, line 36). The existing structural tests check tokens in the existing files — they will not check tokens in the new files. The new sweeper script, if it has a token-level bug (e.g., shell-quoting error around UUID), will not be caught by the structural test suite.

**Smallest viable fix:** Add to the structural test suite:
- An assertion that `scripts/interlock-orphan-sweep` exists and is executable.
- An assertion that the sweeper sources `hooks/lib.sh` and uses `session_index_is_empty()`.
- An assertion that the sweeper does NOT use unquoted `$UUID` interpolation in a `git` command.

This is P3 because structural tests are a tertiary defense; the unit tests at lines 87-92 are the primary defense.

---

## What I did NOT review (per agent boundaries)

- Whether shipping an xfail with no fix in v0.2.16 is a semver correctness issue — fd-semver-coordination.
- Whether the orphan sweeper running on real elf-revel data is operationally safe — fd-repair-safety-protocol.
- Whether the cache mirror has its own test coverage — fd-cache-mirror-drift.
- Whether the test gating creates a merge-block issue — fd-irreversible-action-discipline.

---

## Decision summary

The 6 proposed tests verify implementation details — they do not exercise the failure modes that produced the bugs being fixed.

**The single most important missing test:** a multi-process concurrent-commit test using `multiprocessing.Barrier` or `subprocess.Popen` pairs. CF-1 and CF-5 are concurrency bugs; testing them requires concurrent processes. The proposed `test_concurrent_commit_loss_xfail` is, in its single-process form, a placeholder, not a test of the actual bug.

**The second most important missing artifact:** a runtime stderr warning in v0.2.14 that fires on the dangerous path. xfail in the test suite is invisible to the user. Open question F should be answered "yes, ship the warning in v0.2.14" — both for honesty (the bug is real) and for failure-mode coverage (the warning is the user-facing equivalent of a test).

**Test strategy adequacy verdict:** the 6-test suite is adequate for verifying that v0.2.14's specific code changes work in isolation. It is **inadequate** for verifying that the system as a whole no longer has the bug class that produced CF-1 / CF-5 / CF-3 / CF-4. The plan should add at minimum:
1. One concurrent-process test (`test_concurrent_pre_commit_loss`).
2. One TOCTOU race test (`test_session_init_toctou`).
3. One real-corpus sweeper test (`test_orphan_sweeper_on_real_corpus_sample`).
4. Expansion of test 4 from single env var to all three.
5. Runtime stderr warning in `scripts/interlock-precommit-hook`.

Bringing the test count from 6 to ~10 and adding one stderr warning closes the failure-mode coverage gaps.
