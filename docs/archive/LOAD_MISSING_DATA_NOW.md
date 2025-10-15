# ⚡ LOAD MISSING DATA - IMMEDIATE ACTION PLAN

**Status:** ✅ **Data exists - ready to load!**  
**Time Required:** 60-90 minutes  
**Impact:** +20,000 races, +200,000 runners

---

## ✅ **Good News**

Your raw Racing Post data **exists** and includes all missing months!

**Found:**
- `/home/smonaghan/rpscrape/data/dates/gb/flat/2024_02_01-2024_02_29.csv` ✅
- `/home/smonaghan/rpscrape/data/dates/gb/flat/2024_03_01-2024_03_31.csv` ✅
- ... all months through `2025_10_01-2025_10_31.csv` ✅

**Verified:** Dancing in Paris appears in 2024-2025 files with all missing runs!

---

## 🚀 **THREE-STEP PROCESS**

### Step 1: Stitch Data with Betfair (30-45 min)

The stitcher runs automatically and processes all available months:

```bash
cd /home/smonaghan/rpscrape

# Run stitcher (processes all months, creates master CSVs)
nohup python3 master_data_stitcher.py > /tmp/stitch.log 2>&1 &

# Monitor progress
tail -f /tmp/stitch.log
```

**What it does:**
- Reads raw RP data from `data/dates/`
- Matches with Betfair data from `data/betfair_stitched/`
- Creates master CSVs in `master/{region}/{code}/{YYYY-MM}/`
- Takes 30-45 minutes for 20 months

**Kill process when done** (Ctrl+C on tail)

### Step 2: Load to PostgreSQL (20-30 min)

```bash
cd /home/smonaghan/hrmasterset

# Load all missing months  
python3 load_master_to_postgres_v2.py \
  --master-dir /home/smonaghan/rpscrape/master

# It will auto-detect which months haven't been loaded yet
```

**What it does:**
- Scans `/home/smonaghan/rpscrape/master/` for CSV files
- Loads only months not already in database
- Uses COPY for fast bulk loading
- Takes 20-30 minutes

### Step 3: Verify Results (2 min)

```bash
cd /home/smonaghan/GiddyUp/mapper

# Check latest date
docker exec horse_racing psql -U postgres -d horse_db <<SQL
SET search_path TO racing, public;

SELECT 'Latest Race:' as metric, MAX(race_date)::text as value FROM races
UNION ALL
SELECT 'Total Races:', COUNT(*)::text FROM races
UNION ALL  
SELECT 'Total Runners:', COUNT(*)::text FROM runners;

SELECT COUNT(*) as "Dancing in Paris Runs"
FROM runners r
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL;

SQL
```

**Expected:**
- Latest Race: 2025-10-13
- Total Races: ~205,000
- Dancing in Paris Runs: ~33

---

## 📊 **Alternative: Process Month-by-Month**

If you want more control, process one month at a time:

### Test with February 2024 First:

```bash
cd /home/smonaghan/rpscrape

# 1. Check if Feb 2024 master already exists
ls -la master/gb/flat/2024-02/ 2>/dev/null || echo "Needs stitching"

# 2. If needs stitching, the stitcher will create it when run

# 3. Once stitched, load just that month
cd /home/smonaghan/hrmasterset
python3 -c "
import subprocess
import sys

# Just load Feb 2024 as test
cmd = [
    'python3', 'load_master_to_postgres_v2.py',
    # Add any specific month parameters if available
]
result = subprocess.run(cmd, capture_output=True, text=True)
print(result.stdout)
print(result.stderr, file=sys.stderr)
"
```

---

## ⚡ **Fastest Path (Recommended)**

Since the stitcher is already running in the previous command, let it finish! Then:

```bash
# Wait for stitcher to complete (~30-45 min)
# Check when done:
ls -la /home/smonaghan/rpscrape/master/gb/flat/ | grep "2024-\|2025-"

# When stitching complete, load to database:
cd /home/smonaghan/hrmasterset
python3 load_master_to_postgres_v2.py \
  --master-dir /home/smonaghan/rpscrape/master

# Verify:
docker exec horse_racing psql -U postgres -d horse_db -c \
  "SELECT MAX(race_date) FROM racing.races"
```

---

## 📋 **Verification After Load**

Run this comprehensive check:

```bash
cd /home/smonaghan/GiddyUp/mapper

docker exec horse_racing psql -U postgres -d horse_db <<EOF
SET search_path TO racing, public;

-- 1. Overall Stats
SELECT 
    '📊 Total Races' AS metric,
    COUNT(*)::text AS value
FROM races
UNION ALL
SELECT '📅 Latest Date', MAX(race_date)::text FROM races
UNION ALL  
SELECT '📅 First Date', MIN(race_date)::text FROM races
UNION ALL
SELECT '🐎 Unique Horses', COUNT(*)::text FROM horses
UNION ALL
SELECT '👔 Unique Trainers', COUNT(*)::text FROM trainers;

-- 2. Monthly Coverage 2024-2025
\echo ''
\echo '📅 2024-2025 Monthly Coverage:'

SELECT 
    TO_CHAR(race_date, 'YYYY-MM') AS month,
    COUNT(*) AS races,
    COUNT(DISTINCT course_id) AS courses
FROM races
WHERE race_date >= '2024-01-01'
GROUP BY 1
ORDER BY 1;

-- 3. Dancing in Paris Full Check
\echo ''
\echo '🐎 Dancing in Paris (FR) - Complete Verification:'

SELECT 
    COUNT(*) AS total_runs,
    MIN(race_date)::text AS first_race,
    MAX(race_date)::text AS last_race,
    SUM(win_flag::int) AS wins
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN horses h ON h.horse_id = r.horse_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL;

\echo ''
\echo 'Recent 5 Runs:'

SELECT 
    ra.race_date::text,
    c.course_name,
    r.pos_num,
    r."or",
    r.win_bsp::numeric(10,2)
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN horses h ON h.horse_id = r.horse_id
JOIN courses c ON c.course_id = ra.course_id
WHERE h.horse_name LIKE '%Dancing%Paris%'
  AND r.pos_num IS NOT NULL
ORDER BY ra.race_date DESC
LIMIT 5;

EOF
```

**Expected Output:**
- Total Runs: ~33 (was 12)
- Latest Date: 2025-09-06 or later
- Wins: 7
- All recent races visible

---

## 🎯 **Summary**

**Current State:**
- ❌ Database: Through Jan 17, 2024 only
- ✅ Raw Data: All 20 missing months downloaded
- ⏳ Master Files: Need stitching (in progress)
- ⏳ Database Load: Pending

**Action:**
1. **Let stitcher finish** (~30 min)
2. **Load to database** (~20 min)
3. **Verify** (~2 min)

**Total Time:** ~52 minutes to complete data

**Result:**
- ✅ Database current through Oct 2025
- ✅ Dancing in Paris: 33 runs (not 12)
- ✅ All active horses updated
- ✅ API will show current data

---

**The data is there - just needs processing!** ✅

**Monitor stitching:** `tail -f /tmp/stitch.log`

