#!/bin/bash

# Verification script to test backend API is returning complete data

echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║                                                                      ║"
echo "║            Backend API Data Verification Test                        ║"
echo "║                                                                      ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""

API_URL="http://localhost:8000"

echo "1️⃣  Testing server health..."
HEALTH=$(curl -s "$API_URL/health" 2>/dev/null)
if [ "$HEALTH" = '{"status":"healthy"}' ]; then
    echo "   ✅ Server is healthy"
else
    echo "   ❌ Server not responding"
    exit 1
fi
echo ""

echo "2️⃣  Testing meetings endpoint for today..."
TODAY=$(date +%Y-%m-%d)
MEETINGS=$(curl -s "$API_URL/api/v1/meetings?date=$TODAY" 2>/dev/null)
MEETING_COUNT=$(echo "$MEETINGS" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d))" 2>/dev/null || echo "0")
RACE_COUNT=$(echo "$MEETINGS" | python3 -c "import json,sys; d=json.load(sys.stdin); print(sum(len(m.get('races',[])) for m in d))" 2>/dev/null || echo "0")

echo "   Meetings: $MEETING_COUNT"
echo "   Total Races: $RACE_COUNT"

if [ "$RACE_COUNT" -gt "100" ]; then
    echo "   ⚠️  WARNING: More than 100 races = likely duplicates!"
elif [ "$RACE_COUNT" -gt "0" ]; then
    echo "   ✅ Race data present"
else
    echo "   ❌ No races found"
fi
echo ""

echo "3️⃣  Testing individual race endpoint (with runners)..."
# Get first race ID
RACE_ID=$(echo "$MEETINGS" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d[0]['races'][0]['race_id'] if d and d[0].get('races') else 0)" 2>/dev/null || echo "0")

if [ "$RACE_ID" != "0" ]; then
    echo "   Testing race_id: $RACE_ID"
    RACE_DATA=$(curl -s "$API_URL/api/v1/races/$RACE_ID" 2>/dev/null)
    
    RACE_NAME=$(echo "$RACE_DATA" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('race',{}).get('race_name','')[:50])" 2>/dev/null)
    RUNNER_COUNT=$(echo "$RACE_DATA" | python3 -c "import json,sys; d=json.load(sys.stdin); print(len(d.get('runners',[])))" 2>/dev/null || echo "0")
    
    echo "   Race: $RACE_NAME"
    echo "   Runners: $RUNNER_COUNT"
    
    if [ "$RUNNER_COUNT" -gt "0" ]; then
        echo "   ✅ Runners present in API response"
        echo ""
        echo "   Sample runners:"
        echo "$RACE_DATA" | python3 -c "
import json, sys
d = json.load(sys.stdin)
runners = d.get('runners', [])[:3]
for r in runners:
    name = r.get('horse_name', 'NULL')
    draw = r.get('draw', '-')
    trainer = r.get('trainer_name', 'NULL')
    jockey = r.get('jockey_name', 'NULL')
    price = r.get('win_ppwap', '-')
    print(f'      • {name:25} Draw:{draw:2} Trainer:{trainer:20} Price:{price}')
" 2>/dev/null
    else
        echo "   ❌ No runners in response!"
    fi
else
    echo "   ❌ No race_id found"
fi
echo ""

echo "4️⃣  Testing horse profile..."
curl -s "$API_URL/api/v1/horses/2131337/profile" 2>/dev/null | python3 -c "
import json, sys
try:
    d = json.load(sys.stdin)
    horse = d.get('horse', {})
    form = d.get('recent_form', [])
    print(f'   Horse: {horse.get(\"horse_name\", \"?\")}')
    print(f'   Career runs: {d.get(\"career_summary\", {}).get(\"runs\", 0)}')
    print(f'   Recent form entries: {len(form)}')
    if form:
        print(f'   Latest race:')
        latest = form[0]
        print(f'      Course: {latest.get(\"course_name\", \"NULL\")}')
        print(f'      Position: {latest.get(\"pos_num\", \"-\")}')
        print(f'      BTN: {latest.get(\"btn\", \"-\")}')
        print(f'   ✅ Profile data complete!')
    else:
        print('   ❌ No form data')
except Exception as e:
    print(f'   ❌ Error: {e}')
" 2>/dev/null
echo ""

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "RESULT:"
echo "  If all tests show ✅ → Backend is working, UI has display bug"
echo "  If tests show ❌ → Backend issue, investigate further"
echo ""
echo "For UI issues, send docs/UI_LIVE_PRICES_UPDATE.md to your developer"
echo ""

