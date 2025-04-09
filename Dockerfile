# Build stage
FROM golang:1.22-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev

# Set working directory
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application with explicit architecture settings
ENV GOOS=linux
ENV GOARCH=amd64
RUN CGO_ENABLED=0 go build -o app/field_eyes_api ./cmd/api

# Final stage
FROM alpine:latest

# Install necessary packages
RUN apk --no-cache add ca-certificates tzdata bash

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/app/field_eyes_api /app/field_eyes_api

# Create a startup script
RUN echo '#!/bin/bash' > /app/start.sh && \
    echo 'set -e' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo 'echo "Starting Field Eyes API..."' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Print environment variables (without sensitive data)' >> /app/start.sh && \
    echo 'echo "Environment variables:"' >> /app/start.sh && \
    echo 'echo "DB_HOST: $DB_HOST"' >> /app/start.sh && \
    echo 'echo "DB_PORT: $DB_PORT"' >> /app/start.sh && \
    echo 'echo "DB_USER: $DB_USER"' >> /app/start.sh && \
    echo 'echo "DB_NAME: $DB_NAME"' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Check if DB_HOST is set to localhost and override if needed' >> /app/start.sh && \
    echo 'if [ "$DB_HOST" = "localhost" ]; then' >> /app/start.sh && \
    echo '  echo "WARNING: DB_HOST is set to localhost, which may not work in a containerized environment"' >> /app/start.sh && \
    echo '  echo "If you are deploying to Render, make sure to set DB_HOST to your PostgreSQL service hostname"' >> /app/start.sh && \
    echo 'fi' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Create a .env file with the environment variables' >> /app/start.sh && \
    echo 'cat > /app/.env << EOF' >> /app/start.sh && \
    echo 'DB_HOST=$DB_HOST' >> /app/start.sh && \
    echo 'DB_PORT=$DB_PORT' >> /app/start.sh && \
    echo 'DB_USER=$DB_USER' >> /app/start.sh && \
    echo 'DB_PASSWORD=$DB_PASSWORD' >> /app/start.sh && \
    echo 'DB_NAME=$DB_NAME' >> /app/start.sh && \
    echo 'REDIS_HOST=$REDIS_HOST' >> /app/start.sh && \
    echo 'REDIS_PORT=$REDIS_PORT' >> /app/start.sh && \
    echo 'MQTT_HOST=$MQTT_HOST' >> /app/start.sh && \
    echo 'MQTT_PORT=$MQTT_PORT' >> /app/start.sh && \
    echo 'EOF' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo 'echo ".env file created with environment variables"' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Start the application' >> /app/start.sh && \
    echo 'exec /app/field_eyes_api' >> /app/start.sh && \
    chmod +x /app/start.sh

# Create empty .env file
RUN touch .env

# Set default environment variables
# These will be overridden by Render's environment variables
ENV DB_HOST=localhost
ENV DB_PORT=5432
ENV DB_USER=postgres
ENV DB_PASSWORD=postgres
ENV DB_NAME=field_eyes
ENV REDIS_HOST=localhost
ENV REDIS_PORT=6379
ENV MQTT_HOST=localhost
ENV MQTT_PORT=1883

# Expose port
EXPOSE 8080

# Run the application with the startup script
CMD ["/bin/bash", "/app/start.sh"] 