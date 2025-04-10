FROM golang:1.24.1

WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy the local package files to the container's workspace
COPY . .

# Install required packages
RUN apt-get update && apt-get install -y wget ca-certificates

# Build the application
RUN go build -o field_eyes_api ./cmd/api

# Expose port
EXPOSE 9004

# Set default environment variables using DATABASE_URL
ENV DATABASE_URL=postgres://postgres:postgres123456@db:5432/field_eyes?sslmode=disable
ENV JWT_SECRET=fieldeystuliSmartbalimi

# Command to run the executable
CMD ["./field_eyes_api"]

# Remove Git lock file
RUN rm -f .git/index.lock 