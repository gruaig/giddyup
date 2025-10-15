#!/bin/bash
# Quick test to verify the schema fixes worked

echo "🧪 Quick API Test - Schema Fixes"
echo "=================================="
echo ""

# Check if server is running
if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
    echo "❌ Server is not running on port 8000"
    echo "   Start it with: LOG_LEVEL=DEBUG ./bin/api"
    exit 1
fi

echo "✅ Server is running"
echo ""

# Test courses endpoint (was failing with "relation courses does not exist")
echo "1. Testing GET /api/v1/courses..."
response=$(curl -s http://localhost:8000/api/v1/courses)
if echo "$response" | grep -q "error"; then
    echo "   ❌ FAILED: $response"
else
    count=$(echo "$response" | jq 'length' 2>/dev/null || echo "0")
    echo "   ✅ SUCCESS: Got $count courses"
fi

# Test races endpoint
echo ""
echo "2. Testing GET /api/v1/races?date=2024-01-01..."
response=$(curl -s "http://localhost:8000/api/v1/races?date=2024-01-01")
if echo "$response" | grep -q "error"; then
    echo "   ❌ FAILED: $response"
else
    count=$(echo "$response" | jq 'length' 2>/dev/null || echo "0")
    echo "   ✅ SUCCESS: Got $count races"
fi

# Test search endpoint
echo ""
echo "3. Testing GET /api/v1/search?q=enable..."
response=$(curl -s "http://localhost:8000/api/v1/search?q=enable")
if echo "$response" | grep -q "error"; then
    echo "   ❌ FAILED: $response"
else
    echo "   ✅ SUCCESS: Search returned results"
fi

echo ""
echo "=================================="
echo "Check logs/server.log for detailed error messages if any tests failed"
echo ""

