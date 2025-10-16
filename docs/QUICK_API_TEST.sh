#!/bin/bash
# Quick API test script for UI developers

echo "=== GiddyUp API Quick Test ==="
echo ""

BASE_URL="http://localhost:8000/api/v1"

echo "1️⃣  Health Check..."
curl -s "$BASE_URL/../health" | jq '.'
echo ""

echo "2️⃣  Today's Races (2025-10-15)..."
curl -s "$BASE_URL/races?date=2025-10-15" | jq '.races[] | {race_id, course, race_name, off_time, prelim, ran, runner_count: (.runners | length)}'
echo ""

echo "3️⃣  Sample Race with Runners..."
RACE_ID=$(curl -s "$BASE_URL/races?date=2025-10-15" | jq -r '.races[0].race_id')
echo "Race ID: $RACE_ID"
curl -s "$BASE_URL/races/$RACE_ID" | jq '{race_name, prelim, runners: .runners[] | {horse, form, headgear, comment, win_ppwap, win_ppmax, win_ppmin}}'
echo ""

echo "4️⃣  Tomorrow's Races (2025-10-16)..."
curl -s "$BASE_URL/races?date=2025-10-16" | jq '.races[] | {course, race_name, off_time, prelim}'
echo ""

echo "5️⃣  Live Price Example..."
echo "Fetching prices now..."
PRICE1=$(curl -s "$BASE_URL/races/$RACE_ID" | jq -r '.runners[0].win_ppwap')
echo "Current price: $PRICE1"
echo "Waiting 60 seconds for price update..."
sleep 60
PRICE2=$(curl -s "$BASE_URL/races/$RACE_ID" | jq -r '.runners[0].win_ppwap')
echo "Updated price: $PRICE2"
if [ "$PRICE1" != "$PRICE2" ]; then
  echo "✅ Prices are updating!"
else
  echo "⚠️  Price unchanged (may be same or market not open)"
fi

echo ""
echo "✅ Test complete!"
