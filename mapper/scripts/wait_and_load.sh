#!/bin/bash

# Wait for Stitcher and Auto-Load to Database
# Monitors stitcher progress and loads data when ready

set -e

MASTER_DIR="/home/smonaghan/rpscrape/master"
LOADER_DIR="/home/smonaghan/hrmasterset"

echo "⏳ Waiting for stitcher to complete..."
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Monitor stitcher process
while pgrep -f "master_data_stitcher" > /dev/null; do
    # Count how many 2024-2025 master folders exist
    COUNT=$(ls $MASTER_DIR/gb/flat/ 2>/dev/null | grep -E "2024-|2025-" | wc -l)
    echo "$(date '+%H:%M:%S') - Master folders created: $COUNT/24"
    sleep 10
done

echo ""
echo "✅ Stitcher complete!"
echo ""

# Check what was created
CREATED=$(ls $MASTER_DIR/gb/flat/ | grep -E "2024-|2025-" | wc -l)
echo "📁 Created $CREATED master month folders for 2024-2025"
echo ""

if [ $CREATED -eq 0 ]; then
    echo "❌ No master files created! Check stitcher logs."
    exit 1
fi

echo "🔄 Loading to database..."
echo ""

cd "$LOADER_DIR"

# Run loader
python3 load_master_to_postgres_v2.py 2>&1 | tee /tmp/load_results.log

echo ""
echo "✅ Load complete!"
echo ""

# Verify Dancing in Paris
echo "🐎 Verifying Dancing in Paris..."
echo ""

docker exec horse_racing psql -U postgres -d horse_db <<SQL
SET search_path TO racing, public;

SELECT 
    'Total Runs:' AS metric,
    COUNT(*)::text AS value
FROM runners r
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL

UNION ALL

SELECT 
    'Latest Race:',
    MAX(ra.race_date)::text
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%'

UNION ALL

SELECT
    'Wins:',
    SUM(r.win_flag::int)::text
FROM runners r
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%';

SQL

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ COMPLETE!"
echo ""
echo "Run full verification:"
echo "  cd /home/smonaghan/GiddyUp/mapper"
echo "  ./bin/mapper verify --db-name horse_db --from 2024-01-01 --to 2025-10-13"

