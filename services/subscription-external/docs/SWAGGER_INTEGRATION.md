# Swagger Integration for Renewal System

## Overview

The renewal system has been fully integrated with Swagger/OpenAPI documentation generation. All renewal endpoints, request/response models, and data structures are now documented and accessible through the interactive Swagger UI.

## 🔄 Regenerating Documentation

### Automatic Regeneration

To regenerate the Swagger documentation after making changes:

```bash
cd services/subscription-external
./scripts/regenerate_swagger.sh
```

### Manual Regeneration

```bash
cd services/subscription-external/cmd
swag init -g main.go -d .,../internal,../internal/handler,../internal/service,../internal/transport,../internal/worker,../internal/domain -o ../docs --instanceName swagger
```

## 📚 Documentation Structure

### API Endpoints

The renewal system adds the following endpoint groups to Swagger:

#### 1. **Renewal Worker** (`/api/v1/renewal/worker/*`)
- **POST** `/api/v1/renewal/worker/start` - Start the renewal worker
- **POST** `/api/v1/renewal/worker/stop` - Stop the renewal worker  
- **GET** `/api/v1/renewal/worker/status` - Get worker status and metrics

#### 2. **Renewal Monitoring** (`/api/v1/renewal/*`)
- **GET** `/api/v1/renewal/statistics` - Get renewal statistics
- **GET** `/api/v1/renewal/churn-candidates` - Get churn candidates
- **GET** `/api/v1/renewal/health` - Get system health status
- **GET** `/api/v1/renewal/cycles` - Get renewal cycles (not implemented)
- **GET** `/api/v1/renewal/priority-retry` - Get priority retry queue (not implemented)

#### 3. **Renewal Operations** (`/api/v1/renewal/*`)
- **POST** `/api/v1/renewal/priority-retry/process` - Process priority retry queue
- **POST** `/api/v1/renewal/manual` - Trigger manual renewal
- **POST** `/api/v1/renewal/force-churn-evaluation` - Force churn evaluation

### Data Models

#### Request Models

- **`ManualRenewalRequest`** - Structure for manual renewal requests
  ```json
  {
    "msisdn": "1234567890",
    "product_id": "PROD_001", 
    "channel": "API"
  }
  ```

#### Response Models

- **`RenewalWorkerStatus`** - Worker status and metrics
- **`RenewalHealthResponse`** - System health information
- **`ChurnEvaluationResponse`** - Churn evaluation results
- **`RenewalMetrics`** - Performance metrics
- **`RenewalCycle`** - Renewal cycle tracking
- **`PriorityRetryQueue`** - Retry queue items

## 🏷️ Swagger Tags

The renewal endpoints are organized into logical groups using Swagger tags:

- **`Renewal Worker`** - Worker management operations
- **`Renewal Monitoring`** - Status and statistics endpoints
- **`Renewal Operations`** - Active renewal and churn operations

## 📖 Viewing Documentation

### 1. Start the Service

```bash
cd services/subscription-external
go run cmd/main.go
```

### 2. Access Swagger UI

Open your browser and navigate to:
```
http://localhost:8083/swagger/
```

### 3. Navigate to Renewal Endpoints

- Expand the **Renewal Worker** section for worker management
- Expand the **Renewal Monitoring** section for status and metrics
- Expand the **Renewal Operations** section for active operations

## 🔧 Swagger Annotations

### Handler Methods

Each handler method includes comprehensive Swagger annotations:

```go
// @Summary Start renewal worker
// @Description Starts the automated renewal worker that processes subscription renewals
// @Tags Renewal Worker
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Worker started successfully"
// @Failure 409 {string} string "Worker already running"
// @Failure 503 {string} string "Worker not available"
// @Failure 500 {string} string "Failed to start worker"
// @Router /api/v1/renewal/worker/start [post]
func (h *RenewalHandler) StartRenewalWorker(ctx *fasthttp.RequestCtx) {
```

### Domain Models

Key domain models include Swagger descriptions:

```go
// RenewalCycle tracks an opt-out/opt-in renewal attempt
// @Description Tracks each step of the opt-out/opt-in renewal process
type RenewalCycle struct {
    // ... fields
}
```

