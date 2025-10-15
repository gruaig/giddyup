# Starting PostgreSQL Database for GiddyUp

**Last Updated:** October 14, 2025  
**PostgreSQL Version:** 18.0-alpine3.22

---

## Quick Start (Recommended)

### Start PostgreSQL with Optimized Settings

```bash
docker run -d \
  --network=host \
  --name=horse_racing \
  -v horse_racing_data:/var/lib/postgresql/data \
  -e POSTGRES_PASSWORD=password \
  postgres:18.0-alpine3.22 \
  -c shared_buffers=256MB \
  -c work_mem=8MB \
  -c temp_buffers=16MB \
  -c effective_cache_size=1GB \
  -c max_connections=100
```

**One-liner version:**
```bash
docker run -d --network=host --name=horse_racing -v horse_racing_data:/var/lib/postgresql/data -e POSTGRES_PASSWORD=password postgres:18.0-alpine3.22 -c shared_buffers=256MB -c work_mem=8MB -c temp_buffers=16MB -c effective_cache_size=1GB -c max_connections=100
```

---

## Memory Settings Explained

### Why These Settings Matter

**Without these settings:**
- Profile endpoints fail with "No space left on device" errors
- Comment search runs out of memory
- Complex queries timeout
- 15/33 tests fail

**With these settings:**
- All profile endpoints work
- Comment search works
- All queries complete successfully
- 30/33 tests pass (90.9%)

### Configuration Values

| Setting | Value | Purpose |
|---------|-------|---------|
| `shared_buffers` | 256MB | Main memory pool (prevents "no space" errors) |
| `work_mem` | 8MB | Per-query operation memory (was 4MB, too small) |
| `temp_buffers` | 16MB | Temporary table memory |
| `effective_cache_size` | 1GB | Helps query planner make better decisions |
| `max_connections` | 100 | Reasonable connection limit |

---

## Complete Initialization (Clean Database)

### 1. Start PostgreSQL
```bash
docker run -d --network=host --name=horse_racing -v horse_racing_data:/var/lib/postgresql/data -e POSTGRES_PASSWORD=password postgres:18.0-alpine3.22 -c shared_buffers=256MB -c work_mem=8MB -c temp_buffers=16MB -c effective_cache_size=1GB
```

### 2. Wait for Startup
```bash
sleep 5
```

### 3. Create Database
```bash
docker exec horse_racing psql -U postgres -c "CREATE DATABASE horse_db;"
```

### 4. Run Schema Initialization
```bash
docker cp /home/smonaghan/GiddyUp/postgres/init_clean.sql horse_racing:/tmp/
docker exec horse_racing psql -U postgres -d horse_db -f /tmp/init_clean.sql
```

This creates:
- ✅ All tables (races, runners, horses, etc.)
- ✅ All indexes (performance optimized)
- ✅ All materialized views (mv_runner_base, mv_last_next, mv_draw_bias_flat)
- ✅ FTS indexes for search
- ✅ Trigram indexes for fuzzy matching

### 5. Run Additional Migrations
```bash
docker cp /home/smonaghan/GiddyUp/postgres/migrations/001_ingest_tracking.sql horse_racing:/tmp/
docker exec horse_racing psql -U postgres -d horse_db -f /tmp/001_ingest_tracking.sql
```

### 6. Load Data
```bash
cd /home/smonaghan/GiddyUp/scripts
python3 load_master_to_postgres_v2.py
```

### 7. Verify
```bash
docker exec horse_racing psql -U postgres -d horse_db << 'SQL'
SET search_path TO racing;

SELECT 'races' as table, COUNT(*) as count FROM races
UNION ALL
SELECT 'runners', COUNT(*) FROM runners
UNION ALL
SELECT 'horses', COUNT(*) FROM horses
UNION ALL
SELECT 'mv_runner_base', COUNT(*) FROM mv_runner_base
UNION ALL
SELECT 'mv_last_next', COUNT(*) FROM mv_last_next;

-- Check MV sizes
SELECT 
    matviewname,
    pg_size_pretty(pg_relation_size(matviewname::regclass)) AS size
FROM pg_matviews
WHERE schemaname = 'racing';
SQL
```

