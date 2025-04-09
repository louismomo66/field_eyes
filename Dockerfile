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
RUN apk --no-cache add ca-certificates tzdata bash curl

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/app/field_eyes_api /app/field_eyes_api

# Create startup script
RUN echo '#!/bin/bash' > /app/start.sh && \
    echo 'set -e' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo 'echo "Starting Field Eyes API..."' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Check if DB_HOST is set' >> /app/start.sh && \
    echo 'if [ -z "$DB_HOST" ]; then' >> /app/start.sh && \
    echo '  echo "ERROR: DB_HOST environment variable is not set"' >> /app/start.sh && \
    echo '  exit 1' >> /app/start.sh && \
    echo 'fi' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Check if DB_USER is set' >> /app/start.sh && \
    echo 'if [ -z "$DB_USER" ]; then' >> /app/start.sh && \
    echo '  echo "ERROR: DB_USER environment variable is not set"' >> /app/start.sh && \
    echo '  exit 1' >> /app/start.sh && \
    echo 'fi' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Check if DB_PASSWORD is set' >> /app/start.sh && \
    echo 'if [ -z "$DB_PASSWORD" ]; then' >> /app/start.sh && \
    echo '  echo "ERROR: DB_PASSWORD environment variable is not set"' >> /app/start.sh && \
    echo '  exit 1' >> /app/start.sh && \
    echo 'fi' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Check if DB_NAME is set' >> /app/start.sh && \
    echo 'if [ -z "$DB_NAME" ]; then' >> /app/start.sh && \
    echo '  echo "ERROR: DB_NAME environment variable is not set"' >> /app/start.sh && \
    echo '  exit 1' >> /app/start.sh && \
    echo 'fi' >> /app/start.sh && \
    echo '' >> /app/start.sh && \
    echo '# Print connection info (without sensitive data)' >> /app/start.sh && \
    echo 'echo "Connecting to database at $DB_HOST:$DB_PORT"' >> /app/start.sh && \
    echo 'echo "Using database: $DB_NAME"' >> /app/start.sh && \
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