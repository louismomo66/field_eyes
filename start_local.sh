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

# Create temporary env file with DB_PORT set to 5432 (standard port)
echo -e "${BLUE}Creating temporary environment variables...${NC}"
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres123456
export DB_NAME=field_eyes
export JWT_SECRET=fieldeystuliSmartbalimi

# Check if psql command is available
if command -v psql &>/dev/null; then
    echo -e "${BLUE}Checking if PostgreSQL is running...${NC}"
    if psql -h localhost -p 5432 -U postgres -c "SELECT 1" &>/dev/null; then
        echo -e "${GREEN}PostgreSQL is running.${NC}"
    else
        echo -e "${YELLOW}PostgreSQL is not running or not accessible. Starting local run anyway...${NC}"
    fi
else
    echo -e "${YELLOW}psql command not found, cannot check PostgreSQL status.${NC}"
fi

echo -e "${BLUE}Starting Field Eyes API in local development mode...${NC}"
echo -e "${YELLOW}Note: This script ignores Redis and MQTT dependencies${NC}"
echo -e "${YELLOW}You may see warnings about Redis and MQTT connection failures${NC}"

# Run the application with modified settings
echo -e "${GREEN}Running application...${NC}"
go run ./cmd/api 