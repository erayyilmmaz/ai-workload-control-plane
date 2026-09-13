#!/usr/bin/env bash
# Keep CI's secrets/permission/pinning boundary deterministic without calling GitHub.
set -euo pipefail
cd "$(dirname "$0")/../.."
workflow=.github/workflows/ci.yml
bootstrap=.github/actions/bootstrap/action.yml
dependabot=.github/dependabot.yml

test -s "$workflow" && test -s "$bootstrap" && test -s "$dependabot"
! grep -Eq 'pull_request_target|secrets\.' "$workflow" "$bootstrap"
grep -Eq '^  contents: read$' "$workflow"
grep -Eq '^concurrency:$' "$workflow"
grep -Eq '^  cancel-in-progress: true$' "$workflow"
for job in format lint vet generate-check manifest-check unit-test envtest build docker-build supply-chain e2e; do
  grep -Eq "^  $job:$" "$workflow"
done
test "$(grep -Ec 'timeout-minutes:' "$workflow")" = 11
test "$(grep -Ec 'persist-credentials: false' "$workflow")" = 11
test "$(grep -Ec 'actions/checkout@[0-9a-f]{40}' "$workflow")" = 11
grep -Eq 'actions/cache@[0-9a-f]{40}' "$bootstrap"
grep -Eq 'make e2e' "$workflow"
grep -Eq 'make vuln' "$workflow"
grep -Eq 'git status --porcelain --untracked-files=all' "$workflow"
grep -Eq 'FROM .+@sha256:[0-9a-f]{64}' Dockerfile examples/demo-app/Dockerfile
grep -Eq 'package-ecosystem: gomod' "$dependabot"
grep -Eq 'package-ecosystem: github-actions' "$dependabot"
echo 'PASS: CI has pinned actions, read-only permissions, stable gates, cache, concurrency and no secret-bearing PR path'
