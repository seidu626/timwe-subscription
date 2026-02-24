#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
stale=0
checked=0

# Reuse this CI entrypoint to guard KrakenD list query forwarding.
python3 "$repo_root/scripts/check-krakend-query-forwarding.py" "$repo_root/krakend/krakend.json"

: "${GOCACHE:=/tmp/go-cache}"
export GOCACHE
mkdir -p "$GOCACHE"

: "${GOMODCACHE:=/tmp/go-mod-cache}"
export GOMODCACHE
mkdir -p "$GOMODCACHE"

while IFS= read -r dockerfile; do
  module_dir="$(dirname "$dockerfile")"
  if [[ ! -f "$module_dir/go.mod" ]]; then
    continue
  fi

  rel_module="${module_dir#$repo_root/}"
  if ! git -C "$repo_root" ls-files --error-unmatch "$rel_module/vendor/modules.txt" >/dev/null 2>&1; then
    echo "Skipping $rel_module (vendor not tracked in repository)"
    continue
  fi

  checked=$((checked + 1))

  if [[ ! -d "$module_dir/vendor" ]]; then
    echo "ERROR: missing vendor directory for $module_dir"
    echo "Run: (cd $module_dir && go mod vendor)"
    stale=1
    continue
  fi

  tmp_dir="$(mktemp -d)"
  trap 'rm -rf "$tmp_dir"' EXIT

  (
    cd "$module_dir"
    go mod vendor -o "$tmp_dir/vendor" >/dev/null
  )

  if ! diff -qr "$module_dir/vendor" "$tmp_dir/vendor" >/dev/null; then
    echo "ERROR: vendor drift detected in $module_dir"
    diff -qr "$module_dir/vendor" "$tmp_dir/vendor" | head -n 40 || true
    echo "Fix: (cd $module_dir && go mod vendor)"
    stale=1
  fi

  rm -rf "$tmp_dir"
  trap - EXIT
done < <(grep -rl --include='Dockerfile' -- '-mod=vendor' "$repo_root/services")

if [[ "$checked" -eq 0 ]]; then
  echo "No services with -mod=vendor found."
  exit 0
fi

if [[ "$stale" -ne 0 ]]; then
  exit 1
fi

echo "Vendor directories are in sync for all services using -mod=vendor."
