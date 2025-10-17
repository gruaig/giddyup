#!/bin/bash

# Check if database is ready for betting script
# Run this before get_tomorrows_bets.sh

DATE="${1:-$(date -d tomorrow +%Y-%m-%d)}"

echo "════════════════════════════════════════════════════════════════════"
echo "         BETTING SCRIPT READINESS CHECK"
echo "════════════════════════════════════════════════════════════════════"
echo ""
echo "Target Date: $DATE"
echo ""

docker exec horse_racing psql -U postgres -d horse_db << SQL

-- Main readiness check
SELECT 
    '📊 DATA SUMMARY' as section,
    COUNT(DISTINCT r.race_id) as races,
    COUNT(ru.runner_id) as total_runners
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.race_date = '$DATE';

SELECT '';

-- Foreign key coverage
SELECT 
    '🔗 FOREIGN KEY COVERAGE' as section,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE horse_id IS NOT NULL) as have_horse_id,
    COUNT(*) FILTER (WHERE trainer_id IS NOT NULL) as have_trainer_id,
    COUNT(*) FILTER (WHERE jockey_id IS NOT NULL) as have_jockey_id,
    ROUND(100.0 * COUNT(*) FILTER (WHERE horse_id IS NOT NULL) / COUNT(*)) as pct_horses,
    ROUND(100.0 * COUNT(*) FILTER (WHERE trainer_id IS NOT NULL) / COUNT(*)) as pct_trainers
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.race_date = '$DATE';

SELECT '';

-- CRITICAL: Odds coverage
SELECT 
    '💰 ODDS COVERAGE (CRITICAL!)' as section,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL) as have_betfair,
    COUNT(*) FILTER (WHERE dec IS NOT NULL) as have_book,
    COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL OR dec IS NOT NULL) as have_any,
    ROUND(100.0 * COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL) / COUNT(*)) as pct_betfair,
    ROUND(100.0 * COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL OR dec IS NOT NULL) / COUNT(*)) as pct_any,
    CASE 
        WHEN ROUND(100.0 * COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL OR dec IS NOT NULL) / COUNT(*)) >= 80 THEN '✅ READY'
        WHEN ROUND(100.0 * COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL OR dec IS NOT NULL) / COUNT(*)) >= 50 THEN '⚠️  PARTIAL'
        ELSE '❌ NOT READY'
    END as status
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.race_date = '$DATE';

SELECT '';

-- Betfair selection ID coverage
SELECT 
    '🎯 BETFAIR SELECTION IDS' as section,
    COUNT(*) as total,
    COUNT(*) FILTER (WHERE betfair_selection_id IS NOT NULL) as have_id,
    ROUND(100.0 * COUNT(*) FILTER (WHERE betfair_selection_id IS NOT NULL) / COUNT(*)) as pct
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.race_date = '$DATE';

SELECT '';

-- Sample prices (show first 10 runners with odds)
SELECT 
    '💵 SAMPLE PRICES' as info,
    h.horse_name,
    c.course_name,
    ru.win_ppwap as betfair,
    ru.dec as bookmaker
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id
LEFT JOIN racing.courses c ON c.course_id = r.course_id
WHERE r.race_date = '$DATE'
AND (ru.win_ppwap IS NOT NULL OR ru.dec IS NOT NULL)
ORDER BY r.off_time, ru.num
LIMIT 10;

SQL

echo ""
echo "════════════════════════════════════════════════════════════════════"
echo ""
echo "RECOMMENDATION:"
echo ""
echo "  ✅ >= 80% odds coverage → RUN BETTING SCRIPT"
echo "  ⚠️  50-79% coverage → Partial results possible"
echo "  ❌ < 50% coverage → Wait for more prices"
echo ""
echo "════════════════════════════════════════════════════════════════════"
echo ""

