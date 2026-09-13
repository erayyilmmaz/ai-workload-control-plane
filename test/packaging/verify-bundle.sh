#!/usr/bin/env bash
# Validate the generated release artefacts structurally; this never contacts a cluster.
set -euo pipefail
cd "$(dirname "$0")/../.."
make release-bundle
release_dir=dist/release
install="$release_dir/awcp-crds.yaml"
operator="$release_dir/awcp-operator.yaml"
uninstall="$release_dir/awcp-operator-uninstall.yaml"
checksums="$release_dir/SHA256SUMS"

test -s "$install" && test -s "$operator" && test -s "$uninstall" && test -s "$checksums"
rg -q '^kind: CustomResourceDefinition$' "$install"
! rg -q '^kind: (Deployment|Namespace)$' "$install"
rg -q '^kind: Deployment$' "$operator"
rg -q '^kind: CustomResourceDefinition$' "$operator"
rg -q '^kind: Namespace$' "$operator"
rg -q '^kind: Deployment$' "$uninstall"
! rg -q '^kind: (CustomResourceDefinition|Namespace)$' "$uninstall"
test "$(rg -c 'awcp-.*\.yaml$' "$checksums")" = 3
echo 'PASS: release CRD/operator/uninstall packages are complete and uninstall excludes CRDs and Namespaces'
