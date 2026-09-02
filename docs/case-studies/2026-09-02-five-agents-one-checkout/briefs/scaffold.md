# Lane brief: scaffold (agent sweep-scaffold)

First read `$D/briefs/protocol.md` and follow it for every repo. `$D` is `<scratch>`. Your lane name is `scaffold`.

Your repo list, in order: `$D/briefs/order-scaffold.txt`. Work through it top to bottom. Templates are in `$D/templates/`.

## What to do in each repo `interverse/<repo>/`

Reserve first (protocol step 1) with patterns: `interverse/<repo>/LICENSE`, `interverse/<repo>/CONTRIBUTING.md`, `interverse/<repo>/SECURITY.md`, `interverse/<repo>/CODE_OF_CONDUCT.md`, `interverse/<repo>/README.md`.

1. **LICENSE**: if missing, copy `$D/templates/LICENSE` verbatim. If present, leave it.
2. **CONTRIBUTING.md**: if missing, start from `$D/templates/CONTRIBUTING.md`, replace `interlock` with `<repo>`, and replace the first bullet under "Before you open a PR" with this repo's real check: if `go.mod` exists, `go build ./... && go vet ./... && go test ./...`; else if a `tests/` directory exists, `python3 -m pytest tests -q`; else `bash -n` over the repo's shell scripts. If present, leave it.
3. **SECURITY.md**: if missing, take `$D/templates/SECURITY.md`, keep the first paragraph verbatim, and replace the "Threat model" paragraph with two to four plain sentences about this plugin, written from its README and its `.claude-plugin/plugin.json`: what it runs (hooks, an MCP server, slash commands, skills), what it reads or writes on disk, what network services it talks to, and what it never does. No adjectives, no marketing, no claims you cannot see in the repo. If present, leave it.
4. **CODE_OF_CONDUCT.md**: if missing, copy `$D/templates/CODE_OF_CONDUCT.md` verbatim.
5. **README.md**: if it has no heading containing "Contributing", append at the end:

```
## Contributing

Bug reports and pull requests are welcome; see [CONTRIBUTING.md](CONTRIBUTING.md). Report vulnerabilities as described in [SECURITY.md](SECURITY.md), not in a public issue.
```

Do not change anything else in README.md. Other agents are editing other parts of it at the same time.

## Check before you commit (run from inside the repo)

```
test -f LICENSE && test -f CONTRIBUTING.md && test -f SECURITY.md && test -f CODE_OF_CONDUCT.md && grep -q -i 'CONTRIBUTING.md' README.md && echo PASS
```

## Commit

`git -C interverse/<repo> add LICENSE CONTRIBUTING.md SECURITY.md CODE_OF_CONDUCT.md README.md` (only the ones you created or changed) then `git -C interverse/<repo> commit -m "scaffold: license, contributing, security, code of conduct" -- LICENSE CONTRIBUTING.md SECURITY.md CODE_OF_CONDUCT.md README.md`. Then protocol steps 2 and 3 (release, progress line).
