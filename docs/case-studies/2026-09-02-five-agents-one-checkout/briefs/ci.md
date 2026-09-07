# Lane brief: ci (agent sweep-ci)

First read `$D/briefs/protocol.md` and follow it for every repo. `$D` is `<scratch>`. Your lane name is `ci`.

Your repo list, in order: `$D/briefs/order-ci.txt`. Work through it top to bottom. Each repo's kind is column 2 of `$D/../job-repos.tsv` (`go`, `py`, `go+py`, `py+ts`, `sh`). Templates: `$D/templates/ci-go.yml`, `ci-py.yml`, `ci-sh.yml`.

Reserve first (protocol step 1) with patterns: `interverse/<repo>/.github/workflows/ci.yml`, `interverse/<repo>/README.md`.

## What to do in each repo `interverse/<repo>/`

1. **Pick the check that actually passes here.** From inside the repo run, in this order, and stop at the first that applies and passes:
   - `go.mod` exists: `go build ./... && go vet ./... && test -z "$(gofmt -l .)" && go test ./...` → use `ci-go.yml` (if there is also a `tests/` directory and `python3 -m pytest tests -q` passes, keep the python steps in that template; otherwise delete them).
   - `tests/` directory exists: `python3 -m pytest tests -q` → use `ci-py.yml`.
   - otherwise, or if the check above fails: `bash -n $(git ls-files '*.sh')` and the JSON check from `ci-sh.yml` → use `ci-sh.yml`.
   Record the exact command and its exit code; it goes in the commit message.
2. **`.github/workflows/ci.yml`**: if missing, write the chosen template. If it exists, keep it and only make sure the job runs on both `ubuntu-latest` and `macos-latest` (add the matrix, keep every existing step; add `if: runner.os == 'Linux'` to any harden-runner step). Do not touch other workflow files.
3. **README.md**: if no line contains `actions/workflows/ci.yml/badge.svg`, insert this line directly after the first `# ` title line:

```
[![CI](https://github.com/mistakeknot/<repo>/actions/workflows/ci.yml/badge.svg)](https://github.com/mistakeknot/<repo>/actions/workflows/ci.yml)
```

Do not change anything else in README.md; other agents are editing other parts of it at the same time.

## Check before you commit (run from inside the repo)

```
test -f .github/workflows/ci.yml && grep -q 'macos-latest' .github/workflows/ci.yml && grep -q 'ubuntu-latest' .github/workflows/ci.yml && grep -q 'actions/workflows/ci.yml/badge.svg' README.md && echo PASS
```

## Commit

`git -C interverse/<repo> add .github/workflows/ci.yml` then `git -C interverse/<repo> commit -m "ci: ubuntu and macos matrix; local check exit <code>: <command>" -- .github/workflows/ci.yml README.md`. Then protocol steps 2 and 3.
