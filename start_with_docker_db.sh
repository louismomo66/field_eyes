#!/bin/bash

# Set color variables for better output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# First check if port 9004 is already in use
echo -e "${BLUE}Checking if port 9004 is in use...${NC}"
if lsof -i :9004 -t &>/dev/null; then
    echo -e "${YELLOW}Port 9004 is already in use. Killing processes...${NC}"
    for PID in $(lsof -i :9004 -t); do
        echo -e "Killing process ${PID}..."
        kill -9 $PID
    done
    echo -e "${GREEN}Port 9004 is now free${NC}"
else
    echo -e "${GREEN}Port 9004 is available${NC}"
fi

# Check if Docker is running
echo -e "${BLUE}Checking if Docker is running...${NC}"
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker Desktop first.${NC}"
    exit 1
fi

# Clean up any existing containers
echo -e "${BLUE}Cleaning up any existing containers...${NC}"
docker-compose down -v 2>/dev/null || true
docker rm -f fieldeyes-postgres fieldeyes-redis 2>/dev/null || true

# Start the database containers using docker-compose
echo -e "${BLUE}Starting PostgreSQL and Redis containers...${NC}"
docker-compose up -d postgres redis

# Wait for PostgreSQL to be ready
echo -e "${BLUE}Waiting for PostgreSQL to be ready...${NC}"
attempt=1
max_attempts=30
while [ $attempt -le $max_attempts ]; do
    if docker exec fieldeyes-postgres pg_isready -U postgres > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    echo -e "${YELLOW}Waiting for PostgreSQL to start (attempt $attempt/$max_attempts)...${NC}"
    sleep 2
    attempt=$((attempt + 1))
done

if [ $attempt -gt $max_attempts ]; then
    echo -e "${RED}PostgreSQL failed to start within the expected time.${NC}"
    echo -e "${YELLOW}Continuing anyway, but the application may fail to connect.${NC}"
else
    # Verify the PostgreSQL container is correctly setup
    echo -e "${BLUE}Verifying PostgreSQL container setup...${NC}"
    docker exec fieldeyes-postgres psql -U postgres -c "SELECT 1" || echo -e "${RED}Failed to run psql in container${NC}"
    
    # Get the container's IP address
    CONTAINER_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' fieldeyes-postgres)
    echo -e "${GREEN}PostgreSQL container IP: ${CONTAINER_IP}${NC}"
    
    # Set host.docker.internal for Docker Desktop on Mac
    echo -e "${BLUE}Setting up special Docker DNS mapping...${NC}"
    if [[ "$(uname)" == "Darwin" ]]; then
        echo -e "${GREEN}On macOS, you can use 'host.docker.internal' to access the host from a container${NC}"
        echo -e "${GREEN}But from the host to container, you need to use 'localhost' with the mapped port${NC}"
    fi
fi

# Checking network connectivity
echo -e "${BLUE}Checking network connectivity to PostgreSQL...${NC}"
if nc -z localhost 5432 >/dev/null 2>&1; then
    echo -e "${GREEN}Port 5432 is accessible on localhost${NC}"
else
    echo -e "${RED}Port 5432 is NOT accessible on localhost. This might be a Docker port mapping issue.${NC}"
fi

# Run the application with the appropriate environment variables
echo -e "${BLUE}Starting Field Eyes API in local development mode...${NC}"
echo -e "${GREEN}Running application...${NC}"

# Use the same environment variables as in .env, but ensure DB_HOST is set to localhost
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres123456
export DB_NAME=field_eyes
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=fieldeystuliSmartbalimi

# Run the application
echo -e "${YELLOW}Note: If connection fails with 'localhost', the app will try 'host.docker.internal' and 'fieldeyes-postgres'${NC}"
go run ./cmd/api 