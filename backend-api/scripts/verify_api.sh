#!/bin/bash

# Simple API Verification

BASE="http://localhost:8000/api/v1"

echo "🧪 GiddyUp API Quick Verification"
echo "===================================="
echo ""

echo "✅ Health Check:"
curl -s http://localhost:8000/health
echo -e "\n"

echo "✅ Courses (showing first 3):"
curl -s "$BASE/courses" | python3 -m json.tool | head -15
echo "..."
echo ""

echo "✅ Search for 'Frankel':"
curl -s "$BASE/search?q=Frankel&limit=2" | python3 -m json.tool
echo ""

echo "✅ Races on 2024-01-01 (showing first race):"
curl -s "$BASE/races?date=2024-01-01&limit=1" | python3 -m json.tool | head -25
echo "..."
echo ""

echo "✅ Race Details (ID 339):"
curl -s "$BASE/races/339" | python3 -m json.tool | head -30
echo "..."
echo ""

echo "===================================="
echo "Server Log Summary:"
echo ""
tail -10 /tmp/giddyup-api.log | grep INFO
echo ""
echo "For full logs: tail -f /tmp/giddyup-api.log"

