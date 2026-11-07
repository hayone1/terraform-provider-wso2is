#!/usr/bin/env bash
# Runs `terraform validate` (or `tofu validate` with TF_CLI=tofu) on one example
# directory against a locally built provider.
# Usage: scripts/validate-example.sh <example-dir> <dir-with-provider-binary>
set -euo pipefail
dir=$1
bin=$2
cli=${TF_CLI:-terraform}
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
cp "$dir"/*.tf "$work"/
if ! grep -q required_providers "$work"/*.tf; then
  printf 'terraform {\n  required_providers {\n    wso2is = { source = "hayone1/wso2is" }\n  }\n}\n' > "$work/versions.tf"
fi
cat > "$work/terraformrc" <<RC
provider_installation {
  dev_overrides {
    "hayone1/wso2is" = "$bin"
  }
  direct {}
}
RC
if ! out=$(cd "$work" && TF_CLI_CONFIG_FILE="$work/terraformrc" "$cli" validate -no-color 2>&1); then
  echo "::error::$dir"
  echo "$out" | grep -v -e 'Provider development overrides' -e 'The following provider' -e '^ - hayone1' -e 'The behavior may' -e 'published releases'
  exit 1
fi
echo "ok  $dir"
