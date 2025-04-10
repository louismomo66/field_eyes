#!/bin/bash

# Set color variables
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PORT=9004

echo -e "${BLUE}Checking for processes using port ${PORT}...${NC}"

# For macOS
if [[ "$(uname)" == "Darwin" ]]; then
    # Find processes using the port
    PIDS=$(lsof -i :${PORT} -t)
    
    if [ -z "$PIDS" ]; then
        echo -e "${YELLOW}No processes found using port ${PORT}${NC}"
    else
        echo -e "${GREEN}Found processes using port ${PORT}:${NC}"
        lsof -i :${PORT}
        
        echo -e "${YELLOW}Killing processes...${NC}"
        for PID in $PIDS; do
            echo -e "Killing process ${PID}..."
            kill -9 $PID
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}Successfully killed process ${PID}${NC}"
            else
                echo -e "${RED}Failed to kill process ${PID}${NC}"
            fi
        done
        
        # Verify port is now free
        sleep 1
        if [ -z "$(lsof -i :${PORT} -t)" ]; then
            echo -e "${GREEN}Port ${PORT} is now free${NC}"
        else
            echo -e "${RED}Port ${PORT} is still in use${NC}"
            lsof -i :${PORT}
        fi
    fi
else
    # For Linux
    PIDS=$(netstat -tulpn 2>/dev/null | grep ":${PORT}" | awk '{print $7}' | cut -d'/' -f1)
    
    if [ -z "$PIDS" ]; then
        echo -e "${YELLOW}No processes found using port ${PORT}${NC}"
    else
        echo -e "${GREEN}Found processes using port ${PORT}:${NC}"
        netstat -tulpn | grep ":${PORT}"
        
        echo -e "${YELLOW}Killing processes...${NC}"
        for PID in $PIDS; do
            echo -e "Killing process ${PID}..."
            kill -9 $PID
            if [ $? -eq 0 ]; then
                echo -e "${GREEN}Successfully killed process ${PID}${NC}"
            else
                echo -e "${RED}Failed to kill process ${PID}${NC}"
            fi
        done
        
        # Verify port is now free
        sleep 1
        if [ -z "$(netstat -tulpn 2>/dev/null | grep ":${PORT}")" ]; then
            echo -e "${GREEN}Port ${PORT} is now free${NC}"
        else
            echo -e "${RED}Port ${PORT} is still in use${NC}"
            netstat -tulpn | grep ":${PORT}"
        fi
    fi
fi

echo -e "${BLUE}Process completed${NC}" 