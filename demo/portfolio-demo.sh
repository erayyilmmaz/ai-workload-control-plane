#!/usr/bin/env bash
# Runs the same disposable, real-Kubernetes lifecycle that CI validates, with
# visible 16-step narration. It never reuses the caller's current kubeconfig.
set -euo pipefail
cd "$(dirname "$0")/.."

echo 'AWCP portfolio demo: building local images and running a disposable kind lifecycle.'
echo 'It creates a random awcp-e2e-* cluster and deletes it, including on failure.'
echo 'Grafana is optional; the demo proves the underlying authenticated metrics endpoint.'
exec make e2e
