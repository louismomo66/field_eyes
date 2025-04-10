# Use an official Go runtime as a base
FROM --platform=linux/arm64 golang:1.22.2 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the local package files to the container's workspace
COPY . .

# Install godotenv to manage .env files
RUN go get github.com/joho/godotenv

# Install wget for health checks
RUN apt-get update && apt-get install -y wget

# Build the application to run in a scratch container
RUN CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o field_eyes_api ./cmd/api

# Use a lightweight Alpine image
FROM --platform=linux/arm64 alpine:latest

# Install ca-certificates and wget for health checks
RUN apk --no-cache add ca-certificates wget

# Set the working directory in the container
WORKDIR /app

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/field_eyes_api .

# Create empty .env file
RUN touch .env

# Set default environment variables
ENV DB_HOST=localhost
ENV DB_PORT=5432
ENV DB_USER=postgres
ENV DB_PASSWORD=postgres123456
ENV DB_NAME=field_eyes
ENV DSN="host=localhost port=5432 user=postgres password=postgres123456 dbname=field_eyes sslmode=disable"

# Expose port
EXPOSE 9004

# Command to run the executable
CMD ["./field_eyes_api"] 