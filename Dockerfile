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
RUN apk --no-cache add ca-certificates tzdata bash postgresql-client

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/app/field_eyes_api /app/field_eyes_api

# Create wait-for-it script
RUN echo '#!/bin/bash' > /app/wait-for-it.sh && \
    echo 'set -e' >> /app/wait-for-it.sh && \
    echo '' >> /app/wait-for-it.sh && \
    echo 'host="$1"' >> /app/wait-for-it.sh && \
    echo 'shift' >> /app/wait-for-it.sh && \
    echo 'cmd="$@"' >> /app/wait-for-it.sh && \
    echo '' >> /app/wait-for-it.sh && \
    echo 'until PGPASSWORD=$POSTGRES_PASSWORD psql -h "$host" -U "$POSTGRES_USER" -c '\''\q'\''; do' >> /app/wait-for-it.sh && \
    echo '  >&2 echo "Postgres is unavailable - sleeping"' >> /app/wait-for-it.sh && \
    echo '  sleep 1' >> /app/wait-for-it.sh && \
    echo 'done' >> /app/wait-for-it.sh && \
    echo '' >> /app/wait-for-it.sh && \
    echo '>&2 echo "Postgres is up - executing command"' >> /app/wait-for-it.sh && \
    echo 'exec $cmd' >> /app/wait-for-it.sh && \
    chmod +x /app/wait-for-it.sh

# Create empty .env file
RUN touch .env

# Set default environment variables
ENV DB_HOST=postgres
ENV DB_PORT=5432
ENV DB_USER=postgres
ENV DB_PASSWORD=postgres
ENV DB_NAME=field_eyes

# Expose port
EXPOSE 8080

# Run the application with wait-for-it script
CMD ["/bin/bash", "/app/wait-for-it.sh", "postgres", "/app/field_eyes_api"] 