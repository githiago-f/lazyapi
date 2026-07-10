#!/usr/bin/env bash
set -euo pipefail

# Extract Go doc comments and structured metadata from the lazyapi codebase.
# Outputs JSON to docs/data/extracted.json

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/docs/data/extracted.json"

cd "$ROOT"
mkdir -p "$(dirname "$OUT")"

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

# Build JSON with jq (pre-installed on ubuntu-latest runners)
# Each section is built separately and merged

# ---- Package docs ----
PKG_JSON="{}"
for pkg in cmd/lazyapi internal/cli internal/config internal/model internal/store internal/env internal/app internal/app/pane/editor internal/app/pane/requests internal/app/pane/responses internal/components; do
  DOC=$(go doc "./$pkg" 2>/dev/null || true)
  PKG_JSON=$(echo "$PKG_JSON" | jq --arg pkg "$pkg" --arg doc "$DOC" '. + {($pkg): $doc}')
done

# ---- Keybindings ----
KEYMAP=$(grep -A200 'type KeyMap struct' internal/config/keymap.go | grep -oE '^\s+\w+\s+key' | awk '{print $1}' | sort | jq -R -s 'split("\n") | map(select(length > 0))')

# ---- CLI command names ----
CMDS=$(grep -oP 'case\s+"\K[^"]+' internal/cli/cli.go | jq -R -s 'split("\n") | map(select(length > 0))')

# ---- Color constants ----
COLORS=$(grep -oE '^\s+(Rosewater|Flamingo|Pink|Mauve|Red|Maroon|Peach|Yellow|Green|Teal|Sky|Saphire|Blue|Lavender|Text|Subtext[01]|Overlay[012]|Surface[012]|Base|Mantle|Crust)\s+=' internal/config/config.go | sed 's/.*\(\w\+\)\s*=.*/\1/' | jq -R -s 'split("\n") | map(select(length > 0))')

# ---- Merge all sections ----
echo "$PKG_JSON" | jq \
  --argjson keybindings "$KEYMAP" \
  --argjson commands "$CMDS" \
  --argjson colors "$COLORS" \
  '. + {_keybindings: $keybindings, _commands: $commands, _colors: $colors}' > "$TMP"

cp "$TMP" "$OUT"
echo "Extracted docs to $OUT"
