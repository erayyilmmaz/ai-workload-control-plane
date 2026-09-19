#!/usr/bin/env bash
# Install only checksum-verified, pinned tools into this checkout.
set -euo pipefail
cd "$(dirname "$0")/.."
platform="$(uname -s | tr '[:upper:]' '[:lower:]')-$(uname -m)"
platform="${platform/x86_64/amd64}"
platform="${platform/aarch64/arm64}"
lock=toolchain.lock.json
if ! jq -e --arg p "$platform" '.binaryAssets[$p]' "$lock" >/dev/null; then
  echo "Unsupported bootstrap platform: $platform (see docs/development.md)" >&2
  exit 1
fi
mkdir -p .tools/downloads .tools/bin .tools/envtest
for tool in "${@:-go kubebuilder envtest}"; do
  # Multiple tools may be passed as one space-separated default value.
  for name in $tool; do
    url="$(jq -er --arg p "$platform" --arg t "$name" '.binaryAssets[$p][$t].url' "$lock")"
    expected="$(jq -er --arg p "$platform" --arg t "$name" '.binaryAssets[$p][$t].sha256' "$lock")"
    archive=".tools/downloads/$name-$expected"
    if [ ! -f "$archive" ]; then
      curl --fail --location --retry 3 "$url" -o "$archive.part"
      mv "$archive.part" "$archive"
    fi
    actual="$(shasum -a 256 "$archive" | cut -d ' ' -f 1)"
    if [ "$actual" != "$expected" ]; then
      echo "Checksum mismatch: $name; refusing to execute $archive" >&2
      exit 1
    fi
    case "$name" in
      go) tar -xzf "$archive" -C .tools ;;
      envtest) tar -xzf "$archive" -C .tools/envtest --strip-components=2 ;;
      terraform) unzip -p "$archive" terraform > .tools/bin/terraform; chmod +x .tools/bin/terraform ;;
      trivy) tar -xzf "$archive" -C .tools/bin trivy; chmod +x .tools/bin/trivy ;;
      *) cp "$archive" ".tools/bin/$name"; chmod +x ".tools/bin/$name" ;;
    esac
    echo "Verified and installed $name ($platform)"
  done
done
