# Field Eyes API Makefile

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the API binary"
	@echo "  make run           - Run the API locally"
	@echo "  make docker-build  - Build the Docker image"
	@echo "  make up            - Start all Docker containers"
	@echo "  make down          - Stop all Docker containers"
	@echo "  make docker-logs   - View Docker container logs"
	@echo "  make clean         - Clean up build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make migrate       - Run database migrations"

# Build the API binary to the correct location expected by Docker
.PHONY: build
build:
	@echo "Building API binary..."
	mkdir -p ./app
	go build -o ./app/field_eyes_api ./cmd/api
	@echo "Binary built at ./app/field_eyes_api"

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
.PHONY: up
up: build docker-build
	@echo "Starting Docker containers..."
	docker-compose up -d

# Stop all Docker containers
.PHONY: down
down:
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
	rm -rf ./app
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
dev: build docker-build up
	@echo "Development environment is up and running!"

# All-in-one command to stop development environment
.PHONY: dev-stop
dev-stop: down
	@echo "Development environment stopped!"