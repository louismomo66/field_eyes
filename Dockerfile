FROM golang:1.20-alpine AS builder

# Set working directory
WORKDIR /app

# Install necessary dependencies
RUN apk add --no-cache gcc musl-dev

# Copy go.mod and go.sum
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -o field_eyes_api ./cmd/api

# Create a smaller image for the final container
FROM alpine:latest

# Add necessary certificates for HTTPS
RUN apk --no-cache add ca-certificates tzdata

# Set working directory
WORKDIR /app

# Copy the binary from the builder stage
COPY --from=builder /app/field_eyes_api .
COPY --from=builder /app/.env .

# Expose the application port
EXPOSE 9004

# Run the application
CMD ["./field_eyes_api"] 