#!/usr/bin/env bash
set -euo pipefail

SERVICE_NAME="${SERVICE_NAME:-preipo-market}"
APP_DIR="${APP_DIR:-/opt/preipo-market-platform}"

if [[ "${EUID}" -ne 0 ]]; then
  exec sudo -E bash "$0" "$@"
fi

systemctl stop "$SERVICE_NAME" 2>/dev/null || true
systemctl disable "$SERVICE_NAME" 2>/dev/null || true
rm -f "/etc/systemd/system/$SERVICE_NAME.service"
systemctl daemon-reload
rm -rf "$APP_DIR"

echo "$SERVICE_NAME has been uninstalled."
echo "Database files are kept under /var/lib/preipo-market-platform."
