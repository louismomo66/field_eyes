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

# Start the database containers using docker-compose
echo -e "${BLUE}Starting database containers with docker-compose...${NC}"
docker-compose down -v &>/dev/null || true
docker-compose up -d postgres redis

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

# Starting the application
echo -e "${BLUE}Starting Field Eyes API...${NC}"
go run ./cmd/api 