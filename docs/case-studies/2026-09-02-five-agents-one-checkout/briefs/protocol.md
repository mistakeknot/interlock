# Coordination protocol (read this before your lane brief)

You are one of five agents editing the same checkout at the same time. The other agents are real and are editing the same repos you are, often the same README.md. The point of this job is as much to exercise the coordination tools as to finish the sweep, so follow this protocol exactly and write down every surprise.

Your working directory is `<checkout>`. Every repo you touch is at `interverse/<repo>/` and is its own git repository, already on branch `sweep/2026-09-02`. Never switch branches, never `git add -A`, never `git stash`, never `git pull`, never push. Never touch a repo that is not in your list.

## Before you edit anything in a repo

1. Call `reserve_files` with `patterns` = the exact files you will edit in that repo, as paths relative to `<checkout>` (for example `interverse/intercache/README.md`), `reason` = `"<lane>: <repo>"`, `ttl_minutes` = 20. Keep the reservation ids from the result.
2. If the result reports a conflict with another agent, do NOT edit that file. Call `negotiate_release` with `agent_name` = the holder's name, `file` = the pattern, `reason` = one sentence saying what you need to change, `urgency` = `"normal"`, `wait_seconds` = 0. Then move on to the next repo in your order and come back to this one later. Do not wait or poll in a loop.
3. If a file edit is blocked by a hook message that says the file is reserved by someone else, treat it like step 2. If the block names YOUR OWN agent name, record the full message in your friction file, then retry the edit once. If it is still blocked, record that too, and skip that file for now.

## Every time you start a new repo

Call `fetch_inbox`. For each release request addressed to you:
- If you are not currently editing the file: call `respond_to_release` with `action` = `"release"`, `file`, `requester`, and `thread_id` from the request.
- If you are mid-edit on that file: `respond_to_release` with `action` = `"defer"` and `eta_minutes` = 3, then finish and release within that time.

## After you finish a repo

1. Commit only your files, from inside the repo, with a pathspec: `git -C interverse/<repo> commit -m "<lane>: <what changed>" -- <file> <file>`. Plain text messages, no backticks, no quotes inside the message.
2. Call `release_files` with the reservation ids for that repo.
3. Append one line to `$D/progress-<lane>.tsv`: `<repo>\t<done|skipped|partial>\t<commit sha or ->\t<note>`.

`$D` is `<scratch>`.

## Friction file

`$D/friction-<lane>.md`. Every time interlock, intermute, or a hook does something you did not expect (an error, a block, a confusing message, a wait longer than a minute, a tool that returned something other than what its description promised, a name that did not match), append an entry:

```
## <time> <repo> <tool or hook>
What I did:
What I expected:
What happened (verbatim):
```

Do not fix the coordination tools yourself. Do not work around them silently. Recording the friction is part of the job.

## When you are done with every repo in your list

Write `$D/report-<lane>.md`: repos done, repos skipped and why, how many times you were blocked or negotiated, and the three most annoying things about coordinating. Then run `touch $D/done-<lane>` and stop.
