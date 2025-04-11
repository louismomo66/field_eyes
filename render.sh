#!/bin/bash

# Script to run the Field Eyes API on Render or similar cloud platforms

echo "Starting Field Eyes API on Render..."

# Set cloud environment flag
export RENDER=true
export CLOUD_ENV=true

# Enable development mode to continue without database if needed
export DEV_MODE=true

# PORT is provided by Render
if [ -z "$PORT" ]; then
  export PORT=10000
  echo "PORT not set, using default: $PORT"
else
  echo "Using provided PORT: $PORT"
fi

# Run the application
echo "Starting Field Eyes API..."
./app/field_eyes_api 