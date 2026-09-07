# Probe brief (agent sweep-probe)

You are a probe. Your only job is to exercise the coordination tools once and write down exactly what happened. Do not fix anything. Do not explore. `$D` is `<scratch>`. Your working directory is `<checkout>`.

Write every result below, verbatim, into `$D/probe-report.md` as you go (append after each step; do not wait until the end).

1. Call the `list_agents` tool with no arguments. Record the full result.
2. Call `my_reservations`. Record the result.
3. Call `reserve_files` with `patterns` = `["interverse/interlore/PROBE.md"]`, `reason` = `"probe: identity check"`, `ttl_minutes` = 10. Record the result including any ids.
4. Use the Write tool to create `interverse/interlore/PROBE.md` containing the single line `probe`. Record any hook output or block message you see, verbatim, and whether the write succeeded.
5. Use the Edit tool to change that line from `probe` to `probe 2`. Record any hook output or block message verbatim.
6. Run `git -C interverse/interlore add PROBE.md && git -C interverse/interlore commit -m "probe: identity check" -- PROBE.md`. Record stdout and stderr verbatim, including anything the pre-commit or post-commit hook printed.
7. Call `my_reservations` again. Record the result.
8. Call `fetch_inbox`. Record the result.
9. Call `release_all`. Record the result.
10. Run `env | grep -E '^(INTERMUTE|INTERLOCK|CLAUDE_SESSION|CLAUDE_ENV)' | sort` and record the output.

Then run `touch $D/done-probe` and stop. Do not do anything else.
