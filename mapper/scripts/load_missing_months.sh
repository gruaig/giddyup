#!/bin/bash

# Load Missing Data (Feb 2024 - Oct 2025)
# This script stitches raw RP data with Betfair and loads to database

set -e

RPSCRAPE_DIR="/home/smonaghan/rpscrape"
HRMASTERSET_DIR="/home/smonaghan/hrmasterset"

echo "🔄 Loading Missing Data (Feb 2024 - Oct 2025)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# Step 1: Stitch data
echo "1️⃣  Stitching Racing Post + Betfair data..."
echo "   This will create master CSV files in $RPSCRAPE_DIR/master/"
echo ""

cd "$RPSCRAPE_DIR"

# Run stitcher for 2024
echo "   Processing 2024..."
python3 master_data_stitcher.py \
  --from 2024-02-01 \
  --to 2024-12-31 \
  --verbose 2>&1 | tee /tmp/stitch_2024.log

# Run stitcher for 2025
echo "   Processing 2025..."
python3 master_data_stitcher.py \
  --from 2025-01-01 \
  --to 2025-10-13 \
  --verbose 2>&1 | tee /tmp/stitch_2025.log

echo ""
echo "✅ Stitching complete!"
echo ""

# Step 2: Load to database
echo "2️⃣  Loading to PostgreSQL..."
echo ""

cd "$HRMASTERSET_DIR"

# Load 2024
echo "   Loading 2024..."
python3 load_master_to_postgres_v2.py \
  --master-dir "$RPSCRAPE_DIR/master" \
  --from 2024-02-01 \
  --to 2024-12-31 2>&1 | tee /tmp/load_2024.log

# Load 2025
echo "   Loading 2025..."
python3 load_master_to_postgres_v2.py \
  --master-dir "$RPSCRAPE_DIR/master" \
  --from 2025-01-01 \
  --to 2025-10-13 2>&1 | tee /tmp/load_2025.log

echo ""
echo "✅ Database load complete!"
echo ""

# Step 3: Verify
echo "3️⃣  Verifying data..."
echo ""

docker exec horse_racing psql -U postgres -d horse_db <<SQL
SET search_path TO racing, public;

SELECT 'Latest Race Date:' as metric, MAX(race_date)::text as value FROM races
UNION ALL
SELECT 'Total Races:', COUNT(*)::text FROM races
UNION ALL
SELECT 'Total Runners:', COUNT(*)::text FROM runners;

\echo ''
\echo 'Dancing in Paris Check:'

SELECT COUNT(*) as total_runs
FROM runners r
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL;

SELECT race_date::text, course_name, pos_num, "or"
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN horses h ON h.horse_id = r.horse_id
JOIN courses c ON c.course_id = ra.course_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL
ORDER BY ra.race_date DESC
LIMIT 5;

SQL

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ LOAD COMPLETE!"
echo ""
echo "Next: Run verification again:"
echo "  cd /home/smonaghan/GiddyUp/mapper"
echo "  ./bin/mapper verify --from 2024-01-01 --to 2025-10-13"
echo ""

