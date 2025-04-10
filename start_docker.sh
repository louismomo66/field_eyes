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
docker rm -f fieldeyes-db fieldeyes-redis fieldeyes-api &>/dev/null || true

# Build and start the services with docker-compose
echo -e "${BLUE}Building and starting services with Docker Compose...${NC}"
docker-compose build
docker-compose up -d

# Check if the API is running
echo -e "${BLUE}Waiting for API to start...${NC}"
attempt=1
max_attempts=30
while [ $attempt -le $max_attempts ]; do
    if curl -s http://localhost:9004/health &>/dev/null; then
        echo -e "${GREEN}API is running successfully!${NC}"
        echo -e "${GREEN}API available at: http://localhost:9004${NC}"
        echo -e "${GREEN}Health check endpoint: http://localhost:9004/health${NC}"
        break
    fi
    echo -e "${YELLOW}Waiting for API to start (attempt $attempt/$max_attempts)...${NC}"
    sleep 2
    attempt=$((attempt + 1))
done

if [ $attempt -gt $max_attempts ]; then
    echo -e "${RED}API failed to start within the expected time.${NC}"
    echo -e "${YELLOW}Checking container logs...${NC}"
    docker logs fieldeyes-api
else
    # Display health check response
    echo -e "${BLUE}Health Check Response:${NC}"
    curl -s http://localhost:9004/health | json_pp 2>/dev/null || curl -s http://localhost:9004/health
fi

# Show how to check logs
echo -e "\n${BLUE}To check logs:${NC}"
echo -e "  ${GREEN}docker logs fieldeyes-api${NC}    # View API logs"
echo -e "  ${GREEN}docker logs fieldeyes-db${NC}    # View PostgreSQL logs"
echo -e "  ${GREEN}docker logs fieldeyes-redis${NC}    # View Redis logs"

# Show how to stop the stack
echo -e "\n${BLUE}To stop the stack:${NC}"
echo -e "  ${GREEN}docker-compose down${NC}" 