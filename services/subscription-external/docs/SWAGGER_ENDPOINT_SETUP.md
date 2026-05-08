# Swagger Endpoint Setup for Subscription External Service

## Overview
This document describes the complete Swagger endpoint setup for the subscription-external service, including all available endpoints and how to access the Swagger UI.

## What Was Implemented

### 1. **Swagger Documentation Generation**
- ✅ Regenerated Swagger documentation using `swag init -g cmd/main.go`
- ✅ Updated all endpoint annotations with proper Swagger comments
- ✅ Added missing endpoints (health, metrics, user base upload)
- ✅ Fixed existing annotations and response models

### 2. **New Endpoints Added**

#### Health Check Endpoint
- **URL**: `GET /health`
- **Description**: Check if the service is healthy and running
- **Response**: `200 OK` with "OK" message
- **Swagger Tag**: System

#### Metrics Endpoint
- **URL**: `GET /metrics`
- **Description**: Get Prometheus metrics for the service
- **Response**: `200 OK` with Prometheus metrics in text format
- **Swagger Tag**: System

#### User Base Upload Endpoint
- **URL**: `POST /api/v1/userbase/upload`
- **Description**: Upload and process CSV or XLSX files containing user base data
- **Content-Type**: `multipart/form-data`
- **Parameters**: `file` (CSV or XLSX file)
- **Swagger Tag**: UserBase

### 3. **Existing Endpoints Enhanced**

#### Subscription Endpoints
- **POST /api/v1/subscription-external**: Opt-in a single subscription
- **POST /api/v1/subscription-external/batch**: Batch opt-in subscriptions
- **Swagger Tag**: Subscriptions

### 4. **Router Updates**
- ✅ Added health and metrics endpoints to the router
- ✅ Fixed function calls and imports
- ✅ Proper error handling for all endpoints

## How to Access Swagger UI

### 1. **Start the Service**
```bash
cd services/subscription-external
go build -o subscription-external cmd/main.go
./subscription-external
```

### 2. **Access Swagger UI**
Open your browser and navigate to:
```
http://localhost:8083/swagger/index.html
```

### 3. **Test Endpoints**
You can test the endpoints directly from the Swagger UI or using curl:

```bash
# Health check
curl http://localhost:8083/health

# Metrics
curl http://localhost:8083/metrics

# Swagger documentation
curl http://localhost:8083/swagger/index.html
```

## Available Endpoints

### System Endpoints
| Method | Endpoint | Description | Tags |
|--------|----------|-------------|------|
| GET | `/health` | Health check endpoint | System |
| GET | `/metrics` | Metrics endpoint | System |
| GET | `/swagger/*` | Swagger UI and documentation | - |

### API Endpoints
| Method | Endpoint | Description | Tags |
|--------|----------|-------------|------|
| POST | `/api/v1/subscription-external` | Opt-in a single subscription | Subscriptions |
| POST | `/api/v1/subscription-external/batch` | Batch opt-in subscriptions | Subscriptions |
| POST | `/api/v1/userbase/upload` | Upload user base file | UserBase |

## Swagger Documentation Structure

### Generated Files
- `docs/docs.go`: Main Swagger documentation file
- `docs/swagger.json`: JSON format of the API specification
- `docs/swagger.yaml`: YAML format of the API specification

### Key Features
- ✅ Complete API documentation with all endpoints
- ✅ Request/response models for all endpoints
- ✅ Proper error responses documented
- ✅ Content-Type specifications
- ✅ Parameter validation rules
- ✅ Tag-based organization

## Regenerating Documentation

To regenerate the Swagger documentation after making changes:

```bash
cd services/subscription-external
swag init -g cmd/main.go
```

This will update all the documentation files in the `docs/` directory.

## Configuration

The Swagger configuration is defined in `cmd/main.go`:

```go
// @title Subscription Management API
// @version 1.0
// @description This is the API documentation for the Subscription Management Service.
// @termsOfService https://omni-connect.com/terms/
// @contact.name API Support
// @contact.url https://omni-connect.com/support
// @contact.email support@omni-connect.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8083
// @BasePath /
```

## Troubleshooting

### Common Issues

1. **Service not starting**: Check if port 8083 is available
2. **Swagger UI not loading**: Ensure the service is running and accessible
3. **Missing endpoints**: Regenerate documentation with `swag init -g cmd/main.go`
4. **Build errors**: Check for missing imports or syntax errors

### Verification Steps

1. Build the service: `go build -o subscription-external cmd/main.go`
2. Start the service: `./subscription-external`
3. Test health endpoint: `curl http://localhost:8083/health`
4. Access Swagger UI: `http://localhost:8083/swagger/index.html`

## Dependencies

The Swagger implementation uses:
- `github.com/swaggo/fasthttp-swagger`: FastHTTP Swagger middleware
- `github.com/swaggo/swag`: Swagger documentation generator
- `github.com/swaggo/files`: Swagger UI files

## Conclusion

The subscription-external service now has a complete Swagger endpoint setup with:
- ✅ All endpoints properly documented
- ✅ Interactive Swagger UI accessible at `/swagger/index.html`
- ✅ Health and metrics endpoints for monitoring
- ✅ Proper error handling and response documentation
- ✅ Easy-to-use API testing interface

The Swagger UI provides a comprehensive interface for testing and understanding all available endpoints in the service. 