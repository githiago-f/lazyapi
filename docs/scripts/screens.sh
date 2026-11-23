#!/usr/bin/env bash
# Screenshots of the real TUI for the docs (docs/public/screens/*.svg).
#
# Runs lazyapi on examples/openapi.yml in a detached tmux session against
# `lazyapi mock`, drives it with keys, captures the screen with its colors
# (tmux capture-pane -e) and renders each capture with ansi2svg.mjs.
#
#   bash docs/scripts/screens.sh            # needs go, tmux, node
#
# Keep the scenes in sync with the docs that embed them.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OUT="$ROOT/docs/public/screens"
COLS=140
ROWS=36
PORT=4010 # the example's "local" environment points here

WORK="$(mktemp -d)"
SESSION="lazyapi-screens-$$"
MOCK_PID=""

cleanup() {
  tmux kill-session -t "$SESSION" 2>/dev/null || true
  [[ -n "$MOCK_PID" ]] && kill "$MOCK_PID" 2>/dev/null || true
  rm -rf "$WORK"
}
trap cleanup EXIT

cd "$ROOT"
go build -o "$WORK/lazyapi" ./cmd/lazyapi
mkdir -p "$OUT"

# The TUI writes history and drafts to .lazyapi/.local next to the spec:
# use a copy of the example so the repo stays clean and history starts empty.
cp -r examples "$WORK/examples"
SPEC="$WORK/examples/openapi.yml"

if curl -s -o /dev/null "http://127.0.0.1:$PORT/health"; then
  echo "port $PORT is busy: stop whatever is listening there first" >&2
  exit 1
fi
"$WORK/lazyapi" mock "$SPEC" --port "$PORT" >/dev/null 2>&1 &
MOCK_PID=$!
for _ in $(seq 50); do
  curl -s -o /dev/null "http://127.0.0.1:$PORT/health" && break
  sleep 0.1
done

start() {
  tmux kill-session -t "$SESSION" 2>/dev/null || true
  tmux new-session -d -s "$SESSION" -x "$COLS" -y "$ROWS" -c "$WORK/examples" \
    "TERM=xterm-256color COLORTERM=truecolor $WORK/lazyapi openapi.yml"
  sleep 1.5
}

keys() {
  tmux send-keys -t "$SESSION" "$@"
  sleep 0.4
}

# shot <name> <title>: waits for notifications to fade, then captures.
shot() {
  sleep "${WAIT:-0.6}"
  tmux capture-pane -e -p -t "$SESSION" >"$WORK/$1.ansi"
  node "$ROOT/docs/scripts/ansi2svg.mjs" "$WORK/$1.ansi" "$OUT/$1.svg" \
    --cols "$COLS" --rows "$ROWS" --title "$2"
  echo "  $OUT/$1.svg"
}

echo "capturing:"
start
shot request-list "lazyapi openapi.yml — request list"

keys j j j j j # GET /users/{id}
keys s
WAIT=4 shot list-preview "lazyapi — send from the list (s)"

keys ']' ']' ']' ']'
shot list-checks "lazyapi — Checks tab"

keys Enter
keys M-Enter
WAIT=4 shot request-editor "lazyapi — request editor"

keys Escape
keys Escape
keys Escape
keys C-k
shot palette "lazyapi — command palette (ctrl+k)"

tmux send-keys -t "$SESSION" -l "all operations" # Run All operations (writes included)
sleep 0.4
keys Enter
keys y
WAIT=3 shot runner "lazyapi — runner (R)"

keys Escape
keys H
shot history "lazyapi — history (H)"

keys Escape
keys '?'
shot help "lazyapi — keys (?)"