**Expected Output:**
```
races:          226,136
runners:        2,232,558
horses:         190,892
mv_runner_base: 2,232,558
mv_last_next:   ~1,800,000

mv_runner_base:    391 MB
mv_last_next:      243 MB
mv_draw_bias_flat: 128 KB
```

---

## Restarting Existing Database

### Keep Existing Data
```bash
# Stop container (keeps volume)
docker stop horse_racing

# Remove container (keeps volume)
docker rm horse_racing

# Start with same volume + optimized settings
docker run -d --network=host --name=horse_racing -v horse_racing_data:/var/lib/postgresql/data -e POSTGRES_PASSWORD=password postgres:18.0-alpine3.22 -c shared_buffers=256MB -c work_mem=8MB -c temp_buffers=16MB -c effective_cache_size=1GB
```

**This preserves:**
- ✅ All 226,136 races
- ✅ All runners, horses, trainers
- ✅ All materialized views
- ✅ All indexes

**No data reload needed!**

---

## Troubleshooting

### Problem: "No space left on device" errors

**Cause:** Insufficient shared_buffers or work_mem

**Solution:** Restart Docker with proper memory settings (see Quick Start above)

### Problem: "Function round(double precision, integer) does not exist"

**Cause:** Code not updated with ::numeric casts

**Solution:** Pull latest code from backend-api (October 14, 2025 or later)

### Problem: Profile queries very slow (>1 second)

**Cause:** Code not using mv_runner_base

**Solution:** 
1. Check `backend-api/internal/repository/profile.go`
2. Should use `FROM mv_runner_base rb` not `FROM runners ru JOIN races r`
3. Latest code (Oct 14, 2025) has this optimization

### Problem: Materialized views don't exist

**Cause:** Didn't run production_hardening.sql

**Solution:**
```bash
docker cp /home/smonaghan/GiddyUp/postgres/migrations/production_hardening.sql horse_racing:/tmp/
docker exec horse_racing psql -U postgres -d horse_db -f /tmp/production_hardening.sql
```

---

## Backup & Restore

### Create Backup
```bash
# Full backup
docker exec horse_racing pg_dumpall -U postgres > /tmp/horse_db_backup_$(date +%Y%m%d).sql

# Just data schema
docker exec horse_racing pg_dump -U postgres -d horse_db --schema=racing > /tmp/racing_schema_$(date +%Y%m%d).sql
```

### Restore from Backup
```bash
# Full restore
docker exec -i horse_racing psql -U postgres < /tmp/horse_db_backup_YYYYMMDD.sql

# Schema only
docker exec -i horse_racing psql -U postgres -d horse_db < /tmp/racing_schema_YYYYMMDD.sql
```

---

## Performance Verification

After starting, verify performance:

```bash
# Start backend API
cd /home/smonaghan/GiddyUp/backend-api
./start_server.sh

# Test horse profile (should be <50ms)
time curl -s "http://localhost:8000/api/v1/horses/134020/profile" > /dev/null

# Test comment search (should be <100ms)
time curl -s "http://localhost:8000/api/v1/search/comments?q=led&limit=5" > /dev/null

# Run full test suite
cd backend-api
go test -v ./tests -run "Test[A-G]" 

# Expected: 30/33 passing (90.9%)
```

---

## Docker Commands Reference

### Start
```bash
docker start horse_racing
```

### Stop
```bash
docker stop horse_racing
```

### Remove (keeps volume)
```bash
docker rm horse_racing
```

### Remove with volume (⚠️ DESTROYS DATA)
```bash
docker rm -v horse_racing  # DON'T DO THIS unless you want to delete everything
```

### Check Logs
```bash
docker logs horse_racing
docker logs -f horse_racing  # Follow mode
```

### Connect to Database
```bash
docker exec -it horse_racing psql -U postgres -d horse_db
```

### Check Settings
```bash
docker exec horse_racing psql -U postgres -c "SHOW shared_buffers; SHOW work_mem;"
```

---

## Notes

- Volume name can be custom (replace `horse_racing_data` with your preferred name)
- Network mode `--network=host` makes PostgreSQL available on localhost:5432
- Password is `password` (change for production!)
- All optimizations are in init_clean.sql - run once and you're set

**Everything you need for a clean start is documented here!**

