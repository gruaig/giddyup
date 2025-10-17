# 10. Troubleshooting Guide

**Common issues and solutions for GiddyUp**

Last Updated: October 16, 2025

---

## 🚨 Common Issues

### 1. API Won't Start

**Symptom:** `./bin/api` fails or exits immediately

**Causes & Solutions:**

```bash
# Issue: Port 8000 already in use
lsof -ti :8000          # Find process
pkill -f "bin/api"      # Kill existing server

# Issue: Missing environment variables
source ../settings.env  # Load env vars
env | grep DB_          # Verify DB settings

# Issue: Database connection failed
docker ps | grep horse_racing  # Check if DB is running
docker logs horse_racing       # Check DB logs

# Issue: Binary not built
cd backend-api
go build -o bin/api cmd/api/main.go
```

**Expected healthy output:**
```
2025/10/16 14:30:00 Server starting on :8000
2025/10/16 14:30:01 Auto-update: fetching today...
2025/10/16 14:30:05 Loaded 53 races for 2025-10-16
```

---

### 2. "Unknown Course" in UI

**Symptom:** Course names showing as "Unknown Course" or NULL

**Root Cause:** Orphaned course_ids (races with course_ids not in `racing.courses`)

**Fix Applied:** See [07_COURSE_FIX_COMPLETE.md](07_COURSE_FIX_COMPLETE.md)

**Verify Fix:**
```sql
-- Check for orphaned courses
SELECT COUNT(*) FROM racing.races 
WHERE course_id NOT IN (SELECT course_id FROM racing.courses);

-- Should return 0
```

**If still broken:**
```bash
# Re-apply course fix
cd /home/smonaghan/GiddyUp
psql -U postgres -d horse_db < backend-api/COMPLETE_COURSE_REMAP.sql
```

---

### 3. Missing Horse Names / NULL Data

**Symptom:** Runners showing NULL for horse_name, jockey_name, etc.

**Root Cause:** Foreign key IDs don't match dimension tables

**Check:**
```sql
-- Check orphaned horse_ids
SELECT COUNT(*) FROM racing.runners 
WHERE horse_id IS NOT NULL 
AND horse_id NOT IN (SELECT horse_id FROM racing.horses);

-- Check orphaned trainer_ids
SELECT COUNT(*) FROM racing.runners 
WHERE trainer_id IS NOT NULL 
AND trainer_id NOT IN (SELECT trainer_id FROM racing.trainers);
```

**Fix:**
```bash
# Re-fetch the affected date
cd backend-api
./fetch_all 2025-10-16  # Uses slower but accurate upsert
```

---

### 4. No Recent Rolling Form Data

**Symptom:** Trainer/jockey profiles show "Incomplete data" or all zeros

**Root Cause:** No recent races (last 90 days) for that trainer/jockey

**This is EXPECTED behavior!** The trainer is inactive.

**Verify:**
```sql
-- Check trainer's last race
SELECT MAX(r.race_date) as last_race
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.trainer_id = 728;

-- If > 90 days ago, rolling_form will be 0 (correct!)
```

**UI Fix:** Update frontend to show historical stats even when rolling_form is zero.

---

### 5. Live Prices Not Updating

**Symptom:** Prices stuck or showing "SP"

**Check 1: Betfair Credentials**
```bash
echo $BETFAIR_APP_KEY
echo $BETFAIR_SESSION_TOKEN

# Both should have values
```

**Check 2: Live Prices Enabled**
```bash
echo $ENABLE_LIVE_PRICES  # Should be 'true'
```

**Check 3: Selection IDs Present**
```sql
SELECT COUNT(*) FROM racing.runners 
WHERE betfair_selection_id IS NOT NULL
AND race_id IN (
    SELECT race_id FROM racing.races WHERE race_date = CURRENT_DATE
);

-- Should be > 0 for today's races
```

**Check 4: Betfair API Status**
```bash
# Test API connectivity
curl -H "X-Application: $BETFAIR_APP_KEY" \
     -H "X-Authentication: $BETFAIR_SESSION_TOKEN" \
     https://api.betfair.com/exchange/betting/json-rpc/v1 \
     -d '{"jsonrpc":"2.0","method":"SportsAPING/v1.0/listEventTypes","id":1}'

# Should return event types list
```

---

### 6. Duplicate Races

**Symptom:** Same race appearing multiple times in database/UI

**Root Cause:** Race key generation not unique OR delete-before-insert failed

**Check:**
```sql
-- Find duplicates
SELECT race_date, course_id, off_time, COUNT(*) as dupes
FROM racing.races
GROUP BY race_date, course_id, off_time
HAVING COUNT(*) > 1;
```

**Fix:**
```sql
-- Delete duplicates (keep oldest)
DELETE FROM racing.races r1
WHERE EXISTS (
    SELECT 1 FROM racing.races r2
    WHERE r2.race_date = r1.race_date
    AND r2.course_id = r1.course_id
    AND r2.off_time = r1.off_time
    AND r2.race_id < r1.race_id  -- Keep the older one
);
```

**Prevention:** Fixed in `autoupdate.go` line 189 (SQL syntax bug removed).

---

### 7. Draw Bias Returns Empty

**Symptom:** `/api/v1/bias/draw` returns `[]`

**Causes:**

