#!/bin/bash

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=postgres
export DB_PASSWORD=postgres123456
export DB_NAME=field_eyes
export DSN="host=localhost port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable"
export REDIS_HOST=localhost
export REDIS_PORT=6379
export JWT_SECRET=fieldeystuliSmartbalimi

# Set TLS certificate paths - change these to your actual paths
export TLS_CERT_FILE=/root/fieldeyes/field_eyes/certs/server.crt
export TLS_KEY_FILE=/root/fieldeyes/field_eyes/certs/server.key

# Create certificates directory if it doesn't exist
mkdir -p /root/fieldeyes/field_eyes/certs

# Generate self-signed certificates if they don't exist
if [ ! -f "$TLS_CERT_FILE" ] || [ ! -f "$TLS_KEY_FILE" ]; then
    echo "Generating self-signed certificates..."
    openssl req -x509 -newkey rsa:4096 -keyout "$TLS_KEY_FILE" -out "$TLS_CERT_FILE" -days 365 -nodes -subj "/CN=localhost" -addext "subjectAltName=DNS:localhost,IP:127.0.0.1"
    echo "Certificates generated at $TLS_CERT_FILE and $TLS_KEY_FILE"
fi

# Run the application in the background
echo "Starting Field Eyes API with HTTP and HTTPS..."
nohup /root/fieldeyes/field_eyes/app/field_eyes_api > field_eyes.log 2>&1 &

# Store the PID
PID=$!
echo "Field Eyes API started with PID $PID"
echo "HTTP available at: http://localhost:9004"
echo "HTTPS available at: https://localhost:9443"
echo "Logs available at: field_eyes.log"

# Save PID to a file for easy stopping later
echo $PID > field_eyes.pid 