#!/bin/bash

# Set color variables
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Setting up test environment...${NC}"

# Check if port 9004 is already in use and kill processes if needed
PORT=9004
echo -e "${BLUE}Checking if port ${PORT} is already in use...${NC}"
if [[ "$(uname)" == "Darwin" ]]; then
    PIDS=$(lsof -i :${PORT} -t)
    if [ ! -z "$PIDS" ]; then
        echo -e "${YELLOW}Port ${PORT} is already in use. Killing processes...${NC}"
        for PID in $PIDS; do
            echo -e "Killing process ${PID}..."
            kill -9 $PID
        done
        echo -e "${GREEN}Port ${PORT} is now free${NC}"
    else
        echo -e "${GREEN}Port ${PORT} is available${NC}"
    fi
fi

# Clean up any existing containers with the same names
echo -e "${BLUE}Cleaning up any existing containers...${NC}"
docker rm -f fieldeyes-postgres fieldeyes-redis fieldeyes-api 2>/dev/null || true

# Create docker network
echo -e "${BLUE}Creating docker network...${NC}"
docker network create fieldeyes-test-network 2>/dev/null || true

# Start PostgreSQL container
echo -e "${BLUE}Starting PostgreSQL container...${NC}"
docker run --name fieldeyes-postgres \
  --network fieldeyes-test-network \
  -e POSTGRES_PASSWORD=postgres123456 \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_DB=field_eyes \
  -d postgres:14-alpine

# Start Redis container
echo -e "${BLUE}Starting Redis container...${NC}"
docker run --name fieldeyes-redis \
  --network fieldeyes-test-network \
  -d redis:alpine

echo -e "${BLUE}Waiting for PostgreSQL to initialize (10 seconds)...${NC}"
sleep 10

# Building Docker image for Back4App deployment
echo -e "${BLUE}Building Docker image for Back4App deployment...${NC}"
docker build -t fieldeyes:back4app -f Dockerfile.back4app .

if [ $? -ne 0 ]; then
  echo -e "${RED}Failed to build Docker image${NC}"
  echo -e "${YELLOW}Cleaning up...${NC}"
  docker rm -f fieldeyes-postgres fieldeyes-redis 2>/dev/null || true
  docker network rm fieldeyes-test-network 2>/dev/null || true
  exit 1
fi

# Run container for testing
echo -e "${BLUE}Running container for testing...${NC}"
CONTAINER_ID=$(docker run -d \
  --name fieldeyes-api \
  --network fieldeyes-test-network \
  -p 9004:9004 \
  -e DB_HOST=fieldeyes-postgres \
  -e REDIS_HOST=fieldeyes-redis \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=postgres123456 \
  -e DB_NAME=field_eyes \
  -e DSN="host=fieldeyes-postgres port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable" \
  fieldeyes:back4app)

echo -e "${BLUE}Container started with ID: ${CONTAINER_ID}${NC}"
echo -e "${BLUE}Following logs for 20 seconds...${NC}"
docker logs -f $CONTAINER_ID &
LOGS_PID=$!

echo -e "${BLUE}Waiting for application to start (20 seconds)...${NC}"
sleep 20
kill $LOGS_PID 2>/dev/null

# Test health endpoint
echo -e "${BLUE}Testing health endpoint...${NC}"
RESPONSE=$(curl -v http://localhost:9004/health 2>&1)
HTTP_CODE=$(echo "$RESPONSE" | grep -o "< HTTP/[0-9.]* [0-9]*" | awk '{print $3}')

if [[ "$HTTP_CODE" == "200" ]]; then
  echo -e "${GREEN}Health endpoint is working! HTTP Status: $HTTP_CODE${NC}"
  # Format the JSON response
  curl -s http://localhost:9004/health | json_pp || echo "json_pp not available, showing raw output:" && curl -s http://localhost:9004/health
else
  echo -e "${RED}Health endpoint failed! HTTP Status: $HTTP_CODE${NC}"
  echo -e "${RED}Response:${NC}"
  echo "$RESPONSE"
  
  # Check if the container is listening on port 9004
  echo -e "${YELLOW}Checking if container is listening on port 9004...${NC}"
  docker exec $CONTAINER_ID netstat -tulpn | grep 9004 || echo -e "${RED}Container is not listening on port 9004${NC}"
  
  # Show the most recent logs
  echo -e "${YELLOW}Recent logs from container:${NC}"
  docker logs --tail 50 $CONTAINER_ID
fi

# Clean up
echo -e "${YELLOW}Cleaning up...${NC}"
docker rm -f $CONTAINER_ID fieldeyes-postgres fieldeyes-redis 2>/dev/null
docker network rm fieldeyes-test-network 2>/dev/null

echo -e "${BLUE}Test completed${NC}" 