#!/usr/bin/env bash
# Sets subcategory frontmatter in generated docs.
# Run after tfplugindocs generate.

set -euo pipefail

DOCS_DIR="$(cd "$(dirname "$0")/.." && pwd)/docs"

set_subcategory() {
  local file="$1" subcategory="$2" tmpfile
  tmpfile=$(mktemp)
  sed "s/subcategory: \"\"/subcategory: \"${subcategory}\"/" "$file" > "$tmpfile"
  mv "$tmpfile" "$file"
}

# Format: type|name|subcategory
while IFS='|' read -r type name subcategory; do
  [ -z "$type" ] && continue
  file="$DOCS_DIR/$type/$name.md"
  if [ -f "$file" ]; then
    set_subcategory "$file" "$subcategory"
  else
    echo "Warning: $file not found" >&2
  fi
done <<'EOF'
resources|team|Teams
resources|team_membership|Teams
resources|team_resource_link|Teams
resources|cloud_aws|Cloud
resources|cloud_azure|Cloud
resources|cloud_gcp|Cloud
resources|cloud_kubernetes|Cloud
resources|code_repo_config|Code Repositories
resources|container_config|Containers
resources|domain|Domains
resources|webhook|Webhooks
resources|custom_sast_rule|Custom SAST Rules
resources|zen_app|Zen Firewall
resources|zen_app_blocking|Zen Firewall
resources|zen_app_bot_lists|Zen Firewall
resources|zen_app_countries|Zen Firewall
resources|zen_app_ip_blocklist|Zen Firewall
resources|zen_app_ip_lists|Zen Firewall
data-sources|teams|Teams
data-sources|users|Teams
data-sources|clouds|Cloud
data-sources|code_repos|Code Repositories
data-sources|containers|Containers
data-sources|domains|Domains
data-sources|zen_apps|Zen Firewall
EOF
