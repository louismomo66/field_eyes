FROM golang:1.24.1 AS builder

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o field_eyes_api ./cmd/api

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
  CMD curl -f http://localhost:9004/health || exit 1

# Expose the application port
EXPOSE 9004

# Environment variables documentation
# DATABASE_URL: postgres://username:password@hostname:port/database?sslmode=disable
# REDIS_HOST: Redis hostname (e.g., localhost or redis service name)
# REDIS_PORT: Redis port (default: 6379)
# PORT: Application port (default: 9004)
# JWT_SECRET: Secret for JWT token generation

# Command to run the executable
CMD ["./field_eyes_api"] 