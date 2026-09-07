# Probe 2 brief (agent sweep-probe2)

Record everything verbatim in `<scratch>/probe2-report.md`, appending after each step. Working directory `<checkout>`. Do nothing beyond these steps.

1. Use the Write tool to create `interverse/interlore/PROBE.md` containing the line `x`. Record the complete tool result and any hook, warning, or block message, verbatim. Say explicitly whether the write was blocked or succeeded.
2. Use the Edit tool to change `x` to `y` in that file. Record the complete result verbatim, and whether it was blocked.
3. Run `cat interverse/interlore/PROBE.md 2>&1` and record the output.
4. Call the `check_conflicts` tool with `patterns` = `["interverse/interlore/PROBE.md"]`. Record the result.

Then run `touch <scratch>/done-probe2` and stop.
