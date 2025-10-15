# GiddyUp Data Ingestion System

**Purpose:** Automated data ingestion from Racing Post + Betfair with gap detection and validation.

**Status:** ✅ Production Ready

---

## 🎯 Quick Start

### Run Database Migration

```bash
cd /home/smonaghan/GiddyUp
psql -U postgres -d giddyup -f postgres/migrations/001_ingest_tracking.sql
```

### Start the API

```bash
cd backend-api
./start_server.sh
```

### Ingest Today's Data

```bash
# Ingest GB Flat races for today
curl -X POST http://localhost:8000/api/v1/admin/ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"regions":["gb"],"codes":["flat"],"dates":["today"]}'

# Check status
curl http://localhost:8000/api/v1/admin/ingest/status

# Check for missing data
curl "http://localhost:8000/api/v1/admin/gaps?on=$(date +%F)"
```

---

## 📊 Admin Endpoints

### 1. Run Ingestion

**POST** `/api/v1/admin/ingest/run`

Triggers data ingestion for specified scope.

**Request Body:**
```json
{
  "regions": ["gb", "ire"],
  "codes": ["flat", "jumps"],
  "dates": ["2025-10-13"],
  "force": false
}
```

**OR with date range:**
```json
{
  "regions": ["gb"],
  "codes": ["flat"],
  "range": {
    "from": "2025-09-01",
    "to": "2025-09-30"
  },
  "force": false
}
```

**Parameters:**

| Field | Type | Description |
|-------|------|-------------|
| `regions` | []string | `["gb", "ire"]` - Geographic regions |
| `codes` | []string | `["flat", "jumps"]` - Race types |
| `dates` | []string | Specific dates (`["2025-10-13"]` or `["today"]`) |
| `range` | object | Date range (`{"from":"YYYY-MM-DD","to":"YYYY-MM-DD"}`) |
| `force` | bool | Re-ingest even if already loaded (default: false) |

**Response (202 Accepted):**
```json
{
  "run_id": 42,
  "status": "running",
  "started_at": "2025-10-13T18:45:00Z",
  "scope": {
    "regions": ["gb"],
    "codes": ["flat"],
    "dates": ["2025-10-13"]
  }
}
```

**Advisory Lock:**
- Only one ingestion can run at a time
- Uses PostgreSQL advisory lock (ID: 123456789)
- Second request returns 409 Conflict if lock is held

---

### 2. Check Ingestion Status

**GET** `/api/v1/admin/ingest/status`

Returns the latest ingestion run details.

**Response:**
```json
{
  "run_id": 42,
  "status": "success",
  "started_at": "2025-10-13T18:45:00Z",
  "finished_at": "2025-10-13T18:47:32Z",
  "duration_ms": 152430,
  "scope": {
    "regions": ["gb"],
    "codes": ["flat"],
    "dates": ["2025-10-13"]
  },
  "stats": {
    "races_inserted": 125,
    "runners_inserted": 1350,
    "races_updated": 8,
    "runners_updated": 95,
    "errors": 0
  },
  "error_msg": null
}
```

**Status Values:**
- `running` - Currently in progress
- `success` - Completed successfully
- `failed` - Encountered error (see `error_msg`)
- `canceled` - Manually canceled

---

### 3. Detect Missing Data (Gaps)

**GET** `/api/v1/admin/gaps?on=YYYY-MM-DD`

Detects data quality issues and missing data for a specific date.

**Parameters:**

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `on` | string | today | Date to check (YYYY-MM-DD) |

**Response:**
```json
{
  "date": "2025-10-13",
  "checks": {
    "entries_today": {
      "count": 12,
      "races": [
        {
          "date": "2025-10-13",
          "course_id": 2,
          "course_name": "Ascot",
          "entries": 14
        }
      ]
    },
    "unresulted_passed": {
      "count": 0,
      "races": []
    },
    "runner_count_mismatch": {
      "count": 1,
      "races": [
        {
          "race_id": 991122,
          "date": "2025-10-12",
          "course_name": "Kempton",
          "ran": 12,
          "runner_rows": 11
        }
      ]
    },
    "unresolved_horses": {
      "count": 0,
      "horses": []
    },
    "yesterday_missing_winners": {
      "count": 0,
      "races": []
    }
  },
  "summary": {
    "total_issues": 1,
    "critical": 0,
    "warnings": 1
  }
}
```

