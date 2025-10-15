#!/bin/bash

# Test: Load a Full Race at 14:40 with Complete Details
# Race: Haydock, 2025-09-27, 14:40, 12 runners

RACE_ID=739012
BASE_URL="http://localhost:8000/api/v1"

echo "════════════════════════════════════════════════════════════"
echo "  FULL RACE LOAD TEST - Race ID: $RACE_ID"
echo "════════════════════════════════════════════════════════════"
echo ""

# Measure timing
START=$(date +%s%3N)

# Get full race details
RESPONSE=$(curl -s "${BASE_URL}/races/${RACE_ID}")

END=$(date +%s%3N)
LATENCY=$((END - START))

echo "⏱️  Load Time: ${LATENCY}ms"
echo ""

# Parse and display race details
echo "📋 RACE DETAILS"
echo "─────────────────────────────────────────────────────────────"
echo "$RESPONSE" | jq -r '.race | 
"Date:         \(.race_date[:10])
Time:         \(.off_time)
Course:       \(.course_name)
Race Name:    \(.race_name)
Type:         \(.race_type)
Class:        \(.class // "N/A")
Distance:     \(.dist_f)f (\(.dist_raw // "N/A"))
Going:        \(.going // "N/A")
Surface:      \(.surface // "N/A")
Total Runners:\(.ran)"'

echo ""
echo "🏇 COMPLETE FIELD (All \$(echo "$RESPONSE" | jq -r '.race.ran') Runners)"
echo "─────────────────────────────────────────────────────────────"
echo ""

# Display all runners with full details
echo "$RESPONSE" | jq -r '.runners[] | 
"Runner #\(.num // "?")  Draw: \(.draw // "N/A")  Pos: \(.pos_raw // "?")
═══════════════════════════════════════════════════════════
Horse:     \(.horse_name)
Age/Sex:   \(.age // "?")yr \(.sex // "?")  Weight: \(.lbs // "?")lbs
Jockey:    \(.jockey_name // "Unknown")
Trainer:   \(.trainer_name // "Unknown")

RATINGS:
  Official:  \(.or // "N/A")
  RPR:       \(.rpr // "N/A")

ODDS:
  BSP:       \(.win_bsp // "N/A")
  SP:        \(.dec // "N/A")  
  Morning:   \(.win_ppwap // "N/A")

RESULT:
  Position:  \(.pos_num // "DNF")
  Beaten:    \(.btn // "N/A") lengths
  Winner:    \(if .win_flag then "✅ YES" else "❌ No" end)

COMMENT:
  \(.comment // "No comment")

"'

echo ""
echo "════════════════════════════════════════════════════════════"
echo "  ✅ Test Complete - Total Time: ${LATENCY}ms"
echo "════════════════════════════════════════════════════════════"
