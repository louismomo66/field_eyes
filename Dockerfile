FROM golang:1.22-alpine AS builder

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

# Copy the binary from the app directory
COPY ./app/field_eyes_api /app/field_eyes_api

# Create a default .env file if it doesn't exist
RUN touch .env

# Expose the application port
EXPOSE 9004

# Run the application
CMD ["/app/field_eyes_api"] 