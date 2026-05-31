#!/usr/bin/env bash
# Shallow-clone Subduction at the pinned SHA into .cache/subduction.
# Idempotent: re-runs are safe.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(dirname "$SCRIPT_DIR")"
source "$SCRIPT_DIR/lib.sh"
source "$SCRIPT_DIR/versions.env"

DEST="$ROOT_DIR/$SUBDUCTION_DIR"
if [ -d "$DEST/.git" ]; then
  info "Subduction already cloned at $DEST"
  # Verify the pinned SHA is checked out
  actual=$(git -C "$DEST" rev-parse HEAD)
  if [ "$actual" = "$SUBDUCTION_SHA" ]; then
    info "Already at pinned SHA $SUBDUCTION_SHA — skipping"
    exit 0
  fi
fi
ensure_dir "$ROOT_DIR/$CACHE_DIR"
info "Cloning subduction..."
git clone --depth=1 https://github.com/inkandswitch/subduction.git "$DEST" || true
(cd "$DEST" && git fetch --depth=1 origin "$SUBDUCTION_SHA" && git checkout "$SUBDUCTION_SHA") || true
info "Subduction at $(git -C "$DEST" rev-parse HEAD)"
