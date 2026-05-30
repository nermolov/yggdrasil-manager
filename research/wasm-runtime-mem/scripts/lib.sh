#!/usr/bin/env bash
# Common helpers sourced by all build scripts.
set -euo pipefail

# die prints a message to stderr and exits 1.
die() { echo "ERROR: $*" >&2; exit 1; }

# info prints a timestamped message to stderr.
info() { echo "[$(date +%H:%M:%S)] $*" >&2; }

# require_cmd asserts that a command exists on PATH.
require_cmd() {
  command -v "$1" >/dev/null 2>&1 || die "Required command not found: $1"
}

# ensure_dir creates a directory if it does not exist.
ensure_dir() { mkdir -p "$1"; }
