#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-preipo-market}"
APP_DIR="${APP_DIR:-/opt/preipo-market-platform}"
STATE_DIR="${STATE_DIR:-/var/lib/preipo-market-platform}"
ADDR="${ADDR:-:8080}"
DB_PATH="${DB_PATH:-$STATE_DIR/preipo_demo.db}"
BIN_PATH="$APP_DIR/preipo-market-platform"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

command -v git >/dev/null 2>&1 || { echo "git is required"; exit 1; }
command -v go >/dev/null 2>&1 || { echo "go is required"; exit 1; }
command -v timeout >/dev/null 2>&1 || { echo "timeout is required"; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo "systemd is required"; exit 1; }
command -v journalctl >/dev/null 2>&1 || { echo "journalctl is required"; exit 1; }

if [[ -n "$(git -C "$ROOT_DIR" status --porcelain)" ]]; then
  echo "working tree is not clean; commit or stash local changes before upgrade"
  git -C "$ROOT_DIR" status --short
  exit 1
fi

echo "Updating code..."
timeout 60s git -C "$ROOT_DIR" pull --ff-only

cd "$ROOT_DIR"

echo "Deleting Go caches..."
go clean -cache -testcache

echo "Running tests..."
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

echo "Building binary..."
go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_DIR/preipo-market-platform" "$ROOT_DIR"

echo "Installing binary to $BIN_PATH..."
sudo install -d -m 0755 "$APP_DIR"
sudo install -m 0755 "$TMP_DIR/preipo-market-platform" "$BIN_PATH"

if ! sudo systemctl cat "$SERVICE_NAME" >/dev/null 2>&1; then
  echo "Creating systemd service $SERVICE_NAME..."
  cat <<UNIT | sudo tee "/etc/systemd/system/$SERVICE_NAME.service" >/dev/null
[Unit]
Description=Pre-IPO Market Platform
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$APP_DIR
ExecStart=$BIN_PATH --addr $ADDR --db $DB_PATH
Restart=on-failure
RestartSec=5
DynamicUser=yes
StateDirectory=$(basename "$STATE_DIR")
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true

[Install]
WantedBy=multi-user.target
UNIT
  sudo systemctl daemon-reload
  sudo systemctl enable "$SERVICE_NAME"
fi

echo "Starting service $SERVICE_NAME..."
sudo systemctl restart "$SERVICE_NAME"

echo "Service status:"
sudo systemctl --no-pager --full status "$SERVICE_NAME"

echo "Last 30 service logs:"
sudo journalctl -u "$SERVICE_NAME" -n 30 --no-pager

echo "Upgrade complete: Code by Yuhao@jiansutech.com - $COMMIT_DATETIME - $COMMIT_ID - $BRANCH_NAME"
