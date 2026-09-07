# Lane brief: release (agent sweep-release)

First read `$D/briefs/protocol.md` and follow it for every repo. `$D` is `<scratch>`. Your lane name is `release`.

Your repo list, in order: `$D/briefs/order-release.txt`. Work through it top to bottom.

Reserve first (protocol step 1) with patterns: `interverse/<repo>/CHANGELOG.md`, `interverse/<repo>/kimi.plugin.json`, `interverse/<repo>/README.md`.

## What to do in each repo `interverse/<repo>/`

1. **CHANGELOG.md**. If missing, create it in Keep a Changelog form:

```
# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

- Public-release sweep: contributing and security docs, CI on ubuntu and macos, docs cleanup.

## [<version>] - <date>

- <two to five bullets>
```

`<version>` is `version` from `.claude-plugin/plugin.json`. `<date>` is `git log -1 --format=%ad --date=short -- .claude-plugin/plugin.json`. The bullets summarize `git log --oneline -15` in words a user would understand (what changed for them, not which files moved). If CHANGELOG.md exists, only add the `## [Unreleased]` entry above if there is none, and leave the rest.

2. **kimi.plugin.json**: if it exists and its `version` differs from `.claude-plugin/plugin.json`'s, set it to the same string. Change nothing else in it.

3. **README.md**: if it has no `## Install` or `## Installation` heading, insert this after the first paragraph under the title:

````
## Install

```bash
/plugin marketplace add mistakeknot/interagency-marketplace
/plugin install <repo>
```
````

If `.claude-plugin/plugin.json` has an `mcpServers` object, add after that block one sentence, "Any MCP client can run the server directly:", followed by a JSON code block containing that `mcpServers` object with every `${CLAUDE_PLUGIN_ROOT}` replaced by `/path/to/<repo>`. Do not change anything else in README.md; other agents are editing other parts of it at the same time.

## Check before you commit (run from inside the repo)

```
test -f CHANGELOG.md && grep -q -E '^## ' CHANGELOG.md && python3 -c "import json,os;a=json.load(open('.claude-plugin/plugin.json'))['version'];b=json.load(open('kimi.plugin.json'))['version'] if os.path.exists('kimi.plugin.json') else a;raise SystemExit(0 if a==b else 1)" && grep -q -E '^## Install' README.md && echo PASS
```

## Commit

`git -C interverse/<repo> add CHANGELOG.md` then `git -C interverse/<repo> commit -m "release: changelog, version agreement, install section" -- CHANGELOG.md kimi.plugin.json README.md` (drop the paths you did not touch). Then protocol steps 2 and 3.
