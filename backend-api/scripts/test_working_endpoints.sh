#!/bin/bash

# Quick test of working endpoints

BASE_URL="http://localhost:8000"

echo "🧪 Testing GiddyUp API Endpoints"
echo "================================="
echo ""

echo "1️⃣  Health Check"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/health" | head -1
echo ""

echo "2️⃣  Get All Courses (89 courses)"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/courses" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Found {len(d)} courses')" 2>&1 | head -2
echo ""

echo "3️⃣  Global Search (Frankel)"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/search?q=Frankel&limit=3" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Found {d[\"total_results\"]} total results'); print(f'  Top horse: {d[\"horses\"][0][\"name\"]} (score: {d[\"horses\"][0][\"score\"]:.2f})')" 2>&1 | head -3
echo ""

echo "4️⃣  Search Races (2024-01-01)"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/races?date=2024-01-01&limit=5" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Found {len(d)} races')" 2>&1 | head -2
echo ""

echo "5️⃣  Race Search with Filters"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/races/search?date_from=2024-01-01&date_to=2024-01-02&region=GB&limit=10" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Found {len(d)} GB races')" 2>&1 | head -2
echo ""

echo "6️⃣  Single Race Details (Race ID 339)"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/races/339" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Race: {d[\"race\"][\"race_name\"]}'); print(f'  Runners: {len(d[\"runners\"])}')" 2>&1 | head -3
echo ""

echo "7️⃣  Comment Search"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/search/comments?q=led&limit=3" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Found {len(d)} comments')" 2>&1 | head -2
echo ""

echo "8️⃣  Draw Bias Analysis (Aintree 5-7f)"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/bias/draw?course_id=82&dist_min=5&dist_max=7" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  Analyzed {len(d)} draw positions')" 2>&1 | head -2
echo ""

echo "9️⃣  Market Calibration"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/market/calibration/win?date_from=2024-01-01&date_to=2024-01-31" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  {len(d)} price bins analyzed')" 2>&1 | head -2
echo ""

echo "🔟  Book vs Exchange"
curl -s -w "\n  Status: %{http_code}, Time: %{time_total}s\n" "$BASE_URL/api/v1/market/book-vs-exchange?date_from=2024-01-01&date_to=2024-01-02" | python3 -c "import json, sys; d=json.load(sys.stdin); print(f'  {len(d)} days compared')" 2>&1 | head -2
echo ""

echo "================================="
echo "✅ API Endpoint Tests Complete"

