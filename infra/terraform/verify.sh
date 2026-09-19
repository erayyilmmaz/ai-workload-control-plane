#!/usr/bin/env bash
# Validate the reference configuration without a backend, cloud credentials or apply.
set -euo pipefail

terraform_bin="${1:?usage: verify.sh <terraform-bin> <trivy-bin>}"
trivy_bin="${2:?usage: verify.sh <terraform-bin> <trivy-bin>}"
root="$(cd "$(dirname "$0")" && pwd)"

test -s "$root/.terraform.lock.hcl"
grep -Eq 'source *= *"hashicorp/aws"' "$root/versions.tf"
! grep -REn --include='*.tf' 'provider "kubernetes"|resource "kubernetes_|resource "argocd_|AIWorkload' "$root"
test "$("$terraform_bin" version -json | jq -r '.terraform_version')" = "1.16.2"

"$terraform_bin" -chdir="$root" fmt -check -recursive
"$terraform_bin" -chdir="$root" init -backend=false -input=false -lockfile=readonly -no-color
"$terraform_bin" -chdir="$root" validate -no-color
"$trivy_bin" config --exit-code 1 --severity HIGH,CRITICAL --skip-dirs .terraform "$root"

echo "PASS: Terraform formatting, locked-provider initialization, validation and high/critical IaC scan passed without a cloud apply"
