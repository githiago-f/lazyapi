#!/usr/bin/env bash
set -euo pipefail

# Extract Go doc comments and structured metadata from the lazyapi codebase.
# Outputs JSON to docs/data/extracted.json

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/docs/data/extracted.json"

cd "$ROOT"

TMP=$(mktemp)
trap 'rm -f "$TMP"' EXIT

# ---- Package docs ----
echo "{" > "$TMP"

FIRST=1
for pkg in cmd/lazyapi internal/cli internal/config internal/model internal/store internal/env internal/app internal/app/pane/editor internal/app/pane/requests internal/app/pane/responses internal/components; do
  DOC=$(go doc "./$pkg" 2>/dev/null || true)
  # trim leading/trailing whitespace
  DOC=$(echo "$DOC" | sed 's/^[[:space:]]*//; s/[[:space:]]*$//')
  # escape for JSON
  DOC=$(echo "$DOC" | python3 -c 'import sys,json; print(json.dumps(sys.stdin.read()))' 2>/dev/null || echo "")

  if [ "$FIRST" -eq 1 ]; then
    FIRST=0
  else
    echo "," >> "$TMP"
  fi

  echo "\"$pkg\": $DOC" >> "$TMP"
done

# ---- Keybindings from config/keymap.go ----
KEYMAP=$(grep -A200 'type KeyMap struct' internal/config/keymap.go | grep -E '^\s+\w+\s+key\b' | sed 's/.*\(\w\+\)\s\+key.*/\1/' | sort | python3 -c '
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
print(json.dumps(lines))
' 2>/dev/null || echo "[]")

echo "," >> "$TMP"
echo "\"_keybindings\": $KEYMAP" >> "$TMP"

# ---- CLI command names from cli.go ----
CMDS=$(grep -oP 'case\s+"\K[^"]+' internal/cli/cli.go | python3 -c '
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
print(json.dumps(lines))
' 2>/dev/null || echo "[]")

echo "," >> "$TMP"
echo "\"_commands\": $CMDS" >> "$TMP"

# ---- Color constants from config.go ----
COLORS=$(grep -E '^\s+(Rosewater|Flamingo|Pink|Mauve|Red|Maroon|Peach|Yellow|Green|Teal|Sky|Saphire|Blue|Lavender|Text|Subtext[01]|Overlay[012]|Surface[012]|Base|Mantle|Crust)\s+=' internal/config/config.go | sed 's/.*\(\w\+\)\s*=.*/\1/' | python3 -c '
import sys, json
lines = [l.strip() for l in sys.stdin if l.strip()]
print(json.dumps(lines))
' 2>/dev/null || echo "[]")

echo "," >> "$TMP"
echo "\"_colors\": $COLORS" >> "$TMP"

echo "}" >> "$TMP"

# Validate JSON
python3 -c "import json; json.load(open('$TMP'))" 2>/dev/null || {
  echo "ERROR: generated invalid JSON" >&2
  exit 1
}

cp "$TMP" "$OUT"
echo "Extracted docs to $OUT"
