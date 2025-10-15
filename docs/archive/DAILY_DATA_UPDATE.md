# Daily Data Update - Quick Reference

**Status:** ✅ Pipeline is production-ready  
**Last Updated:** October 14, 2025

---

## 🔄 **DAILY UPDATE PROCESS**

### Step 1: Scrape Yesterday's Data
```bash
cd /home/smonaghan/rpscrape
python3 scrape_racing_post_by_month.py
# This scrapes the most recent data
```

### Step 2: Ensure Betfair Classification is Correct (Optional)
```bash
# Only run if you've added new betfair data
python3 reclassify_betfair_stitched.py
```

### Step 3: Stitch Racing Post + Betfair
```bash
# Configure fixed_stitcher_2024_2025.py for the date range you want
python3 fixed_stitcher_2024_2025.py
# Outputs to: /home/smonaghan/rpscrape/master/
```

### Step 4: Load to Database
```bash
cd /home/smonaghan/GiddyUp/scripts
python3 load_master_to_postgres_v2.py
# The loader automatically detects new months
# Uses DO NOTHING to skip existing data
```

### Step 5: Verify (Optional)
```bash
python3 verify_data_completeness.py
# Checks for duplicates and missing data
```

---

## ⚡ **QUICK COMMANDS**

### Check Database State
```bash
python3 -c "
import psycopg2
conn = psycopg2.connect(host='localhost', database='horse_db', user='postgres', password='password')
cur = conn.cursor()
cur.execute('SET search_path TO racing')
cur.execute('SELECT COUNT(*) FROM races')
print(f'Total races: {cur.fetchone()[0]:,}')
cur.execute('SELECT MAX(race_date) FROM races')
print(f'Latest date: {cur.fetchone()[0]}')
"
```

### Check for Duplicates
```bash
python3 -c "
import psycopg2
conn = psycopg2.connect(host='localhost', database='horse_db', user='postgres', password='password')
cur = conn.cursor()
cur.execute('SET search_path TO racing')
cur.execute('SELECT COUNT(*) FROM (SELECT race_key, COUNT(*) FROM races GROUP BY race_key HAVING COUNT(*) > 1) d')
print(f'Duplicate race_keys: {cur.fetchone()[0]}')
"
```

### Search for a Horse
```bash
python3 -c "
import psycopg2
horse_name = 'Dancing in Paris'
conn = psycopg2.connect(host='localhost', database='horse_db', user='postgres', password='password')
cur = conn.cursor()
cur.execute('SET search_path TO racing')
cur.execute('SELECT COUNT(*) FROM runners r JOIN horses h ON h.horse_id = r.horse_id WHERE LOWER(h.horse_name) LIKE %s', (f'%{horse_name.lower()}%',))
print(f'{horse_name}: {cur.fetchone()[0]} runs')
"
```

---

## 🛡️ **DATA PROTECTION**

### Key Principles
1. **Never modify source data** (raw scrapes)
2. **Always use DO NOTHING** in loaders (idempotent)
3. **Verify before committing** transactions
4. **Keep backups** of master files

### If Something Goes Wrong
```bash
# 1. Check for duplicates
cd /home/smonaghan/GiddyUp/scripts
python3 verify_data_completeness.py

# 2. If duplicates found, rollback to specific date
psql -U postgres -d horse_db
DELETE FROM racing.races WHERE race_date >= 'YYYY-MM-DD';

# 3. Re-run pipeline from clean state
```

---

## 🔧 **KEY FILES**

### Pipeline Components
```
Reclassifier:  /home/smonaghan/rpscrape/reclassify_betfair_stitched.py
Stitcher:      /home/smonaghan/rpscrape/fixed_stitcher_2024_2025.py
Loader:        /home/smonaghan/GiddyUp/scripts/load_master_to_postgres_v2.py
Verifier:      /home/smonaghan/GiddyUp/scripts/verify_data_completeness.py
```

### Data Locations
```
Raw RP:        /home/smonaghan/rpscrape/data/dates/
Raw Betfair:   /home/smonaghan/rpscrape/betfair_directory/
BF Stitched:   /home/smonaghan/rpscrape/data/betfair_stitched/
Master:        /home/smonaghan/rpscrape/master/
Database:      horse_db (PostgreSQL)
```

---

## 📈 **MONITORING**

### What to Check Daily
1. **No duplicates** (should always be 0)
2. **Latest date** matches yesterday
3. **Runner counts** match expected numbers
4. **No BSP constraint violations**

### Red Flags
- ❌ Duplicate race_keys found
- ❌ Missing months in coverage
- ❌ BSP constraint violations
- ❌ Large discrepancies between master and DB

### Green Flags
- ✅ All months present
- ✅ Zero duplicates
- ✅ Master files match database
- ✅ Verification passes

---

## ✅ **PIPELINE STATUS: PRODUCTION READY**

**Your data is complete, clean, and ready to serve!** 🚀

