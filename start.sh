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
docker-compose down &>/dev/null || true
docker rm -f fieldeyes-postgres fieldeyes-redis &>/dev/null || true

# Start the database containers using docker-compose
echo -e "${BLUE}Starting database containers with docker-compose...${NC}"
docker-compose up -d postgres redis

# Get the PostgreSQL container IP address
echo -e "${BLUE}Getting PostgreSQL container IP address...${NC}"
sleep 3  # Give the container a moment to get its network ready
POSTGRES_IP=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' fieldeyes-postgres)
if [ -z "$POSTGRES_IP" ]; then
    echo -e "${RED}Could not get PostgreSQL container IP address.${NC}"
    POSTGRES_IP="fieldeyes-postgres"  # Fallback to container name
else
    echo -e "${GREEN}PostgreSQL container IP: ${POSTGRES_IP}${NC}"
fi

# Wait for PostgreSQL to be ready
echo -e "${BLUE}Waiting for PostgreSQL to initialize...${NC}"
attempt=1
max_attempts=30
while [ $attempt -le $max_attempts ]; do
    if docker exec fieldeyes-postgres pg_isready -U postgres &>/dev/null; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    echo -e "${YELLOW}Waiting for PostgreSQL to start (attempt $attempt/$max_attempts)...${NC}"
    sleep 2
    attempt=$((attempt + 1))
done

if [ $attempt -gt $max_attempts ]; then
    echo -e "${RED}PostgreSQL failed to start within the expected time. Please check Docker logs.${NC}"
    exit 1
fi

# Verify PostgreSQL connection
echo -e "${BLUE}Verifying PostgreSQL connection...${NC}"
if docker exec fieldeyes-postgres psql -U postgres -d field_eyes -c "SELECT 1" &>/dev/null; then
    echo -e "${GREEN}Successfully connected to PostgreSQL database!${NC}"
else
    echo -e "${RED}Failed to connect to PostgreSQL database.${NC}"
    docker logs fieldeyes-postgres
    exit 1
fi

# Update the DSN to use the container name instead of localhost
# This is important for Docker networking
echo -e "${BLUE}Setting up environment variables with Docker-specific settings...${NC}"
export DB_HOST="fieldeyes-postgres"  # Use container name for Docker networking
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres123456
export DB_NAME=field_eyes
export DSN="host=fieldeyes-postgres port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable"
export REDIS_HOST="fieldeyes-redis"  # Use container name for Redis too

# Starting the application
echo -e "${BLUE}Starting Field Eyes API with explicit Docker networking...${NC}"
echo -e "${GREEN}Using DSN: ${DSN}${NC}"
go run ./cmd/api 