## 📊 Example API Calls

### Start Renewal Worker

```bash
curl -X POST "http://localhost:8083/api/v1/renewal/worker/start" \
  -H "Content-Type: application/json"
```

**Response:**
```json
{
  "status": "success",
  "message": "Renewal worker started successfully",
  "running": true
}
```

### Get Worker Status

```bash
curl "http://localhost:8083/api/v1/renewal/worker/status"
```

**Response:**
```json
{
  "running": true,
  "metrics": {
    "total_processed": 150,
    "successful_renewals": 142,
    "failed_renewals": 8,
    "success_rate": 0.947
  }
}
```

### Get Renewal Statistics

```bash
curl "http://localhost:8083/api/v1/renewal/statistics?days=7"
```

**Response:**
```json
{
  "total_processed": 1050,
  "successful_renewals": 998,
  "failed_renewals": 52,
  "churned_subscriptions": 15,
  "success_rate": 0.95,
  "average_cycle_time": 2.3,
  "last_run_time": "2024-01-15T02:00:00Z"
}
```

## 🚀 Testing with Swagger UI

### 1. Interactive Testing

- Use the **Try it out** button on any endpoint
- Fill in required parameters
- Execute the request
- View the response and status code

### 2. Parameter Validation

- Swagger UI validates required fields
- Shows parameter types and descriptions
- Provides example values where specified

### 3. Response Examples

- View expected response structures
- See error response formats
- Understand status codes

## 🔍 Troubleshooting

### Common Issues

1. **Swagger Not Loading**
   ```bash
   # Check if service is running
   curl http://localhost:8083/health
   
   # Check if docs directory exists
   ls -la docs/
   ```

2. **Missing Endpoints**
   ```bash
   # Regenerate documentation
   ./scripts/regenerate_swagger.sh
   
   # Check Swagger generation logs
   swag init -g main.go -d . --debug
   ```

3. **Model Documentation Issues**
   - Ensure all types have proper Swagger annotations
   - Check for circular references in models
   - Verify import paths are correct

### Debug Mode

Enable debug mode for Swagger generation:

```bash
swag init -g main.go -d . --debug --parseDependency --parseInternal
```

## 📝 Adding New Endpoints

When adding new renewal endpoints:

1. **Add Swagger annotations** to the handler method
2. **Define request/response models** with proper tags
3. **Update the router** to include the new endpoint
4. **Regenerate documentation** using the script
5. **Test the endpoint** in Swagger UI

### Example New Endpoint

```go
// @Summary Get renewal history
// @Description Returns renewal history for a specific subscription
// @Tags Renewal Monitoring
// @Produce json
// @Param subscription_id path int true "Subscription ID"
// @Success 200 {array} []*domain.RenewalCycle "Renewal history"
// @Failure 404 {string} string "Subscription not found"
// @Router /api/v1/renewal/history/{subscription_id} [get]
func (h *RenewalHandler) GetRenewalHistory(ctx *fasthttp.RequestCtx) {
    // Implementation
}
```

## 🌟 Benefits

### For Developers
- **Interactive API testing** without external tools
- **Clear endpoint documentation** with examples
- **Request/response validation** before implementation
- **Easy integration testing** with real endpoints

### For Operations
- **API monitoring** and health checks
- **Performance metrics** and statistics
- **Error handling** and status codes
- **System health** and worker status

### For Integration
- **Client SDK generation** from OpenAPI spec
- **API versioning** and change tracking
- **Compliance documentation** for audits
- **Team collaboration** on API design

## 📚 Additional Resources

- [Swagger/OpenAPI Specification](https://swagger.io/specification/)
- [Swaggo Documentation](https://github.com/swaggo/swag)
- [FastHTTP Swagger Integration](https://github.com/swaggo/fasthttp-swagger)
- [API Documentation Best Practices](https://swagger.io/blog/api-documentation/)

---

**Note**: The renewal system is now fully documented in Swagger, providing comprehensive API documentation, interactive testing, and clear integration guidelines for all stakeholders. 