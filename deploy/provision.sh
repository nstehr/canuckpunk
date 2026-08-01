#!/usr/bin/env bash
# Prepares a fresh host to run canuckpunk: service account, state directory,
# and the packages the deploy targets rely on. Idempotent — safe to re-run.
#
# Not run directly. `make provision` pipes it to the host over SSH.
set -euo pipefail

SERVICE_USER=canuckpunk
STATE_DIR=/var/lib/canuckpunk
PORT="${CANUCKPUNK_PORT:-1867}"

if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
	echo "==> creating $SERVICE_USER system user"
	useradd --system --home-dir "$STATE_DIR" --shell /usr/sbin/nologin "$SERVICE_USER"
else
	echo "==> $SERVICE_USER already exists"
fi

echo "==> ensuring $STATE_DIR"
# systemd's StateDirectory= would create this too, but the narratives deploy
# rsyncs into it before the unit has ever started.
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$STATE_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0750 "$STATE_DIR/narratives"

if ! command -v rsync >/dev/null 2>&1; then
	echo "==> installing rsync"
	DEBIAN_FRONTEND=noninteractive apt-get update -qq
	DEBIAN_FRONTEND=noninteractive apt-get install -y -qq rsync
fi

# Only touch the firewall when one is actually running; a droplet with ufw
# inactive is left alone rather than half-configured.
if command -v ufw >/dev/null 2>&1 && ufw status | grep -q "^Status: active"; then
	echo "==> opening port $PORT in ufw"
	ufw allow "$PORT/tcp"
fi

echo "==> provision complete"