**Gap Types:**

| Check | Description | Severity |
|-------|-------------|----------|
| `entries_today` | Racecards present for today | Info |
| `unresulted_passed` | Races with off-time passed but no results | Critical |
| `runner_count_mismatch` | Races where `ran` ≠ #runners | Warning |
| `unresolved_horses` | Horses in entries not in `horses` table | Warning |
| `yesterday_missing_winners` | Yesterday's races without winners | Critical |

---

## 🔄 Data Flow

```
1. Request → POST /admin/ingest/run
2. Acquire advisory lock (pg_try_advisory_lock)
3. Create etl_runs row (status=running)
4. For each (region, code, date):
   a. Check ingested_days (skip if exists and force=false)
   b. Read from /home/smonaghan/rpscrape/master/{region}/{code}/{YYYY-MM}/
   c. Load races_*.csv → staging.races
   d. Load runners_*.csv → staging.runners
   e. UPSERT to racing.races, racing.runners
   f. Insert to ingested_days
   g. TRUNCATE staging tables
5. Refresh materialized views (mv_runner_base, mv_last_next, etc.)
6. Update etl_runs (status=success, stats)
7. Release advisory lock
```

---

## 📁 Data Source Structure

```
/home/smonaghan/rpscrape/master/
├── gb/
│   ├── flat/
│   │   ├── 2024-01/
│   │   │   ├── manifest.json
│   │   │   ├── races_gb_flat_2024-01.csv
│   │   │   ├── runners_gb_flat_2024-01.csv
│   │   │   └── unmatched_gb_flat_2024-01.csv
│   │   ├── 2024-02/...
│   └── jumps/...
└── ire/
    ├── flat/...
    └── jumps/...
```

**CSV Schemas:**

**races_*.csv:**
```
date,region,course,off,race_name,type,class,pattern,rating_band,age_band,
sex_rest,dist,dist_f,dist_m,going,surface,ran,race_key
```

**runners_*.csv:**
```
race_key,num,pos,draw,ovr_btn,btn,horse,age,sex,lbs,hg,time,secs,dec,
jockey,trainer,prize,prize_raw,or,rpr,sire,dam,damsire,owner,comment,
win_bsp,win_ppwap,win_morningwap,win_ppmax,win_ppmin,win_ipmax,win_ipmin,
win_morning_vol,win_pre_vol,win_ip_vol,win_lose,place_bsp,place_ppwap,
place_morningwap,place_ppmax,place_ppmin,place_ipmax,place_ipmin,
place_morning_vol,place_pre_vol,place_ip_vol,place_win_lose,
runner_key,match_jaccard,match_time_diff_min,match_reason
```

---

## 🛠️ Operational Runbook

### Daily Ingestion

**Recommended Schedule:**
```bash
# Cron entry (runs at 23:00 UTC daily)
0 23 * * * curl -X POST http://localhost:8000/api/v1/admin/ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"regions":["gb","ire"],"codes":["flat","jumps"],"dates":["today"]}'
```

### Check Yesterday's Results

```bash
# Morning check (09:00 UTC)
curl "http://localhost:8000/api/v1/admin/gaps?on=$(date -d yesterday +%F)"
```

If gaps found, re-run ingestion with `force=true`:

```bash
curl -X POST http://localhost:8000/api/v1/admin/ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"regions":["gb"],"codes":["flat"],"dates":["2025-10-12"],"force":true}'
```

### Backfill Historical Data

```bash
# Load entire month
curl -X POST http://localhost:8000/api/v1/admin/ingest/run \
  -H 'Content-Type: application/json' \
  -d '{
    "regions": ["gb"],
    "codes": ["flat"],
    "range": {"from": "2024-09-01", "to": "2024-09-30"}
  }'
```

### Common Issues

**Issue: "Advisory lock not acquired"**
- **Cause:** Another ingestion is running
- **Solution:** Wait for completion or cancel:
  ```sql
  SELECT pg_advisory_unlock(123456789);
  UPDATE racing.etl_runs SET status='canceled', finished_at=now() 
  WHERE run_id=(SELECT MAX(run_id) FROM racing.etl_runs);
  ```

