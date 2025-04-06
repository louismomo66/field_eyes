# Field Eyes API

Field Eyes is an API for managing IoT devices and user accounts.

## Docker Setup

The application is containerized using Docker and includes:
- API service (Go)
- PostgreSQL database
- Redis cache

### Quick Start

1. Make sure Docker and Docker Compose are installed on your system.
2. Set up your environment variables (see `.env.example`).
3. Start the application:

```bash
# Using Make (recommended)
make dev

# Using Docker Compose directly
docker-compose up -d
```

## Makefile Commands

This project includes a Makefile to simplify common tasks:

```
make help         - Show available commands
make build        - Build the API binary
make run          - Run the API locally
make docker-build - Build the Docker image
make docker-up    - Start all Docker containers
make docker-down  - Stop all Docker containers
make docker-logs  - View Docker container logs
make clean        - Clean up build artifacts
make test         - Run tests
make migrate      - Run database migrations
make dev          - Start development environment
make dev-stop     - Stop development environment
```

## Account Management Features

### User Signup
- Endpoint: `POST /api/signup`
- Request Body:
  ```json
  {
    "username": "username",
    "email": "user@example.com",
    "password": "password123"
  }
  ```

### User Login
- Endpoint: `POST /api/login`
- Request Body:
  ```json
  {
    "email": "user@example.com",
    "password": "password123"
  }
  ```
- Response: JSON Web Token (JWT)

### Password Recovery
The API supports account recovery through a two-step process:

1. **Request Password Reset**
   - Endpoint: `POST /api/forgot-password`
   - Request Body:
     ```json
     {
       "email": "user@example.com"
     }
     ```
   - An OTP will be sent to the registered email address

2. **Reset Password with OTP**
   - Endpoint: `POST /api/reset-password`
   - Request Body:
     ```json
     {
       "email": "user@example.com",
       "otp": "123456",
       "new_password": "newpassword123"
     }
     ```

## Device Management Features

### Device Registration Workflow
The system supports an automatic device registration flow:

1. **Auto-Registration**: When a device sends data for the first time, it's automatically registered in the system with no user assigned.
2. **User Assignment**: Users can claim auto-registered devices by using their serial number during device registration.
3. **Data Association**: All data logged by a device is associated with that device, even before it's assigned to a user.

This workflow allows devices to start logging data immediately upon deployment, and users can claim them later.

### Register/Claim Device
- Endpoint: `POST /api/register-device`
- Requires authentication 
- Request Body:
  ```json
  {
    "device_type": "soil_sensor", // Optional for auto-registered devices
    "serial_number": "SN12345678"
  }
  ```
- This endpoint can either:
  - Register a new device (if serial number is new)
  - Claim an existing auto-registered device (if device exists but has no user)
  - Return success if device is already registered to the user

### Log Device Data
- Endpoint: `POST /api/log-device-data`
- Request Body:
  ```json
  {
    "serial_number": "SN12345678",
    "temperature": 25.5,
    "humidity": 60.2,
    "nitrogen": 45.0,
    "phosphorous": 30.5,
    "potassium": 20.3,
    "ph": 6.8,
    "soil_moisture": 35.2,
    "soil_temperature": 22.1,
    "soil_humidity": 55.4,
    "longitude": 37.7749,
    "latitude": -122.4194
  }
  ```
- If the device doesn't exist, it will be auto-registered with device type "auto_registered"
- Data is associated with the device regardless of whether it has a user assigned

### MQTT Device Data Logging
Devices can also send data via MQTT, which follows the same auto-registration and data logging workflow as the HTTP endpoint.

#### Regular (Single Message) Format
- **Topic Format**: `field_eyes/devices/SERIAL_NUMBER/data`
- Replace `SERIAL_NUMBER` with your device's actual serial number
- **Message Format**: Same JSON format as the HTTP endpoint:
  ```json
  {
    "serial_number": "SN12345678",
    "temperature": 25.5,
    "humidity": 60.2,
    "nitrogen": 45.0,
    "phosphorous": 30.5,
    "potassium": 20.3,
    "ph": 6.8,
    "soil_moisture": 35.2,
    "soil_temperature": 22.1,
    "soil_humidity": 55.4,
    "longitude": 37.7749,
    "latitude": -122.4194
  }
  ```

#### Chunked Message Format (for Large Payloads)
For devices with limited buffer sizes or when dealing with larger payloads, the system supports sending data in chunks:

- **Topic Format**: `field_eyes/devices/SERIAL_NUMBER/chunked/TOTAL_CHUNKS/CHUNK_NUMBER`
  - `SERIAL_NUMBER`: Your device's serial number
  - `TOTAL_CHUNKS`: Total number of chunks in the message (e.g., 3)
  - `CHUNK_NUMBER`: The index of this chunk (starting from 1)
- **Example**: 
  - First chunk: `field_eyes/devices/SN12345678/chunked/3/1`
  - Second chunk: `field_eyes/devices/SN12345678/chunked/3/2`
  - Third chunk: `field_eyes/devices/SN12345678/chunked/3/3`
- Each message chunk contains a portion of the JSON payload
- The server will reassemble the chunks and process them once all chunks are received
- Incomplete sets of chunks are automatically cleaned up after 1 hour

- **Auto-registration**: Just like the HTTP endpoint, if the device doesn't exist, it will be auto-registered
- **QoS Level**: The server uses QoS level 1 (at least once delivery)

### Get Device Logs
- Endpoint: `GET /api/get-device-logs?serial_number=SN12345678`
- Requires authentication
- Only returns logs for devices registered to the authenticated user
- Response: Array of device log entries

## Data Analysis Features

### ML-Based Device Data Analysis
- Endpoint: `GET /api/analyze-device?serial_number=SN12345678&type=soil`
- Requires authentication
- Analysis Types:
  - `soil`: Analysis of soil health metrics
  - `temperature`: Analysis of temperature patterns
  - `moisture`: Analysis of moisture levels
  - `nutrient`: Analysis of soil nutrient levels (N, P, K)
- Response:
  ```json
  {
    "device_id": 1,
    "serial_number": "SN12345678",
    "analysis_type": "soil",
    "recommendations": [
      "Based on soil pH and moisture levels, consider adjusting irrigation schedule",
      "Soil is too acidic, consider adding lime"
    ],
    "predictions": {
      "optimal_ph": 6.5
    },
    "trends": {
      "soil_moisture": "decreasing"
    },
    "last_updated": "2023-05-01T12:34:56Z"
  }
  ```

## Caching Implementation

Field Eyes uses Redis for caching to improve performance and support ML analysis operations:

1. **Device Logs Caching**: Device log data is cached when first retrieved and invalidated when new data is logged.
2. **ML Analysis Results Caching**: Analysis results are cached to avoid redundant processing and improve response times.
3. **User Device Caching**: A user's devices are cached for faster access.

## Environment Configuration

The API requires the following environment variables:

```
# JWT Configuration
JWT_SECRET=your_jwt_secret_key

# SMTP Configuration (for password reset emails)
SMTP_HOST=smtp.example.com
SMTP_PORT=587
SMTP_USERNAME=your_email@example.com
SMTP_PASSWORD=your_password
SMTP_FROM=noreply@fieldeyes.com

# Redis Configuration (for caching)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password
REDIS_DB=0

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=field_eyes

# MQTT Configuration
MQTT_BROKER_URL=tcp://localhost:1883
MQTT_USERNAME=your_mqtt_username  # Optional
MQTT_PASSWORD=your_mqtt_password  # Optional
MQTT_CLIENT_ID=field_eyes_server
MQTT_TOPIC_ROOT=field_eyes/devices
```
