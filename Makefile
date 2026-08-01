.PHONY: build run build-migrate generate generate-sqlc new-migration migrate migrate-down migrate-status db-reset clean test lint fmt \
	build-linux provision deploy deploy-narratives deploy-restart deploy-logs deploy-status require-droplet

# Include local env vars if present
-include .env.local
export

CANUCKPUNK_DB ?= canuckpunk.db

# Build the main application
build:
	go build -o bin/canuckpunk ./cmd/canuckpunk

# Run the main application
run:
	go run ./cmd/canuckpunk

# Build the migration binary
build-migrate:
	go build -o bin/canuckpunk-migrate ./cmd/canuckpunk-migrate

# Generate sqlc code
generate-sqlc:
	rm -f internal/db/*.sql.go
	sqlc generate

# Generate all code
generate: generate-sqlc

# Create a new migration (usage: make new-migration name=add_rooms_table)
new-migration:
	goose -dir data/migrations create $(name) sql

# Run migrations
migrate:
	go run ./cmd/canuckpunk-migrate -db $(CANUCKPUNK_DB) up

# Rollback last migration
migrate-down:
	go run ./cmd/canuckpunk-migrate -db $(CANUCKPUNK_DB) down

# Show migration status
migrate-status:
	go run ./cmd/canuckpunk-migrate -db $(CANUCKPUNK_DB) status

# Reset database (delete and re-migrate)
db-reset:
	rm -f $(CANUCKPUNK_DB)
	$(MAKE) migrate
	@echo "Database reset."

# Clean build artifacts
clean:
	rm -rf bin/

# Run all tests
test:
	go test ./...

# Run linters
lint:
	golangci-lint run ./...

# Format code
fmt:
	golangci-lint fmt ./...

# ----------------------------------------------------------------------------
# Deployment
#
# Host and credentials come from .env.local (see .env.example); nothing about
# a particular server is committed.
# ----------------------------------------------------------------------------

DROPLET_USER ?= root
DROPLET_SSH_KEY ?= ~/.ssh/id_droplet
DROPLET_SSH_PORT ?= 22

DEPLOY_GOOS ?= linux
DEPLOY_GOARCH ?= amd64
DIST_DIR := bin/$(DEPLOY_GOOS)_$(DEPLOY_GOARCH)
STAGE_DIR := /tmp/canuckpunk-deploy
REMOTE_NARRATIVES ?= /var/lib/canuckpunk/narratives

# Make does no tilde expansion, and the key path is likely to be written with
# one, so expand it here rather than depending on the shell's word splitting.
DROPLET_SSH_KEY := $(patsubst ~/%,$(HOME)/%,$(DROPLET_SSH_KEY))


# scp spells the port -P where ssh spells it -p, so the two differ.
KEY_OPT = -i "$(DROPLET_SSH_KEY)"
SSH_OPTS = $(KEY_OPT) -p $(DROPLET_SSH_PORT)
SCP_OPTS = $(KEY_OPT) -P $(DROPLET_SSH_PORT)
REMOTE = $(DROPLET_USER)@$(DROPLET_HOST)
SSH = ssh $(SSH_OPTS) $(REMOTE)

require-droplet:
	@if [ -z "$(DROPLET_HOST)" ]; then \
		echo "DROPLET_HOST is not set. Copy .env.example to .env.local and fill it in." >&2; \
		exit 1; \
	fi
	@if [ ! -f "$(DROPLET_SSH_KEY)" ]; then \
		echo "SSH key not found: $(DROPLET_SSH_KEY)" >&2; \
		exit 1; \
	fi

# Build the Linux binaries the droplet runs. Pure-Go SQLite, so no cross
# compiler is needed.
build-linux:
	CGO_ENABLED=0 GOOS=$(DEPLOY_GOOS) GOARCH=$(DEPLOY_GOARCH) \
		go build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/canuckpunk ./cmd/canuckpunk
	CGO_ENABLED=0 GOOS=$(DEPLOY_GOOS) GOARCH=$(DEPLOY_GOARCH) \
		go build -trimpath -ldflags="-s -w" -o $(DIST_DIR)/canuckpunk-migrate ./cmd/canuckpunk-migrate

# One-time host preparation: service account, state directory, packages.
# Safe to re-run.
provision: require-droplet
	$(SSH) 'bash -s' < deploy/provision.sh
	$(MAKE) deploy-narratives

# Build, ship, migrate, restart.
deploy: require-droplet build-linux
	$(SSH) 'rm -rf $(STAGE_DIR) && mkdir -p $(STAGE_DIR)'
	scp $(SCP_OPTS) \
		$(DIST_DIR)/canuckpunk $(DIST_DIR)/canuckpunk-migrate deploy/canuckpunk.service \
		$(REMOTE):$(STAGE_DIR)/
	$(SSH) 'bash -s' < deploy/install.sh

# Ship prose only. The server reads narratives from disk per request, so this
# needs no restart — edits are live on the next screen a player reaches.
deploy-narratives: require-droplet
	$(SSH) 'install -d -o canuckpunk -g canuckpunk -m 0750 $(REMOTE_NARRATIVES)'
	rsync -avz --delete --exclude '*.go' \
		-e 'ssh $(SSH_OPTS)' \
		narratives/ $(REMOTE):$(REMOTE_NARRATIVES)/
	$(SSH) 'chown -R canuckpunk:canuckpunk $(REMOTE_NARRATIVES)'

deploy-restart: require-droplet
	$(SSH) 'systemctl restart canuckpunk && systemctl --no-pager --lines=0 status canuckpunk'

deploy-status: require-droplet
	$(SSH) 'systemctl --no-pager status canuckpunk'

deploy-logs: require-droplet
	$(SSH) 'journalctl -u canuckpunk -n 100 -f'