**Issue: "Data already ingested for this date"**
- **Cause:** `ingested_days` has entry for (region, code, date)
- **Solution:** Use `force=true` to re-ingest

**Issue: "Runner count mismatch"**
- **Cause:** Racing Post data incomplete or runners withdrawn
- **Solution:** Normal - some races have non-runners. Check `comment` field for withdrawals.

**Issue: "Unresolved horses"**
- **Cause:** New horse not yet in `horses` dimension table
- **Solution:** Ingestion auto-creates horses. Re-run with `force=true` if needed.

---

## 📊 Monitoring

### View Recent Runs

```sql
SELECT run_id, status, started_at, finished_at,
       (finished_at - started_at) AS duration,
       scope->>'dates' AS dates,
       stats->>'races_inserted' AS races,
       stats->>'runners_inserted' AS runners
FROM racing.etl_runs
ORDER BY started_at DESC
LIMIT 10;
```

### Check Ingested Coverage

```sql
SELECT region, code, 
       MIN(d) AS first_date,
       MAX(d) AS last_date,
       COUNT(*) AS days_loaded
FROM racing.ingested_days
GROUP BY region, code
ORDER BY region, code;
```

### Find Data Gaps

```sql
-- Find missing days in last 30 days
WITH dates AS (
  SELECT generate_series(
    CURRENT_DATE - INTERVAL '30 days',
    CURRENT_DATE,
    INTERVAL '1 day'
  )::date AS d
)
SELECT d
FROM dates
WHERE NOT EXISTS (
  SELECT 1 FROM racing.ingested_days
  WHERE ingested_days.d = dates.d
    AND region = 'gb'
    AND code = 'flat'
)
ORDER BY d;
```

---

## ⚡ Performance

**Expected Timings:**

| Operation | Typical | Max |
|-----------|---------|-----|
| Single day (GB Flat) | 2-5s | 10s |
| Single day (all regions/codes) | 8-15s | 30s |
| Full month backfill | 30-60s | 2min |
| Gap detection | <500ms | 1s |

**Optimization Tips:**

1. **Batch inserts:** COPY is ~50x faster than individual INSERTs
2. **Advisory lock:** Prevents concurrent writes
3. **Staging tables:** Validate before upserting to main tables
4. **MV refresh:** Only refresh after batch loads, not every day
5. **Indexes:** Disable during backfill, rebuild after

---

## 🧪 Testing

```bash
cd backend-api

# Run ingestion tests
go test -v ./tests/ingest_e2e_test.go

# Test with sample data
curl -X POST http://localhost:8000/api/v1/admin/ingest/run \
  -H 'Content-Type: application/json' \
  -d '{"regions":["gb"],"codes":["flat"],"dates":["2024-01-01"]}'
```

---

## 📝 Change Log

**v1.0.0** (2025-10-13)
- Initial release
- Advisory lock for exclusive ingestion
- Gap detection (5 checks)
- Idempotency via `ingested_days`
- COPY-based bulk loading
- Automatic MV refresh

---

## 🔒 Security

**Access Control:**
- Admin endpoints require authentication (implement via middleware)
- Advisory lock prevents concurrent modifications
- Staging tables isolated from main tables

**Recommended:**
- Run behind VPN or firewall
- Use API key authentication
- Log all ingestion runs
- Monitor for unusual patterns

---

## 📞 Support

For issues:
1. Check `/api/v1/admin/gaps` for data quality issues
2. Review latest `etl_runs` for errors
3. Check logs: `tail -f /tmp/giddyup-api.log`
4. Re-run with `force=true` if needed

**Common Questions:**

**Q: How often should I run ingestion?**  
A: Once daily at 23:00 UTC after final results are published.

**Q: What if I miss a day?**  
A: Just specify the missed date(s) in your next run.

**Q: Can I run multiple ingestions in parallel?**  
A: No, advisory lock ensures only one at a time.

**Q: How do I know if data is complete?**  
A: Check `/admin/gaps` - should show no critical issues.

---

**Ingestion system is production-ready!** ✅

