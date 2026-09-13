#!/usr/bin/env bash
# Keep CI's secrets/permission/pinning boundary deterministic without calling GitHub.
set -euo pipefail
cd "$(dirname "$0")/../.."
workflow=.github/workflows/ci.yml
bootstrap=.github/actions/bootstrap/action.yml
dependabot=.github/dependabot.yml

test -s "$workflow" && test -s "$bootstrap" && test -s "$dependabot"
! rg -q 'pull_request_target|secrets\.' "$workflow" "$bootstrap"
rg -q '^  contents: read$' "$workflow"
rg -q '^concurrency:$' "$workflow"
rg -q '^  cancel-in-progress: true$' "$workflow"
for job in format lint vet generate-check manifest-check unit-test envtest build docker-build supply-chain e2e; do
  rg -q "^  $job:$" "$workflow"
done
test "$(rg -c 'timeout-minutes:' "$workflow")" = 11
test "$(rg -c 'persist-credentials: false' "$workflow")" = 11
test "$(rg -c 'actions/checkout@[0-9a-f]{40}' "$workflow")" = 11
rg -q 'actions/cache@[0-9a-f]{40}' "$bootstrap"
rg -q 'make e2e' "$workflow"
rg -q 'make vuln' "$workflow"
rg -q 'git status --porcelain --untracked-files=all' "$workflow"
rg -q 'FROM .+@sha256:[0-9a-f]{64}' Dockerfile examples/demo-app/Dockerfile
rg -q 'package-ecosystem: gomod' "$dependabot"
rg -q 'package-ecosystem: github-actions' "$dependabot"
echo 'PASS: CI has pinned actions, read-only permissions, stable gates, cache, concurrency and no secret-bearing PR path'
