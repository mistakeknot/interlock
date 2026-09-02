# Lane brief: jargon (agent sweep-LANE)

First read `$D/briefs/protocol.md` and follow it for every repo. `$D` is `<scratch>`. Your lane name is `LANE`.

Your repo list, in order: `$D/briefs/order-LANE.txt`. Work through it top to bottom.

## Files you may touch in each repo `interverse/<repo>/`

`README.md`, every `*.md` under `docs/`, and the `description` strings in `.claude-plugin/plugin.json` and `kimi.plugin.json`. Nothing else. Never touch `hooks/`, `scripts/`, `commands/`, `skills/`, `agents/`, or code.

Reserve first (protocol step 1) with patterns: `interverse/<repo>/README.md`, `interverse/<repo>/docs/**/*.md`, `interverse/<repo>/.claude-plugin/plugin.json`, `interverse/<repo>/kimi.plugin.json`.

## What to change

Find candidates with, from inside the repo:

```
grep -rIn -E '/home/mk|~|/root/projects|Demarch|PHILOSOPHY|[Cc]lavain|\bbd\b|\bbeads?\b' -- README.md docs .claude-plugin/plugin.json kimi.plugin.json 2>/dev/null
```

Then, line by line:

1. **Personal paths** (`/home/mk/...`, `~/...`, `/root/projects/...`): replace with `~/projects/...` when the rest of the path is generic, otherwise `/path/to/...`.
2. **Demarch**: it is an internal program name a stranger has never heard of. Say what the sentence means without it, or delete the sentence if it only exists to name it.
3. **Clavain** in prose: it is the maintainer's private orchestration plugin. Replace with "your orchestration layer" or "the host plugin", or delete the clause. Keep identifiers a program consumes exactly as they are: `clavain:` command names, `/clavain:...` slash commands, environment variable names, file paths that exist in the repo.
4. **PHILOSOPHY** references: delete the reference.
5. **bd / bead / beads** in prose: replace with "issue" or "task" when it is prose for a reader. Leave commands, flags, file names, and code blocks alone.

Make the smallest edit that removes the term. Do not rewrite paragraphs, do not improve style, do not change meaning, do not touch anything the grep did not flag.

## Check before you commit (run from inside the repo)

```
test -z "$(grep -rIl -E '/home/mk|~|/root/projects|Demarch|PHILOSOPHY' -- README.md docs .claude-plugin/plugin.json kimi.plugin.json 2>/dev/null)" && test -z "$(grep -rIn -E '[Cc]lavain' -- README.md docs 2>/dev/null | grep -v -E 'clavain:[a-z-]+|/clavain:')" && echo PASS
```

If a remaining hit is an identifier a program consumes, leave it and say so in your progress line.

## Commit

`git -C interverse/<repo> commit -m "jargon: scrub personal paths and internal names from user-facing docs" -- README.md docs .claude-plugin/plugin.json kimi.plugin.json` (only the paths you changed; drop the ones you did not touch). Then protocol steps 2 and 3.
