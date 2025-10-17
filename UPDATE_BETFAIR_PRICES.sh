#!/bin/bash

# Update Betfair Prices - Continuous updater for betting script
# Fetches and updates Betfair PPWAP prices for tomorrow's races

set -e

DATE="${1:-$(date -d tomorrow +%Y-%m-%d)}"

echo "════════════════════════════════════════════════════════════════════"
echo "         BETFAIR PRICE UPDATER - $(date)"
echo "════════════════════════════════════════════════════════════════════"
echo ""
echo "Target Date: $DATE"
echo ""

# Check if races exist
echo "📋 Checking if races exist for $DATE..."
RACE_COUNT=$(docker exec horse_racing psql -U postgres -d horse_db -t -c \
    "SELECT COUNT(*) FROM racing.races WHERE race_date = '$DATE';")

if [ "$RACE_COUNT" -eq 0 ]; then
    echo "❌ No races found for $DATE"
    echo "   → Run: cd backend-api && ./fetch_all $DATE"
    exit 1
fi

echo "✅ Found $RACE_COUNT races"
echo ""

# Check Betfair selection ID coverage
echo "🔍 Checking Betfair selection ID coverage..."
BETFAIR_STATS=$(docker exec horse_racing psql -U postgres -d horse_db -t -c "
    SELECT 
        COUNT(*) as total,
        COUNT(*) FILTER (WHERE betfair_selection_id IS NOT NULL) as with_id,
        ROUND(100.0 * COUNT(*) FILTER (WHERE betfair_selection_id IS NOT NULL) / COUNT(*)) as pct
    FROM racing.runners ru
    JOIN racing.races r ON r.race_id = ru.race_id
    WHERE r.race_date = '$DATE';
")

echo "   $BETFAIR_STATS"
echo ""

# Show current price status
echo "💰 Current Price Status:"
docker exec horse_racing psql -U postgres -d horse_db -c "
    SELECT 
        COUNT(DISTINCT r.race_id) as races,
        COUNT(ru.runner_id) as runners,
        COUNT(*) FILTER (WHERE ru.win_ppwap IS NOT NULL) as have_ppwap,
        COUNT(*) FILTER (WHERE ru.dec IS NOT NULL) as have_dec,
        ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_ppwap IS NOT NULL OR ru.dec IS NOT NULL) / COUNT(*)) as pct_ready
    FROM racing.runners ru
    JOIN racing.races r ON r.race_id = ru.race_id
    WHERE r.race_date = '$DATE';
" | head -5

echo ""
echo "════════════════════════════════════════════════════════════════════"
echo ""
echo "⚠️  BETFAIR API INTEGRATION REQUIRED"
echo ""
echo "To enable automatic price updates, you need:"
echo ""
echo "1. Betfair API Credentials:"
echo "   • App Key (from Betfair developer portal)"
echo "   • Session Token (login certificate)"
echo ""
echo "2. Add to settings.env:"
echo "   export BETFAIR_APP_KEY='your_app_key'"
echo "   export BETFAIR_SESSION_TOKEN='your_token'"
echo ""
echo "3. Then this script will:"
echo "   • Discover markets for $DATE"
echo "   • Fetch prices every 30 minutes"
echo "   • Update racing.runners.win_ppwap"
echo "   • Calculate PPWAP (weighted average price)"
echo ""
echo "════════════════════════════════════════════════════════════════════"
echo ""

# Check if Betfair credentials exist
if [ -z "$BETFAIR_APP_KEY" ]; then
    echo "❌ BETFAIR_APP_KEY not set in environment"
    echo ""
    echo "MANUAL WORKAROUND (for testing):"
    echo "════════════════════════════════════════════════════════════════════"
    echo ""
    echo "# Copy decimal odds to win_ppwap as temporary fallback:"
    echo ""
    echo "docker exec horse_racing psql -U postgres -d horse_db -c \\"
    echo "    \"UPDATE racing.runners ru"
    echo "     SET win_ppwap = ru.dec"
    echo "     FROM racing.races r"
    echo "     WHERE ru.race_id = r.race_id"
    echo "     AND r.race_date = '$DATE'"
    echo "     AND ru.win_ppwap IS NULL"
    echo "     AND ru.dec IS NOT NULL;\""
    echo ""
    echo "⚠️  This uses bookmaker odds (dec) instead of Betfair exchange prices"
    echo "⚠️  Not ideal but allows betting script to run for testing"
    echo ""
    exit 1
fi

# If credentials exist, would fetch prices here
echo "✅ Betfair credentials found"
echo "🔄 Fetching prices..."
echo ""
echo "⚠️  TODO: Implement Betfair API price fetching"
echo ""

