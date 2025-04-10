#!/bin/bash

# Set color variables
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[0;33m'
NC='\033[0m' # No Color

echo -e "${YELLOW}Building Docker image for Back4App deployment...${NC}"
docker build -t fieldeyes:test -f Dockerfile.back4app .

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to build Docker image${NC}"
    exit 1
fi

echo -e "${YELLOW}Running container for testing...${NC}"
echo -e "${YELLOW}Container will be removed after testing${NC}"

# Start the container in detached mode with port 9004 mapped to localhost:9004
CONTAINER_ID=$(docker run -d -p 9004:9004 fieldeyes:test)

if [ $? -ne 0 ]; then
    echo -e "${RED}Failed to start container${NC}"
    exit 1
fi

echo -e "${YELLOW}Container started with ID: ${CONTAINER_ID}${NC}"
echo -e "${YELLOW}Waiting 5 seconds for the application to start...${NC}"
sleep 5

echo -e "${YELLOW}Testing health endpoint...${NC}"
HEALTH_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9004/health)

if [ "$HEALTH_RESPONSE" == "200" ]; then
    echo -e "${GREEN}Health endpoint responded with status 200 OK${NC}"
    echo -e "${YELLOW}Getting health endpoint content:${NC}"
    curl -s http://localhost:9004/health | json_pp || echo "Failed to format JSON response"
else
    echo -e "${RED}Health endpoint failed with status: $HEALTH_RESPONSE${NC}"
    echo -e "${YELLOW}Checking container logs:${NC}"
    docker logs $CONTAINER_ID
fi

# Check if the container is listening on port 9004
echo -e "${YELLOW}Checking if the container is listening on port 9004...${NC}"
docker exec $CONTAINER_ID netstat -tulpn 2>/dev/null | grep 9004 || echo -e "${RED}Container is not listening on port 9004${NC}"

echo -e "${YELLOW}Cleaning up - removing container...${NC}"
docker rm -f $CONTAINER_ID

echo -e "${GREEN}Test completed${NC}" 