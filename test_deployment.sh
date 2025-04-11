#!/bin/bash

# Set color variables for better output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${BLUE}Starting Field Eyes API deployment test...${NC}"

# Check if Docker is running
echo -e "${BLUE}Checking if Docker is running...${NC}"
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Docker is not running. Please start Docker Desktop first.${NC}"
    exit 1
fi

# Create a Docker network for the test
echo -e "${YELLOW}Creating test network...${NC}"
docker network create fieldeyes-test-network || true

# Clean up any existing test containers
echo -e "${YELLOW}Cleaning up any existing test containers...${NC}"
docker rm -f fieldeyes-postgres-test fieldeyes-redis-test fieldeyes-api-test > /dev/null 2>&1

# Start PostgreSQL container
echo -e "${YELLOW}Starting PostgreSQL container...${NC}"
docker run --name fieldeyes-postgres-test \
  --network fieldeyes-test-network \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres123456 \
  -e POSTGRES_DB=field_eyes \
  -d postgres:15 > /dev/null

# Start Redis container
echo -e "${YELLOW}Starting Redis container...${NC}"
docker run --name fieldeyes-redis-test \
  --network fieldeyes-test-network \
  -d redis:7 > /dev/null

# Wait for PostgreSQL to initialize
echo -e "${YELLOW}Waiting for PostgreSQL to initialize (10 seconds)...${NC}"
sleep 10

# Build the application image
echo -e "${YELLOW}Building Docker image for the application...${NC}"
docker build -t fieldeyes:test .

# Check if the build was successful
if [ $? -ne 0 ]; then
  echo -e "${RED}Docker build failed!${NC}"
  # Clean up
  docker rm -f fieldeyes-postgres-test fieldeyes-redis-test > /dev/null 2>&1
  docker network rm fieldeyes-test-network > /dev/null 2>&1
  exit 1
fi

# Run the application container
echo -e "${YELLOW}Starting application container...${NC}"
CONTAINER_ID=$(docker run -d \
  --name fieldeyes-api-test \
  --network fieldeyes-test-network \
  -p 9004:9004 \
  -e PORT=9004 \
  -e DB_HOST=fieldeyes-postgres-test \
  -e DB_PORT=5432 \
  -e DB_USER=postgres \
  -e DB_PASSWORD=postgres123456 \
  -e DB_NAME=field_eyes \
  -e DATABASE_URL="postgres://postgres:postgres123456@fieldeyes-postgres-test:5432/field_eyes?sslmode=disable" \
  -e REDIS_HOST=fieldeyes-redis-test \
  -e REDIS_PORT=6379 \
  -e JWT_SECRET=fieldeystuliSmartbalimi \
  fieldeyes:test)

# Wait for app to start
echo -e "${YELLOW}Waiting for application to start (10 seconds)...${NC}"
sleep 10

# Check application logs
echo -e "${YELLOW}Application logs:${NC}"
docker logs fieldeyes-api-test | tail -20

# Test health endpoint
echo -e "${YELLOW}Testing health endpoint...${NC}"
HEALTH_RESPONSE=$(curl -s -o health_response.json -w "%{http_code}" http://localhost:9004/health)

if [ "$HEALTH_RESPONSE" == "200" ]; then
  echo -e "${GREEN}Health check passed!${NC}"
  echo -e "${YELLOW}Health endpoint content:${NC}"
  cat health_response.json | json_pp || cat health_response.json
  rm health_response.json
else
  echo -e "${RED}Health check failed with status code: $HEALTH_RESPONSE${NC}"
  echo -e "${YELLOW}Container logs:${NC}"
  docker logs fieldeyes-api-test | tail -50
  echo -e "${YELLOW}Checking if container is listening on port 9004:${NC}"
  docker exec fieldeyes-api-test netstat -tulpn || echo "netstat not available"
fi

# Clean up
echo -e "${YELLOW}Cleaning up containers and network...${NC}"
docker rm -f fieldeyes-api-test fieldeyes-postgres-test fieldeyes-redis-test > /dev/null 2>&1
docker network rm fieldeyes-test-network > /dev/null 2>&1

echo -e "${GREEN}Test complete!${NC}" 