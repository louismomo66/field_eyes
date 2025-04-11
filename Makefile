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
	@go run ./cmd/api

# Run the API locally with local environment settings
.PHONY: run-local
run-local:
	@echo "Running API locally with .env.local..."
	@cp .env.local .env
	@DEV_MODE=true go run ./cmd/api

# Build Docker image for development
.PHONY: docker-build
docker-build:
	@echo "Building Docker image..."
	docker-compose build

# Build Docker image for cloud deployment
.PHONY: docker-build-cloud
docker-build-cloud:
	@echo "Building Docker image for cloud deployment..."
	docker-compose --profile cloud build api-cloud

# Start all Docker containers for development
.PHONY: up
up: docker-build docker-up
	@echo "Docker services started!"

# Stop all Docker containers
.PHONY: down
down: docker-down
	@echo "Docker services stopped!"

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

# Kill processes on port 9004
kill:
	@echo "Killing processes on port 9004..."
	@./kill_processes.sh

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

# Docker compose commands
docker-up:
	@echo "Starting Docker services..."
	docker-compose up -d

docker-down:
	@echo "Stopping Docker services..."
	docker-compose down

# New targets
.PHONY: build up down clean logs debug

# Build the Docker images
build:
	@echo "Building Docker image..."
	docker-compose build

# Start the Docker services
up: build
	@echo "Starting Docker services..."
	docker-compose up -d

# Stop the Docker services
down:
	@echo "Stopping Docker services..."
	docker-compose down

# Stop and remove all Docker resources
clean:
	@echo "Cleaning up all Docker resources..."
	docker-compose down --volumes --remove-orphans
	docker network prune -f

# Show logs from the app container
logs:
	docker-compose logs -f app

# Run the debug container
debug:
	docker-compose run --rm debug

# Run the database setup
db-init:
	docker-compose exec db psql -U postgres -c "CREATE DATABASE field_eyes WITH OWNER postgres ENCODING 'UTF8';"

# Show Docker status
status:
	@echo "Docker containers:"
	@docker ps -a --filter "name=fieldeyes*"
	@echo "\nDocker networks:"
	@docker network ls --filter "name=field_eyes*"
	@echo "\nDocker volumes:"
	@docker volume ls --filter "name=field_eyes*"