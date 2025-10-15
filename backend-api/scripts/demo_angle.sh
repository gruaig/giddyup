#!/bin/bash

# Demo: Near-Miss-No-Hike Angle (search for horses that finished close 2nd LTO)
# Usage: ./demo_angle.sh [date]
# Example: ./demo_angle.sh 2024-01-15

BASE_URL="http://localhost:8000/api/v1/angles/near-miss-no-hike"

# Get date from parameter or use default
TARGET_DATE=${1:-2024-01-15}

# Calculate backtest window (month containing the date)
YEAR_MONTH=$(echo $TARGET_DATE | cut -d'-' -f1,2)
DATE_FROM="${YEAR_MONTH}-01"
# Last day of month (approximation)
DATE_TO="${YEAR_MONTH}-31"

echo "🎯 Near-Miss-No-Hike Angle Demo"
echo "==============================="
echo "Target Date: $TARGET_DATE"
echo ""
echo "Strategy: Find horses that:"
echo "  - Finished 2nd last time out (LTO)"
echo "  - Beaten ≤ 3 lengths"
echo "  - Running again within 14 days"
echo "  - No OR increase (rating penalty)"
echo "  - Same surface"
echo ""

echo "📊 BACKTEST MODE: $DATE_FROM to $DATE_TO"
echo "================================"
echo ""

BACKTEST=$(curl -s "$BASE_URL/past?date_from=$DATE_FROM&date_to=$DATE_TO&limit=10")

echo "Summary:"
echo "$BACKTEST" | python3 -c "
import json, sys
d = json.load(sys.stdin)
if 'summary' in d and d['summary']:
    s = d['summary']
    print(f\"  Total Qualifiers: {s['n']}\")
    print(f\"  Winners: {s['wins']}\")
    print(f\"  Strike Rate: {s['win_rate']*100:.1f}%\")
    if s.get('roi') is not None:
        print(f\"  ROI: {s['roi']*100:+.2f}%\")
"

echo ""
echo "Sample Cases (first 5):"
echo "$BACKTEST" | python3 -c "
import json, sys
d = json.load(sys.stdin)
for i, c in enumerate(d.get('cases', [])[:5]):
    print(f\"\\n{i+1}. {c['horse_name']}\")
    print(f\"   Last Run: {c['last_date'][:10]} - Pos {c['last_pos']}, Beaten {c.get('last_btn', 'N/A')}L\")
    print(f\"   Next Run: {c['next_date'][:10]} - Pos {c.get('next_pos', '?')}, Won: {c['next_win']}\")
    print(f\"   Days Between: {c['dsr']} days\")
    print(f\"   OR Change: {c['rating_change']:+d}\")
    if c.get('price'):
        print(f\"   BSP: {c['price']:.2f}\")
        pl = (c['price'] - 1) if c['next_win'] else -1
        print(f\"   P/L: {pl:+.2f} units\")
"

echo ""
echo "📅 TODAY'S QUALIFIERS MODE"
echo "=========================="
echo "Date: $TARGET_DATE"
echo ""

TODAY=$(curl -s "$BASE_URL/today?on=$TARGET_DATE&limit=5")

# Check if it's an array or error
echo "$TODAY" | python3 -c "
import json, sys
try:
    data = json.load(sys.stdin)
    # Check if it's a list (successful) or dict with error
    if isinstance(data, dict) and 'error' in data:
        print(f\"  Error: {data['error']}\")
        print('  Note: Need racecard data (races with pos_raw=NULL) for this feature')
    elif isinstance(data, list):
        if len(data) == 0:
            print('  No qualifiers found for this date')
        else:
            print(f'  Found {len(data)} qualifiers:\\n')
            for i, q in enumerate(data):
                print(f\"{i+1}. {q['horse_name']}\")
                print(f\"   Upcoming: {q['entry']['date'][:10]} at Course ID {q['entry']['course_id']}\")
                print(f\"   Last Run: {q['last']['date'][:10]} - 2nd, Beaten {q['last'].get('btn', 'N/A')}L\")
                print(f\"   Quick Return: {q['dsr']} days since last run\")
                print(f\"   OR: {q['last'].get('or', 'N/A')} → {q['entry'].get('or', 'N/A')} (change: {q['rating_change']:+d})\")
                print('')
    else:
        print('  No qualifiers found (need racecard data with pos_raw=NULL)')
except Exception as e:
    print(f'  Error parsing response: {e}')
    print('  Note: Racecard data needs to be loaded for today mode')
"

echo ""
echo "✅ Angle implementation complete!"
echo ""
echo "API Endpoints:"
echo "  GET $BASE_URL/past     - Backtest historical performance"
echo "  GET $BASE_URL/today    - Today's qualifiers"
echo ""
echo "Usage:"
echo "  ./demo_angle.sh              # Use default date (2024-01-15)"
echo "  ./demo_angle.sh 2024-01-20   # Test specific date"
echo "  ./demo_angle.sh \$(date +%Y-%m-%d)  # Use today's date"
echo ""
echo "See backend-api/tests/angle_test.go for comprehensive tests"

