.PHONY: build run build-migrate generate generate-sqlc new-migration migrate migrate-down migrate-status db-reset clean test lint fmt

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
