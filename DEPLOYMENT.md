# Deploying Field Eyes API to Back4App

This guide provides step-by-step instructions on how to deploy the Field Eyes API to Back4App (or similar container platforms).

## Prerequisites

1. A Back4App account
2. The Back4App CLI installed and configured
3. Docker installed on your local machine

## Deployment Steps

### 1. Build and Push the Docker Image

First, set your registry URL as an environment variable:

```bash
export REGISTRY_URL=your-registry-url
```

Then build and push the deployment Docker image:

```bash
make deploy
```

Alternatively, you can push the image manually:

```bash
# Build the cloud image
docker-compose --profile cloud build api-cloud

# Push the image to your registry
docker-compose --profile cloud push api-cloud
```

### 2. Configure the Back4App Container Service

1. Log in to your Back4App dashboard
2. Navigate to "Containers" and create a new container
3. Choose the "Bring your own image" option
4. Enter your Docker image URL
5. Configure the following settings:
   - Port: 9004
   - Environment Variables:
     - DB_HOST: (your database host)
     - DB_PORT: (your database port)
     - DB_USER: (your database username)
     - DB_PASSWORD: (your database password)
     - DB_NAME: (your database name)
     - DSN: host=(your database host) port=(your database port) user=(your database username) password=(your database password) dbname=(your database name) sslmode=disable
     - JWT_SECRET: (your JWT secret)
   - Health Check Path: /health

### 3. Setting Up a Database

You have two options for the database:

#### Option A: Using Back4App's Database Service
1. Create a new PostgreSQL database in Back4App
2. Configure the connection settings in your container's environment variables

#### Option B: Using External Database
1. Use any PostgreSQL provider (AWS RDS, DigitalOcean, etc.)
2. Make sure the database is accessible from Back4App
3. Configure the connection settings in your container's environment variables

### 4. Verify Deployment

1. Wait for the container to show "Running" status in the Back4App dashboard
2. Click on the URL provided by Back4App to access your API
3. Test the API endpoints using Postman or any API testing tool

### Troubleshooting

If your container fails to start or becomes unhealthy:

1. Check the container logs in the Back4App dashboard
2. Verify that your database connection settings are correct
3. Make sure the health check endpoint is working
4. Ensure that port 9004 is exposed in your Dockerfile and correctly mapped in the container settings
5. If you see "no such host" errors, ensure your environment variables are properly set
6. If the database connection fails, check if the database is accessible from the container

## Additional Configuration

### Custom Domain

To set up a custom domain for your API:

1. Go to your container's settings in Back4App
2. Navigate to the "Networking" tab
3. Add your custom domain
4. Configure DNS settings as instructed by Back4App

### Scaling

To scale your API for higher traffic:

1. Go to your container's settings in Back4App
2. Navigate to the "Scaling" tab
3. Adjust the number of instances or resource allocation as needed 