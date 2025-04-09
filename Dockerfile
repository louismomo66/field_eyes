# Use an official Go runtime as a base
FROM golang:1.22.2 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy the local package files to the container's workspace
COPY . .

# Install godotenv to manage .env files
RUN go get github.com/joho/godotenv

# Download all dependencies
RUN go mod tidy

# Build the application to run in a scratch container
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o field_eyes_api ./cmd/api

# Use a lightweight Alpine image
FROM alpine:latest

# Install ca-certificates in case your application makes external HTTPS calls
RUN apk --no-cache add ca-certificates

# Set the working directory in the container
WORKDIR /app

# Copy the pre-built binary file from the previous stage
COPY --from=builder /app/field_eyes_api .

# Create empty .env file
RUN touch .env

# Expose port
EXPOSE 9004

# Command to run the executable
CMD ["./field_eyes_api"] 