**A. Insufficient Data**
```sql
-- Need at least min_runners races
SELECT COUNT(*) FROM racing.races 
WHERE course_id = 16591 
AND race_type = 'Flat' 
AND ran >= 10;  -- Default min_runners

-- If < 100, increase date range or lower min_runners
```

**B. Wrong Course ID**
```bash
# Get valid course IDs
curl http://localhost:8000/api/v1/courses | jq '.[] | {id: .course_id, name: .course_name}'
```

**C. No Draw Data**
```sql
-- Check if draws are populated
SELECT COUNT(*) FROM racing.runners 
WHERE draw IS NOT NULL;

-- Should be > 0 for flat races
```

---

### 8. Auto-Update Not Running

**Symptom:** Today/tomorrow races not loading automatically

**Check Logs:**
```bash
grep "Auto-update" backend-api/logs/server.log | tail -20
```

**Expected:**
```
Auto-update: starting
Auto-update: fetching today (2025-10-16)
Auto-update: fetching tomorrow (2025-10-17)
Auto-update: loaded 53 races for today
Auto-update: loaded 47 races for tomorrow
Auto-update: complete (duration: 8.3s)
```

**If not running:**
```go
// Check internal/services/autoupdate.go
// Ensure StartAutoUpdate() is called in cmd/api/main.go
```

---

### 9. Database Restore Fails

**Symptom:** `psql < db_backup.sql` errors

**Common Issues:**

**A. Database doesn't exist**
```bash
# Create database first
docker exec -i horse_racing psql -U postgres << SQL
DROP DATABASE IF EXISTS horse_db;
CREATE DATABASE horse_db;
SQL

# Then restore
cat db_backup.sql | docker exec -i horse_racing psql -U postgres -d horse_db
```

**B. Schema conflicts**
```bash
# Drop and recreate schema
docker exec horse_racing psql -U postgres -d horse_db -c "DROP SCHEMA IF EXISTS racing CASCADE;"

# Then restore
```

**C. Encoding issues**
```bash
# Specify encoding
docker exec -i horse_racing psql -U postgres -d horse_db \
  -c "SET client_encoding = 'UTF8';" < db_backup.sql
```

---

### 10. High Memory Usage

**Symptom:** API using > 2GB RAM

**Causes:**

**A. Large result sets**
```go
// Add LIMIT to queries
query := `SELECT * FROM racing.runners LIMIT 10000`
```

**B. Missing indexes**
```sql
-- Check missing indexes
SELECT tablename, indexname FROM pg_indexes 
WHERE schemaname = 'racing';

-- Create if needed
CREATE INDEX idx_runners_race_id ON racing.runners(race_id);
```

**C. Unbounded cache**
```go
// Set TTL on caches
cache.SetTTL(5 * time.Minute)
```

---

## 🔍 Debugging Techniques

### Enable Debug Logging

```go
// In cmd/api/main.go
logger.SetLevel(logger.DEBUG)

// Now logs will show:
// DEBUG: GetHorseProfile: horse_id=123456
// DEBUG: Query took 45ms
```

### SQL Query Profiling

```sql
-- Enable timing
\timing

-- Explain query plan
EXPLAIN ANALYZE 
SELECT * FROM racing.runners WHERE race_id = 123456;

-- Look for:
-- - Seq Scan (bad - add index)
-- - Index Scan (good)
-- - Execution time < 100ms
```

### API Request Tracing

```bash
# Tail logs with grep filter
tail -f backend-api/logs/server.log | grep "GET /api/v1/races"

# Output:
# GET /api/v1/races/123456 200 45ms
# GET /api/v1/races/today 200 234ms
```

---

## 📊 Health Checks

### Database Health

```sql
-- Check table sizes
SELECT 
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'racing'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### API Health

```bash
# Health endpoint
curl http://localhost:8000/health

# Expected:
{"status":"healthy","database":"connected","version":"1.0.0"}
```

### Data Completeness

```sql
-- Check recent data
SELECT 
    race_date,
    COUNT(DISTINCT course_id) as courses,
    COUNT(DISTINCT race_id) as races,
    COUNT(*) as runners
FROM racing.races r
LEFT JOIN racing.runners ru ON r.race_id = ru.race_id
WHERE race_date >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY race_date
ORDER BY race_date DESC;
```

---

## 🛠️ Maintenance Tasks

### Weekly Vacuum

```sql
-- Reclaim space
VACUUM ANALYZE racing.races;
VACUUM ANALYZE racing.runners;
```

### Monthly Backup

```bash
# Create backup
./BACKUP_DATABASE.sh

# Verify backup size
ls -lh db_backup_*.sql

# Should be 900MB+ for full dataset
```

### Clear Old Logs

```bash
# Keep last 7 days
find backend-api/logs -name "*.log" -mtime +7 -delete
```

---

## 📞 Getting Help

### Check Documentation
1. [00_START_HERE.md](00_START_HERE.md) - Overview
2. [02_API_DOCUMENTATION.md](02_API_DOCUMENTATION.md) - API reference
3. [03_DATABASE_GUIDE.md](03_DATABASE_GUIDE.md) - Database schema

### Search Archive
```bash
# Search historical docs
grep -r "your issue" docs/archive/
```

### Database Queries

```sql
-- Most common issues have been documented
-- Check racing schema for hints
\d racing.races
\d racing.runners
```

---

**Last Updated:** October 16, 2025  
**Status:** ✅ Complete  
**Coverage:** Top 10 issues + debugging techniques


