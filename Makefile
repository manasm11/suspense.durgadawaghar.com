.PHONY: all build run clean generate templ sqlc deps test

# Default target
all: generate build

# Install dependencies
deps:
	go mod download
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

# Generate templ files
templ:
	templ generate

# Generate sqlc files (if you need to regenerate)
sqlc:
	sqlc generate

# Generate all code
generate: templ

# Build the application
build: generate
	go build -o bin/server ./cmd/server

# Run the application
run: generate
	go run ./cmd/server

# Run with specific port
run-port:
	go run ./cmd/server -port=$(PORT)

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f suspense.db
	find . -name "*_templ.go" -delete

# Run tests
test:
	go test ./...

# Format code
fmt:
	go fmt ./...
	templ fmt .

# Development mode with auto-reload (requires air)
dev:
	air

# Initialize database only
init-db:
	sqlite3 suspense.db < internal/db/schema.sql

# Help
help:
	@echo "Available targets:"
	@echo "  all      - Generate code and build (default)"
	@echo "  deps     - Install Go dependencies and tools"
	@echo "  templ    - Generate templ files"
	@echo "  sqlc     - Regenerate sqlc files"
	@echo "  generate - Generate all code"
	@echo "  build    - Build the server binary"
	@echo "  run      - Run the server"
	@echo "  clean    - Remove build artifacts"
	@echo "  test     - Run tests"
	@echo "  fmt      - Format code"
	@echo "  dev      - Run with hot reload (requires air)"
	@echo "  init-db  - Initialize SQLite database"
