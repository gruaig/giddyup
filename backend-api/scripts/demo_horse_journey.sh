#!/bin/bash

# Demo: Complete Horse Information Journey
# Shows search -> select -> full profile with odds
# Usage: ./demo_horse_journey.sh [horse_name]
# Example: ./demo_horse_journey.sh "Enable"
# Example: ./demo_horse_journey.sh "Frankel"

BASE_URL="http://localhost:8000/api/v1"

# Get horse name from parameter or use default
HORSE_NAME=${1:-"Captain Scooby"}

echo "🐎 GiddyUp API - Complete Horse Journey Demo"
echo "=============================================="
echo "Searching for: $HORSE_NAME"
echo ""

# Step 1: Search for a horse
echo "Step 1: Searching for '$HORSE_NAME'..."
SEARCH_QUERY=$(echo "$HORSE_NAME" | sed 's/ /%20/g')
SEARCH_RESULT=$(curl -s "$BASE_URL/search?q=$SEARCH_QUERY&limit=5")
echo "$SEARCH_RESULT" | python3 -m json.tool | head -20
echo ""

# Extract horse ID
HORSE_ID=$(echo "$SEARCH_RESULT" | python3 -c "import json, sys; d=json.load(sys.stdin); print(d['horses'][0]['id'] if d.get('horses') and len(d['horses']) > 0 else 0)")

if [ "$HORSE_ID" = "0" ]; then
    echo "❌ Horse not found!"
    exit 1
fi

echo "✅ Found Horse ID: $HORSE_ID"
echo ""

# Step 2: Get complete profile
echo "Step 2: Getting complete profile for Horse ID $HORSE_ID..."
PROFILE=$(curl -s "$BASE_URL/horses/$HORSE_ID/profile")

echo ""
echo "=== CAREER SUMMARY ==="
echo "$PROFILE" | python3 -c "import json, sys; d=json.load(sys.stdin); cs=d['career_summary']; print(f\"Total Runs: {cs['runs']}\"); print(f\"Wins: {cs['wins']}\"); print(f\"Places: {cs['places']}\"); print(f\"Strike Rate: {cs['wins']/cs['runs']*100:.1f}%\"); print(f\"Peak RPR: {cs['peak_rpr']}\"); print(f\"Total Prize Money: £{cs['total_prize']:.2f}\")"

echo ""
echo "=== LAST 3 RUNS (with odds) ==="
echo "$PROFILE" | python3 -c "
import json, sys
d = json.load(sys.stdin)
for i, form in enumerate(d['recent_form'][:3]):
    print(f\"\\nRun {i+1}: {form['race_date'][:10]}\")
    print(f\"  Course: {form['course_name']}\")
    print(f\"  Race: {form['race_name']}\")
    print(f\"  Position: {form['pos_raw']}\")
    if form.get('win_bsp'):
        print(f\"  Betfair SP (BSP): {form['win_bsp']:.2f}\")
    if form.get('dec'):
        print(f\"  Bookmaker SP: {form['dec']:.2f}\")
    if form.get('rpr'):
        print(f\"  RPR: {form['rpr']}\")
    if form.get('or'):
        print(f\"  OR: {form['or']}\")
    print(f\"  Trainer: {form['trainer_name']}\")
    print(f\"  Jockey: {form['jockey_name']}\")
    if form.get('days_since_run'):
        print(f\"  Days Since Last Run: {form['days_since_run']}\")
"

echo ""
echo "=== GOING PERFORMANCE ==="
echo "$PROFILE" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f\"{'Going':<20} {'Runs':>5} {'Wins':>5} {'SR':>7} {'Avg RPR':>8}\")
print('-' * 55)
for split in d['going_splits'][:5]:
    avg_rpr = f\"{split['avg_rpr']:.1f}\" if split.get('avg_rpr') else 'N/A'
    print(f\"{split['category']:<20} {split['runs']:>5} {split['wins']:>5} {split['sr']:>6.1f}% {avg_rpr:>8}\")
"

echo ""
echo "=== DISTANCE PERFORMANCE ==="
echo "$PROFILE" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f\"{'Distance':<15} {'Runs':>5} {'Wins':>5} {'SR':>7} {'Avg RPR':>8}\")
print('-' * 50)
for split in d['distance_splits']:
    avg_rpr = f\"{split['avg_rpr']:.1f}\" if split.get('avg_rpr') else 'N/A'
    print(f\"{split['category']:<15} {split['runs']:>5} {split['wins']:>5} {split['sr']:>6.1f}% {avg_rpr:>8}\")
"

echo ""
echo "=== TOP COURSES ==="
echo "$PROFILE" | python3 -c "
import json, sys
d = json.load(sys.stdin)
print(f\"{'Course':<25} {'Runs':>5} {'Wins':>5} {'SR':>7}\")
print('-' * 50)
for split in d['course_splits'][:5]:
    print(f\"{split['category']:<25} {split['runs']:>5} {split['wins']:>5} {split['sr']:>6.1f}%\")
"

echo ""
echo "✅ Complete horse journey successful!"
echo ""
echo "Usage:"
echo "  ./demo_horse_journey.sh                    # Default (Captain Scooby)"
echo "  ./demo_horse_journey.sh \"Enable\"           # Search for Enable"
echo "  ./demo_horse_journey.sh \"Frankel\"          # Search for Frankel"
echo "  ./demo_horse_journey.sh \"Sea The Stars\"    # Multi-word names supported"

