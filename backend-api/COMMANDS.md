# GiddyUp Backend Commands

Quick reference for all available commands.

---

## 🚀 Server Commands

### Start API Server (Interactive)
```bash
./start_with_logging.sh
```
- Sources `settings.env` automatically
- Outputs to **both** console and `logs/server.log`
- Best for development/debugging

### Start API Server (Background)
```bash
./start_server.sh
```
- Sources `settings.env` automatically  
- Runs in background
- Outputs to `logs/server.log` only
- Returns PID for easy stopping

---

## 📥 Data Fetching Commands

### Fetch Single Date
```bash
./fetch_all 2024-10-15
./fetch_all --date 2024-10-15 --force    # Force refresh
```
**What it does:**
1. Fetches Sporting Life data (jockey, trainer, owner, odds, Betfair selectionId)
2. Loads Betfair CSV stitched data (BSP, PPWAP, etc.)
3. Matches and merges both sources
4. Upserts to database

**Use cases:**
- Backfill specific dates
- Re-fetch corrected data
- One-off historical loads

### Backfill Date Range
```bash
./bin/backfill_dates --start-date 2024-01-01 --end-date 2024-12-31
```
**What it does:**
- Detects missing dates in database
- Fetches and inserts only missing data
- Bulk backfill operation

**Use cases:**
- Fill gaps in historical data
- Initial database population
- Recover from data loss

---

## fetch_all_betfair

**Standalone command to fetch LIVE Betfair prices for a specific date using API-NG**

```bash
./fetch_all_betfair <date>

# Examples
./fetch_all_betfair 2025-10-16
./fetch_all_betfair $(date +%Y-%m-%d)  # today
```

**Prerequisites:**
- Race data must exist (run `fetch_all` first)
- Betfair credentials in environment (`settings.env`)
- Only works for active races (before off_time + 30 minutes)

**What it does:**
1. Loads races from database for specified date
2. Discovers Betfair WIN markets using API-NG
3. Matches races to markets (by course, time, region)
4. Fetches live prices in batches (back, lay, VWAP)
5. Inserts into `racing.live_prices` table
6. Mirrors latest prices to `racing.runners` table

**Uses same technology as:**
- Automatic live price service (`internal/services/liveprices.go`)
- Same API calls, same matching logic, same database schema
- Runs once on-demand instead of continuously every 60 seconds

**Use cases:**
- Manual price refresh for a specific date
- Backfill live prices outside auto-update schedule
- Testing Betfair integration without running full server
- Debugging market matching

See `cmd/fetch_all_betfair/README.md` for detailed documentation.

---

## 🔧 Utility Commands

### Build All Binaries
```bash
# API server
go build -o bin/api cmd/api/main.go

# fetch_all
go build -o bin/fetch_all cmd/fetch_all/main.go

# fetch_all_betfair
go build -o bin/fetch_all_betfair cmd/fetch_all_betfair/main.go

# backfill_dates
go build -o bin/backfill_dates cmd/backfill_dates/main.go
```

### Check Missing Data
```bash
./bin/check_missing --start 2024-01-01 --end 2024-12-31
```
Shows which dates are missing in the database.

---

## 📊 Common Recipes

### Fetch Yesterday's Data
```bash
./fetch_all $(date -d "yesterday" +%Y-%m-%d)
```

### Fetch Last 7 Days
```bash
for i in {0..6}; do
  DATE=$(date -d "$i days ago" +%Y-%m-%d)
  echo "Fetching $DATE..."
  ./fetch_all $DATE
  sleep 2
done
```

### Fetch Entire Month (October 2024)
```bash
for day in {01..31}; do
  ./fetch_all 2024-10-$day || true
  sleep 2
done
```

### Force Refresh Today's Data
```bash
./fetch_all $(date +%Y-%m-%d) --force
```

---

## 🧪 Testing Commands

### Test API Health
```bash
curl http://localhost:8000/health
```

### Get Today's Races
```bash
curl http://localhost:8000/api/v1/races/today | jq .
```

### Get Specific Horse Profile
```bash
curl http://localhost:8000/api/v1/horses/123456/profile | jq .
```

### Search Horses
```bash
curl "http://localhost:8000/api/v1/search/horses?q=Enable" | jq .
```

---

## 🗄️ Database Commands

### Connect to Database
```bash
psql -U postgres -d horse_db
```

### Check Data for Date
```sql
SELECT COUNT(*) FROM racing.races WHERE race_date = '2024-10-15';
SELECT COUNT(*) FROM racing.runners WHERE race_date = '2024-10-15';
```

### View Recent Races
```sql
SELECT race_date, COUNT(*) as races, SUM(ran) as runners
FROM racing.races
GROUP BY race_date
ORDER BY race_date DESC
LIMIT 10;
```

### Check Betfair Selection IDs
```sql
SELECT 
  r.race_name,
  ru.horse_name,
  ru.betfair_selection_id,
  ru.best_odds
FROM racing.races r
JOIN racing.runners ru ON ru.race_id = r.race_id
WHERE r.race_date = '2024-10-15'
  AND ru.betfair_selection_id IS NOT NULL
LIMIT 10;
```

---

## 🛑 Stop Commands

### Stop API Server
```bash
# If you have the PID
kill <PID>

# Or find and kill
pkill -f "bin/api"

# Or use lsof
lsof -ti :8000 | xargs kill -9
```

### Clear Logs
```bash
rm -f logs/server.log
rm -f logs/*.log
```

---

## 📦 Environment Variables

All commands respect these environment variables (from `settings.env`):

### Database
- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_NAME` - Database name (default: horse_db)
- `DB_USER` - Database user (default: postgres)
- `DB_PASSWORD` - Database password

### Paths
- `DATA_DIR` - Data directory (default: /home/smonaghan/GiddyUp/data)
- `LOG_DIR` - Log directory (default: logs/)

### API
- `PORT` - API server port (default: 8000)
- `LOG_LEVEL` - Logging level (default: info)

### Betfair
- `BETFAIR_APP_KEY` - Betfair API application key
- `BETFAIR_SESSION_TOKEN` - Betfair session token
- `ENABLE_LIVE_PRICES` - Enable live price updates (default: false)

---

## 🔗 Related Documentation

- [01_DEVELOPER_GUIDE.md](../docs/01_DEVELOPER_GUIDE.md) - Development setup
- [02_API_DOCUMENTATION.md](../docs/02_API_DOCUMENTATION.md) - API endpoints
- [06_SPORTING_LIFE_API.md](../docs/06_SPORTING_LIFE_API.md) - Data source
- [cmd/fetch_all/README.md](cmd/fetch_all/README.md) - fetch_all details
- [cmd/backfill_dates/README.md](cmd/backfill_dates/README.md) - backfill details

---

**Last Updated**: October 16, 2025

