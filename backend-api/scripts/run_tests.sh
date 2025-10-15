#!/bin/bash

# GiddyUp API End-to-End Test Runner

echo "🧪 GiddyUp API End-to-End Tests"
echo "================================"
echo ""

# Check if server is running
echo "Checking if API server is running..."
if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
    echo "❌ API server is not running!"
    echo "Please start the server first:"
    echo "  cd /home/smonaghan/GiddyUp/backend-api"
    echo "  ./bin/api"
    exit 1
fi

echo "✅ API server is running"
echo ""

# Run the tests
echo "Running end-to-end tests..."
echo ""

cd /home/smonaghan/GiddyUp/backend-api

# Run tests with verbose output
go test -v ./tests/... -timeout 5m

# Check test result
if [ $? -eq 0 ]; then
    echo ""
    echo "✅ All tests PASSED!"
    echo ""
else
    echo ""
    echo "❌ Some tests FAILED!"
    echo ""
    exit 1
fi

