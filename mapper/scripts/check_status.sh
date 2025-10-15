#!/bin/bash

# Check Data Pipeline Status

echo "🔍 DATA PIPELINE STATUS CHECK"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

# 1. Racing Post Raw Data
echo "1️⃣  Racing Post Raw Data (/rpscrape/data/dates/):"
RP_2024=$(ls /home/smonaghan/rpscrape/data/dates/gb/flat/2024_*.csv 2>/dev/null | wc -l)
RP_2025=$(ls /home/smonaghan/rpscrape/data/dates/gb/flat/2025_*.csv 2>/dev/null | wc -l)
echo "   2024: $RP_2024 months"
echo "   2025: $RP_2025 months"
echo "   Status: $([ $RP_2024 -eq 12 ] && [ $RP_2025 -ge 10 ] && echo '✅ Complete' || echo '⚠️ Missing months')"
echo ""

# 2. Betfair Data
echo "2️⃣  Betfair Stitched Data (/rpscrape/data/betfair_stitched/):"
BF_2024=$(ls /home/smonaghan/rpscrape/data/betfair_stitched/gb/flat/ 2>/dev/null | grep "2024-" | wc -l)
BF_2025=$(ls /home/smonaghan/rpscrape/data/betfair_stitched/gb/flat/ 2>/dev/null | grep "2025-" | wc -l)
echo "   2024 races: $BF_2024"
echo "   2025 races: $BF_2025"
echo "   Status: $([ $BF_2024 -gt 1000 ] && echo '✅ Has data' || echo '⚠️ Limited data')"
echo ""

# 3. Master Files (Stitched)
echo "3️⃣  Master Files (/rpscrape/master/):"
MASTER_2024=$(ls /home/smonaghan/rpscrape/master/gb/flat/ 2>/dev/null | grep "2024-" | wc -l)
MASTER_2025=$(ls /home/smonaghan/rpscrape/master/gb/flat/ 2>/dev/null | grep "2025-" | wc -l)
echo "   2024: $MASTER_2024 months"
echo "   2025: $MASTER_2025 months"
echo "   Status: $([ $MASTER_2024 -eq 12 ] && [ $MASTER_2025 -ge 10 ] && echo '✅ Complete' || echo '⏳ Stitching in progress...')"
echo ""

# 4. Stitcher Process
echo "4️⃣  Stitcher Process:"
if pgrep -f "master_data_stitcher" > /dev/null; then
    STITCHER_PID=$(pgrep -f "master_data_stitcher" | head -1)
    STITCHER_MEM=$(ps -p $STITCHER_PID -o %mem= 2>/dev/null | xargs)
    STITCHER_CPU=$(ps -p $STITCHER_PID -o %cpu= 2>/dev/null | xargs)
    echo "   Status: 🔄 RUNNING"
    echo "   PID: $STITCHER_PID"
    echo "   CPU: $STITCHER_CPU%"
    echo "   Memory: $STITCHER_MEM%"
else
    echo "   Status: ⏹️  NOT RUNNING"
fi
echo ""

# 5. Database Status
echo "5️⃣  Database Status:"
DB_LATEST=$(docker exec horse_racing psql -U postgres -d horse_db -t -c \
    "SET search_path TO racing, public; SELECT MAX(race_date)::text FROM races;" 2>/dev/null | xargs)
DB_RACES=$(docker exec horse_racing psql -U postgres -d horse_db -t -c \
    "SET search_path TO racing, public; SELECT COUNT(*)::text FROM races;" 2>/dev/null | xargs)

echo "   Latest date: $DB_LATEST"
echo "   Total races: $DB_RACES"
echo "   Status: $([ "$DB_LATEST" \> "2025-09-01" ] && echo '✅ Current' || echo '❌ Outdated')"
echo ""

# 6. Dancing in Paris Check
echo "6️⃣  Dancing in Paris Verification:"
DIP_RUNS=$(docker exec horse_racing psql -U postgres -d horse_db -t -c \
    "SET search_path TO racing, public; 
     SELECT COUNT(*)::text FROM runners r 
     JOIN horses h ON h.horse_id = r.horse_id 
     WHERE h.horse_name LIKE '%Dancing%Paris%' AND r.pos_num IS NOT NULL;" 2>/dev/null | xargs)

echo "   Runs in DB: $DIP_RUNS"
echo "   Expected: 33"
echo "   Status: $([ "$DIP_RUNS" = "33" ] && echo '✅ COMPLETE' || echo "❌ Missing $(( 33 - ${DIP_RUNS:-0} )) runs")"

echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""

if [ "$DIP_RUNS" = "33" ] && [ "$DB_LATEST" \> "2025-09-01" ]; then
    echo "🎉 SUCCESS! Database is complete and current!"
elif pgrep -f "master_data_stitcher" > /dev/null; then
    echo "⏳ Stitcher running - wait for completion..."
    echo "   Monitor: ps aux | grep stitcher"
elif [ $MASTER_2024 -lt 12 ] || [ $MASTER_2025 -lt 10 ]; then
    echo "⚠️  Master files incomplete - stitcher may have failed"
    echo "   Check: ls /home/smonaghan/rpscrape/master/gb/flat/"
else
    echo "✅ Master files ready - run loader now:"
    echo "   cd /home/smonaghan/hrmasterset"
    echo "   python3 load_master_to_postgres_v2.py"
fi

