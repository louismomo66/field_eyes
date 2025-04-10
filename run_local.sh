#!/bin/bash

# Set color variables for better output
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Check if port 9004 is already in use
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

# Try to use Docker if available
DOCKER_RUNNING=false
if command -v docker &> /dev/null && docker info &> /dev/null; then
    echo -e "${GREEN}Docker is running. Starting PostgreSQL container...${NC}"
    DOCKER_RUNNING=true
    
    # Clean up any existing containers
    echo -e "${BLUE}Cleaning up any existing containers...${NC}"
    docker rm -f fieldeyes-postgres fieldeyes-redis &>/dev/null || true
    
    # Start PostgreSQL container directly
    echo -e "${BLUE}Starting PostgreSQL container...${NC}"
    docker run --name fieldeyes-postgres -e POSTGRES_PASSWORD=postgres123456 \
        -e POSTGRES_USER=postgres -e POSTGRES_DB=field_eyes \
        -p 5432:5432 -d postgres:14-alpine
    
    # Wait for PostgreSQL to be ready
    echo -e "${BLUE}Waiting for PostgreSQL to be ready...${NC}"
    attempt=1
    max_attempts=15
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
        echo -e "${RED}PostgreSQL failed to start within the expected time.${NC}"
        echo -e "${YELLOW}Will run application in development mode without PostgreSQL...${NC}"
        DOCKER_RUNNING=false
    fi
else
    echo -e "${YELLOW}Docker is not running. Will run application in development mode without PostgreSQL...${NC}"
fi

# Decide how to run the application
if [ "$DOCKER_RUNNING" = true ]; then
    echo -e "${GREEN}Running application with PostgreSQL database...${NC}"
    
    # Use the correct environment variables
    export DB_HOST=localhost
    export DB_PORT=5432
    export DB_USER=postgres
    export DB_PASSWORD=postgres123456
    export DB_NAME=field_eyes
    export DSN="host=localhost port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable"
    
    # Run the application
    go run ./cmd/api
else
    echo -e "${YELLOW}Running application in DEVELOPMENT MODE without database...${NC}"
    echo -e "${RED}CAUTION: This is for UI development only. Database features will not work.${NC}"
    
    # Modify the initDB function to return a nil connection without crashing
    echo -e "${BLUE}Creating temporary override file...${NC}"
    cat > ./cmd/api/db_override.go << EOF
package main

import (
	"gorm.io/gorm"
	"log"
)

// Override initDB for development mode
func (app *Config) initDB() *gorm.DB {
	log.Println("⚠️  Running in DEVELOPMENT MODE without database ⚠️")
	log.Println("⚠️  Database features will not work ⚠️")
	return nil
}
EOF
    
    # Run with the override
    go run ./cmd/api
    
    # Clean up the override file
    rm ./cmd/api/db_override.go
fi 