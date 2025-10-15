#!/bin/bash

# Build Mapper Service

set -e

cd "$(dirname "$0")"

echo "🔨 Building Mapper Service..."

# Download dependencies
echo "📦 Downloading dependencies..."
go mod download

# Build binary
echo "🏗️  Building binary..."
mkdir -p bin
go build -o bin/mapper cmd/mapper/main.go

echo "✅ Build complete!"
echo ""
echo "Try it out:"
echo "  ./bin/mapper test-db                    # Test DB connection"
echo "  ./bin/mapper verify --today             # Verify today's data"
echo "  ./bin/mapper verify --from 2024-10-01   # Verify date range"
echo "  ./bin/mapper verify --verbose           # Detailed output"

