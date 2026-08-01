#!/usr/bin/env bash
# Installs whatever `make deploy` staged in /tmp/canuckpunk-deploy: the two
# binaries and the systemd unit. Stops the service, swaps, migrates, starts.
#
# Not run directly. `make deploy` pipes it to the host over SSH.
set -euo pipefail

STAGE=/tmp/canuckpunk-deploy
SERVICE_USER=canuckpunk
STATE_DIR=/var/lib/canuckpunk
BIN_DIR=/usr/local/bin
DB_PATH="$STATE_DIR/canuckpunk.db"

for f in canuckpunk canuckpunk-migrate canuckpunk.service; do
	if [ ! -f "$STAGE/$f" ]; then
		echo "missing staged file: $STAGE/$f" >&2
		exit 1
	fi
done

echo "==> installing unit"
install -m 0644 "$STAGE/canuckpunk.service" /etc/systemd/system/canuckpunk.service
systemctl daemon-reload

# Only stop when it is actually running, so a first deploy onto a fresh host
# does not fail on a unit that has never started.
if systemctl is-active --quiet canuckpunk; then
	echo "==> stopping canuckpunk"
	systemctl stop canuckpunk
fi

# rename rather than copy: a copy onto a running binary fails with ETXTBSY,
# and a rename swaps the inode atomically.
echo "==> installing binaries"
for b in canuckpunk canuckpunk-migrate; do
	install -m 0755 "$STAGE/$b" "$BIN_DIR/.$b.new"
	mv -f "$BIN_DIR/.$b.new" "$BIN_DIR/$b"
done

echo "==> migrating $DB_PATH"
# As the service user, so the database file it creates is one the service can
# still write once systemd drops privileges.
runuser -u "$SERVICE_USER" -- "$BIN_DIR/canuckpunk-migrate" -db "$DB_PATH" up

echo "==> starting canuckpunk"
systemctl enable --now canuckpunk

rm -rf "$STAGE"

systemctl --no-pager --lines=0 status canuckpunk || true
echo "==> deploy complete"
