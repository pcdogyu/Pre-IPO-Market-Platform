#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-preipo-market}"
APP_DIR="${APP_DIR:-/opt/preipo-market-platform}"
STATE_DIR="${STATE_DIR:-/var/lib/preipo-market-platform}"
ADDR="${ADDR:-:80}"
DB_PATH="${DB_PATH:-$STATE_DIR/preipo_demo.db}"
BIN_PATH="$APP_DIR/preipo-market-platform"

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

command -v go >/dev/null 2>&1 || { echo "go is required"; exit 1; }
command -v git >/dev/null 2>&1 || { echo "git is required"; exit 1; }
command -v systemctl >/dev/null 2>&1 || { echo "systemd is required"; exit 1; }

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

go build -trimpath -ldflags "$LDFLAGS" -o "$TMP_DIR/preipo-market-platform" "$ROOT_DIR"

install -d -m 0755 "$APP_DIR"
install -d -m 0755 "$STATE_DIR"
install -m 0755 "$TMP_DIR/preipo-market-platform" "$BIN_PATH"

cat > "/etc/systemd/system/$SERVICE_NAME.service" <<UNIT
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

systemctl daemon-reload
systemctl enable "$SERVICE_NAME"
systemctl restart "$SERVICE_NAME"

if [[ "$ADDR" == :* ]]; then
  SERVICE_URL="http://localhost$ADDR"
else
  SERVICE_URL="http://$ADDR"
fi

echo "$SERVICE_NAME is running."
echo "URL: $SERVICE_URL"
echo "Footer: Code by Yuhao@jiansutech.com - $COMMIT_DATETIME - $COMMIT_ID - $BRANCH_NAME"
echo "Logs: sudo journalctl -u $SERVICE_NAME -f"
