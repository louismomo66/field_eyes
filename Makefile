# Field Eyes API Makefile

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the API binary"
	@echo "  make run           - Run the API locally"
	@echo "  make docker-build  - Build the Docker image"
	@echo "  make docker-up     - Start all Docker containers"
	@echo "  make docker-down   - Stop all Docker containers"
	@echo "  make docker-logs   - View Docker container logs"
	@echo "  make clean         - Clean up build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make migrate       - Run database migrations"

# Build the API binary
.PHONY: build
build:
	@echo "Building API binary..."
	go build -o ./bin/field_eyes_api ./cmd/api

# Run the API locally
.PHONY: run
run:
	@echo "Running API locally..."
	go run ./cmd/api

# Build Docker image
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker-compose build

# Start all Docker containers
.PHONY: docker-up
docker-up:
	@echo "Starting Docker containers..."
	docker-compose up -d

# Stop all Docker containers
.PHONY: docker-down
docker-down:
	@echo "Stopping Docker containers..."
	docker-compose down

# View Docker container logs
.PHONY: docker-logs
docker-logs:
	@echo "Viewing Docker logs..."
	docker-compose logs -f

# Clean up build artifacts
.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	rm -rf ./bin
	go clean

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	go test -v ./...

# Database migrations
.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	go run ./cmd/api/migrate.go

# All-in-one command to start development environment
.PHONY: dev
dev: docker-build docker-up
	@echo "Development environment is up and running!"

# All-in-one command to stop development environment
.PHONY: dev-stop
dev-stop: docker-down
	@echo "Development environment stopped!" 