#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
snapshot="$(mktemp -d "${TMPDIR:-/tmp}/awcp-generated.XXXXXX")"
trap 'rm -r "$snapshot"' EXIT
cp -R api "$snapshot/api"
cp -R config/crd "$snapshot/crd"
cp config/rbac/role.yaml "$snapshot/role.yaml"
make generate manifests
diff -ru "$snapshot/api" api
diff -ru "$snapshot/crd" config/crd
diff -u "$snapshot/role.yaml" config/rbac/role.yaml
echo 'CRD, RBAC and DeepCopy generation is unchanged'
