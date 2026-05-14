#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-preipo-market}"
APP_DIR="${APP_DIR:-/opt/preipo-market-platform}"
STATE_DIR="${STATE_DIR:-/var/lib/preipo-market-platform}"
ADDR="${ADDR:-:80}"
DB_PATH="${DB_PATH:-$STATE_DIR/preipo_demo.db}"
BIN_PATH="$APP_DIR/preipo-market-platform"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEBUG="${DEBUG:-1}"
GIT_RETRIES="${GIT_RETRIES:-3}"
GIT_PULL_TIMEOUT="${GIT_PULL_TIMEOUT:-15s}"

log() {
  printf '[%s] %s\n' "$(date -Is)" "$*"
}

debug() {
  if [[ "$DEBUG" == "1" || "$DEBUG" == "true" ]]; then
    log "DEBUG: $*"
  fi
}

git_pull_with_retry() {
  local attempt=1
  local delay=3
  local status=0
  while (( attempt <= GIT_RETRIES )); do
    log "GitHub pull attempt $attempt/$GIT_RETRIES with timeout $GIT_PULL_TIMEOUT..."
    if timeout "$GIT_PULL_TIMEOUT" git -C "$ROOT_DIR" pull --ff-only; then
      log "GitHub pull succeeded."
      return 0
    fi
    status=$?
    log "GitHub pull failed with exit code $status."
    debug "Git status after failed pull:"
    git -C "$ROOT_DIR" status --short || true
    if (( attempt == GIT_RETRIES )); then
      log "GitHub pull failed after $GIT_RETRIES attempts."
      return "$status"
    fi
    log "Retrying GitHub pull in ${delay}s..."
    sleep "$delay"
    attempt=$((attempt + 1))
    delay=$((delay * 2))
  done
}

command -v git >/dev/null 2>&1 || { echo "git is required"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "go is required"; exit 1; }
command -v timeout >/dev/null 2>&1 || { echo "timeout is required"; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo "systemd is required"; exit 1; }
command -v journalctl >/dev/null 2>&1 || { echo "journalctl is required"; exit 1; }

if [[ -z "${HOME:-}" ]]; then
  if [[ "$(id -u)" -eq 0 ]]; then
    export HOME="/root"
  else
    export HOME="$APP_DIR/.home"
  fi
fi
export GOPATH="${GOPATH:-$HOME/go}"
export GOMODCACHE="${GOMODCACHE:-$GOPATH/pkg/mod}"
export GOCACHE="${GOCACHE:-$HOME/.cache/go-build}"
export GIT_TERMINAL_PROMPT="${GIT_TERMINAL_PROMPT:-0}"
mkdir -p "$GOMODCACHE" "$GOCACHE"

log "Upgrade script started."
debug "ROOT_DIR=$ROOT_DIR"
debug "APP_DIR=$APP_DIR"
debug "STATE_DIR=$STATE_DIR"
debug "SERVICE_NAME=$SERVICE_NAME"
debug "ADDR=$ADDR"
debug "DB_PATH=$DB_PATH"
debug "BIN_PATH=$BIN_PATH"
debug "HOME=$HOME"
debug "GOPATH=$GOPATH"
debug "GOMODCACHE=$GOMODCACHE"
debug "GOCACHE=$GOCACHE"
debug "PATH=$PATH"
debug "User=$(id -un 2>/dev/null || true), UID=$(id -u 2>/dev/null || true)"
debug "git path=$(command -v git)"
debug "go path=$(command -v go)"
debug "git version=$(git --version)"
debug "go version=$(go version)"
debug "git branch=$(git -C "$ROOT_DIR" rev-parse --abbrev-ref HEAD 2>/dev/null || true)"
debug "git commit=$(git -C "$ROOT_DIR" rev-parse --short=12 HEAD 2>/dev/null || true)"
debug "git remote origin=$(git -C "$ROOT_DIR" remote get-url origin 2>/dev/null || true)"
debug "git status before upgrade:"
git -C "$ROOT_DIR" status --short || true

if [[ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]]; then
  log "working tree is not clean; commit or stash local changes before upgrade"
  git -C "$ROOT_DIR" status --short
  exit 1
fi

log "Updating code..."
git_pull_with_retry

cd "$ROOT_DIR"

log "Running tests..."
go test ./...

COMMIT_EPOCH="$(git -C "$ROOT_DIR" log -1 --format=%ct)"
COMMIT_DATETIME="$(TZ=Asia/Shanghai date -d "@$COMMIT_EPOCH" "+%Y-%m-%d %H:%M")"
COMMIT_ID="$(git -C "$ROOT_DIR" rev-parse --short=8 HEAD)"
BRANCH_NAME="$(git -C "$ROOT_DIR" rev-parse --abbrev-ref HEAD)"

LDFLAGS="-s -w"
LDFLAGS="$LDFLAGS -X 'pre-ipo-market-platform/internal/buildinfo.commitDateTime=$COMMIT_DATETIME'"
LDFLAGS="$LDFLAGS -X 'pre-ipo-market-platform/internal/buildinfo.commitID=$COMMIT_ID'"
LDFLAGS="$LDFLAGS -X 'pre-ipo-market-platform/internal/buildinfo.branchName=$BRANCH_NAME'"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

log "Building binary..."
go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_DIR/preipo-market-platform" "$ROOT_DIR"

log "Installing binary to $BIN_PATH..."
sudo install -d -m 0755 "$APP_DIR"
sudo install -d -m 0755 "$STATE_DIR"
sudo install -m 0755 "$TMP_DIR/preipo-market-platform" "$BIN_PATH"

log "Writing systemd service $SERVICE_NAME..."
cat <<UNIT | sudo tee "/etc/systemd/system/$SERVICE_NAME.service" >/dev/null
[Unit]
Description=Pre-IPO Market Platform
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$APP_DIR
Environment=PREIPO_UPGRADE_SCRIPT=$ROOT_DIR/upgrade.sh
ExecStart=$BIN_PATH --addr $ADDR --db $DB_PATH
Restart=on-failure
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ReadWritePaths=$APP_DIR $STATE_DIR $ROOT_DIR

[Install]
WantedBy=multi-user.target
UNIT
sudo systemctl daemon-reload
sudo systemctl enable "$SERVICE_NAME"

log "Starting service $SERVICE_NAME..."
sudo systemctl restart "$SERVICE_NAME"

log "Service status:"
sudo systemctl --no-pager --full status "$SERVICE_NAME"

log "Last 30 service logs:"
sudo journalctl -u "$SERVICE_NAME" -n 30 --no-pager

log "Upgrade complete: Code by Yuhao@jiansutech.com - $COMMIT_DATETIME - $COMMIT_ID - $BRANCH_NAME"
