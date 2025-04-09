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
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/app/field_eyes_api /app/field_eyes_api

# Create empty .env file
RUN touch .env

# Expose port
EXPOSE 8080

# Run the application
CMD ["./field_eyes_api"] 