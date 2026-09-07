#!/usr/bin/env bash
# validate.sh — run the plan's per-lane acceptance lines in every job repo.
# Prints one TSV row per repo: repo, scaffold, jargon, release, ci, dirty, commits, subjects-ok
# Exit 0 only when every cell is ok.
D="$(cd "$(dirname "$0")" && pwd)"
cd <checkout>/interverse || exit 2
fail=0
printf 'repo\tscaffold\tjargon\trelease\tci\tdirty\tcommits\tsubjects\n'
while IFS=$'\t' read -r r k h m; do
  [ -d "$r" ] || continue
  (
    cd "$r" || exit 1
    s=ok; j=ok; rl=ok; c=ok; d=ok; sub=ok
    test -f LICENSE && test -f CONTRIBUTING.md && test -f SECURITY.md && test -f CODE_OF_CONDUCT.md && { [ ! -f README.md ] || grep -q -i 'CONTRIBUTING.md' README.md; } || s=FAIL
    test -z "$(grep -rIl -E '/home/mk|~|/root/projects|Demarch|PHILOSOPHY' -- README.md docs .claude-plugin/plugin.json kimi.plugin.json 2>/dev/null)" && test -z "$(grep -rIn -E '[Cc]lavain' -- README.md docs 2>/dev/null | grep -v -E 'clavain:[a-z-]+|/clavain:')" || j=FAIL
    test -f CHANGELOG.md && grep -q -E '^## ' CHANGELOG.md && python3 -c "import json,os;a=json.load(open('.claude-plugin/plugin.json'))['version'];b=json.load(open('kimi.plugin.json'))['version'] if os.path.exists('kimi.plugin.json') else a;raise SystemExit(0 if a==b else 1)" && { [ ! -f README.md ] || grep -q -E '^## Install' README.md; } || rl=FAIL
    test -f .github/workflows/ci.yml && grep -q 'macos-latest' .github/workflows/ci.yml && grep -q 'ubuntu-latest' .github/workflows/ci.yml && { [ ! -f README.md ] || grep -q 'actions/workflows/ci.yml/badge.svg' README.md; } || c=FAIL
    [ -z "$(git status --porcelain)" ] || d=DIRTY
    base=main; git show-ref -q --verify refs/heads/main || base=master
    n=$(git rev-list --count "$base..sweep/2026-09-02" 2>/dev/null || echo 0)
    bad=$(git log --format=%s "$base..sweep/2026-09-02" 2>/dev/null | grep -v -E '^(scaffold|jargon|release|ci|Fix the hook|Post-commit|fetch_inbox)' | head -1)
    [ -z "$bad" ] || sub="FAIL:$bad"
    printf '%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' "$r" "$s" "$j" "$rl" "$c" "$d" "$n" "$sub"
    [ "$s$j$rl$c$d$sub" = "okokokokokok" ] || exit 1
  ) || fail=1
done < "$D/../job-repos.tsv"
exit $fail
