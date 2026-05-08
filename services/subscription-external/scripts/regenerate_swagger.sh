#!/bin/bash

# Script to regenerate Swagger documentation for the renewal system
# This script updates the Swagger docs to include all renewal endpoints

set -e

echo "🔄 Regenerating Swagger documentation..."

# Check if swag is installed
if ! command -v swag &> /dev/null; then
    echo "❌ swag command not found. Installing..."
    go install github.com/swaggo/swag/cmd/swag@latest
fi

# Navigate to the cmd directory
cd cmd

# Clean up old docs
echo "🧹 Cleaning up old documentation..."
rm -rf ../docs

# Generate new Swagger documentation
echo "📚 Generating new Swagger documentation..."
swag init -g main.go -d .,../internal,../internal/handler,../internal/service,../internal/transport,../internal/worker,../internal/domain -o ../docs --instanceName swagger

# Check if generation was successful
if [ $? -eq 0 ]; then
    echo "✅ Swagger documentation generated successfully!"
    echo "📁 Documentation saved to: ../docs/"
    
    # List generated files
    echo "📋 Generated files:"
    ls -la ../docs/
    
    echo ""
    echo "🌐 To view the documentation:"
    echo "   1. Start the service: go run cmd/main.go"
    echo "   2. Open browser: http://localhost:8083/swagger/"
    
else
    echo "❌ Failed to generate Swagger documentation"
    exit 1
fi

echo ""
echo "🎉 Swagger documentation regeneration complete!"
echo "   The renewal system endpoints are now documented in the API docs." 