#!/bin/bash

# Check if PID file exists
if [ -f "field_eyes.pid" ]; then
    PID=$(cat field_eyes.pid)
    echo "Stopping Field Eyes API (PID: $PID)..."
    kill $PID
    rm field_eyes.pid
    echo "Field Eyes API stopped."
else
    echo "PID file not found. Trying to find process..."
    PID=$(ps aux | grep field_eyes_api | grep -v grep | awk '{print $2}')
    
    if [ -z "$PID" ]; then
        echo "Field Eyes API is not running."
    else
        echo "Found Field Eyes API process (PID: $PID). Stopping..."
        kill $PID
        echo "Field Eyes API stopped."
    fi
fi 