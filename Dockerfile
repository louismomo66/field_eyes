FROM golang:1.24.1 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application with optimization
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o field_eyes_api ./cmd/api

# Create a minimal production image
FROM alpine:latest

# Install CA certificates for HTTPS and other tools
RUN apk --no-cache add ca-certificates tzdata curl netcat-openbsd

# Set the working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/field_eyes_api .

# Create necessary directories
RUN mkdir -p /app/templates

# Add a simple healthcheck
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:${PORT:-9004}/health || exit 1

# Expose the application port (will be overridden by PORT env var if set)
EXPOSE ${PORT:-9004}

# Default environment variables
ENV DEV_MODE=false
ENV PORT=9004
ENV DOCKER_ENV=true

# Special environment for Render and other cloud platforms
ENV CLOUD_ENV=false

# Environment variables documentation
# DATABASE_URL: postgres://username:password@hostname:port/database?sslmode=disable
# REDIS_HOST: Redis hostname (e.g., localhost or redis service name)
# REDIS_PORT: Redis port (default: 6379)
# PORT: Application port (default: 9004)
# JWT_SECRET: Secret for JWT token generation

# Command to run the executable
CMD ["./field_eyes_api"] 