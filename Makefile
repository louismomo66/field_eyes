# Field Eyes API Makefile

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build         - Build the API binary"
	@echo "  make run           - Run the API locally"
	@echo "  make run-local     - Run the API locally with local environment"
	@echo "  make docker-build  - Build the Docker image"
	@echo "  make up            - Start all Docker containers"
	@echo "  make down          - Stop all Docker containers"
	@echo "  make docker-logs   - View Docker container logs"
	@echo "  make clean         - Clean up build artifacts"
	@echo "  make test          - Run tests"
	@echo "  make migrate       - Run database migrations"
	@echo "  make deploy        - Build and deploy to cloud platform"

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

# Run the API locally with local environment settings
.PHONY: run-local
run-local:
	@echo "Running API locally with local environment..."
	cp .env.local .env
	go run ./cmd/api

# Build Docker image for development
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker-compose --profile dev build

# Build Docker image for cloud deployment
.PHONY: docker-build-cloud
docker-build-cloud:
	@echo "Building Docker image for cloud deployment..."
	docker-compose --profile cloud build api-cloud

# Start all Docker containers for development
.PHONY: up
up: build docker-build
	@echo "Starting Docker containers..."
	docker-compose --profile dev up -d

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

# Deploy to cloud platform
.PHONY: deploy
deploy: docker-build-cloud
	@echo "Deploying to cloud platform..."
	@echo "Make sure you are logged in to the cloud platform's CLI"
	@echo "Set REGISTRY_URL environment variable to your registry URL"
	docker-compose --profile cloud push api-cloud

# All-in-one command to start development environment
.PHONY: dev
dev: build docker-build up
	@echo "Development environment is up and running!"

# All-in-one command to stop development environment
.PHONY: dev-stop
dev-stop: down
	@echo "Development environment stopped!"