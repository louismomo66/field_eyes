# Field Eyes API Makefile

# Default target
.PHONY: help
help:
	@echo "Available commands:"
	@echo "  make build             - Build the API binary"
	@echo "  make run               - Run the API locally"
	@echo "  make run-local         - Run the API locally with local environment"
	@echo "  make run-cloud         - Run for cloud deployments (Render, etc.) without DB requirements"
	@echo "  make docker-build      - Build the Docker image"
	@echo "  make docker-run        - Build and run the API in Docker with all services"
	@echo "  make docker-run-api    - Run only the API container (assumes services are running)"
	@echo "  make docker-services   - Start only PostgreSQL and Redis services"
	@echo "  make docker-test       - Run tests with the test_deployment.sh script"
	@echo "  make up                - Start all Docker containers"
	@echo "  make down              - Stop all Docker containers"
	@echo "  make docker-logs       - View Docker container logs"
	@echo "  make clean             - Clean up build artifacts"
	@echo "  make test              - Run tests"
	@echo "  make migrate           - Run database migrations"
	@echo "  make status            - Show status of Docker containers"
	@echo "  make deploy            - Build and deploy to cloud platform"

# Check if Docker is running
docker-check:
	@if ! docker info > /dev/null 2>&1; then \
		echo "Error: Docker is not running. Please start Docker Desktop first."; \
		exit 1; \
	fi

# Free port 9004 if it's in use
free-port:
	@if lsof -i :9004 -t > /dev/null 2>&1; then \
		echo "Port 9004 is in use. Killing processes..."; \
		for pid in $$(lsof -i :9004 -t); do \
			echo "Killing process $$pid"; \
			kill -9 $$pid; \
		done; \
		echo "Port 9004 is now free"; \
	else \
		echo "Port 9004 is available"; \
	fi

# Build the API binary with proper optimization
.PHONY: build
build:
	@echo "Building optimized API binary..."
	@mkdir -p ./app
	@go build -ldflags="-s -w" -o ./app/field_eyes_api ./cmd/api
	@echo "Binary built at ./app/field_eyes_api"

# Run the API locally
.PHONY: run
run: free-port
	@echo "Running API locally..."
	@go run ./cmd/api

# Run the API locally with local environment settings
.PHONY: run-local
run-local: free-port
	@echo "Running API locally with local environment..."
	@cp -f .env .env.backup 2>/dev/null || true
	@cp -f .env.local .env 2>/dev/null || echo "Warning: .env.local not found, using existing .env"
	@DEV_MODE=true go run ./cmd/api
	@mv -f .env.backup .env 2>/dev/null || true

# Run the API in cloud environment (like Render) with graceful fallbacks
.PHONY: run-cloud
run-cloud:
	@echo "Running API in cloud deployment mode..."
	@echo "This mode assumes external database connections may not be available"
	@DEV_MODE=true PORT=$(PORT) go run ./cmd/api

# Build for Render cloud deployment
.PHONY: build-render
build-render:
	@echo "Building for Render deployment..."
	@mkdir -p ./app
	@go build -ldflags="-s -w" -o ./app/field_eyes_api ./cmd/api
	@echo "Binary built at ./app/field_eyes_api"

# Run on Render cloud platform
.PHONY: start-render
start-render: build-render
	@echo "Starting API on Render..."
	@echo "Using PORT: $(PORT)"
	@DEV_MODE=true ./app/field_eyes_api

# Build Docker image for development
.PHONY: docker-build
docker-build: docker-check
	@echo "Building Docker image..."
	@docker-compose build

# Run a comprehensive test using the test script
.PHONY: docker-test
docker-test: docker-check
	@echo "Running deployment test..."
	@chmod +x ./test_deployment.sh
	@./test_deployment.sh

# Start only database services (PostgreSQL and Redis)
.PHONY: docker-services
docker-services: docker-check
	@echo "Starting database services (PostgreSQL and Redis)..."
	@docker-compose up -d postgres redis
	@echo "Waiting for PostgreSQL to initialize..."
	@for i in $$(seq 1 10); do \
		if docker exec fieldeyes-postgres pg_isready -U postgres > /dev/null 2>&1; then \
			echo "PostgreSQL is ready!"; \
			break; \
		fi; \
		echo "Waiting for PostgreSQL to start (attempt $$i/10)..."; \
		sleep 2; \
	done
	@echo "Services are running! PostgreSQL at localhost:5432, Redis at localhost:6379"

# Run only the API container, connecting to existing services
.PHONY: docker-run-api
docker-run-api: docker-check docker-build
	@echo "Starting API container connecting to existing services..."
	@docker-compose up -d api
	@echo "API is running at http://localhost:9004"
	@echo "Check health at http://localhost:9004/health"

# Build and run everything in Docker
.PHONY: docker-run
docker-run: docker-check down
	@echo "Starting full application stack with Docker..."
	@docker-compose up -d
	@echo "Waiting for API to start..."
	@for i in $$(seq 1 15); do \
		if curl -s http://localhost:9004/health > /dev/null 2>&1; then \
			echo "API is running successfully!"; \
			echo "API available at: http://localhost:9004"; \
			echo "Health check endpoint: http://localhost:9004/health"; \
			break; \
		fi; \
		echo "Waiting for API to start (attempt $$i/15)..."; \
		sleep 2; \
	done

# Start all Docker containers for development
.PHONY: up
up: docker-check docker-build
	@echo "Starting Docker services..."
	@docker-compose up -d
	@echo "Docker services started!"

# Stop all Docker containers
.PHONY: down
down: docker-check
	@echo "Stopping Docker services..."
	@docker-compose down
	@echo "Docker services stopped!"

# View Docker container logs
.PHONY: docker-logs
docker-logs: docker-check
	@echo "Viewing Docker logs..."
	@docker-compose logs -f

# Clean up build artifacts and Docker resources
.PHONY: clean
clean:
	@echo "Cleaning up build artifacts..."
	@rm -rf ./bin ./app
	@go clean
	@echo "Cleaning up Docker resources..."
	@docker-compose down --volumes --remove-orphans 2>/dev/null || true
	@docker network prune -f 2>/dev/null || true

# Run tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v ./...

# Database migrations
.PHONY: migrate
migrate:
	@echo "Running database migrations..."
	@go run ./cmd/api/migrate.go

# Deploy to cloud platform
.PHONY: deploy
deploy: docker-build-cloud
	@echo "Deploying to cloud platform..."
	@echo "Make sure you are logged in to the cloud platform's CLI"
	@echo "Set REGISTRY_URL environment variable to your registry URL"
	@docker-compose --profile cloud push api-cloud

# Build Docker image for cloud deployment
.PHONY: docker-build-cloud
docker-build-cloud: docker-check
	@echo "Building Docker image for cloud deployment..."
	@docker-compose --profile cloud build api-cloud

# Show Docker status
.PHONY: status
status: docker-check
	@echo "Docker containers:"
	@docker ps -a --filter "name=fieldeyes*"
	@echo "\nDocker networks:"
	@docker network ls --filter "name=fieldeyes*"
	@echo "\nDocker volumes:"
	@docker volume ls --filter "name=field_eyes*"
	@echo "\nAPI health status:"
	@curl -s http://localhost:9004/health 2>/dev/null | grep status || echo "API not running"