# Backend API - Command-Line Tools

This directory contains Go-based command-line tools for managing the GiddyUp racing database.

## Available Tools

### 1. API Server (`api`)
Main REST API server for querying racing data.

**Build**:
```bash
go build -o bin/api ./cmd/api/
```

**Run**:
```bash
# Basic
./bin/api

# With auto-update enabled
AUTO_UPDATE_ON_STARTUP=true ./bin/api

# With custom data directory
AUTO_UPDATE_ON_STARTUP=true DATA_DIR=/path/to/data ./bin/api
```

**Environment Variables**:
- `PORT` - Server port (default: 8000)
- `DATABASE_URL` - PostgreSQL connection string
- `AUTO_UPDATE_ON_STARTUP` - Enable auto-backfill (true/false)
- `DATA_DIR` - Data cache directory (default: `/home/smonaghan/GiddyUp/data`)

---

### 2. Load Master (`load_master`)
Efficient bulk loader for historical master CSV data (20 years in ~45 minutes).

**Build**:
```bash
go build -o bin/load_master ./cmd/load_master/
```

**Usage**:
```bash
./bin/load_master \
  -dsn "host=localhost port=5432 user=postgres password=password dbname=horse_db sslmode=disable" \
  -master-dir /path/to/master/data \
  -v
```

**Flags**:
- `-dsn` - PostgreSQL connection string
- `-master-dir` - Path to master CSV directory (default: `/home/smonaghan/GiddyUp/data/master`)
- `-region` - Filter by region: `gb`, `ire`, or `*` (default: `*`)
- `-type` - Filter by type: `flat`, `jumps`, or `*` (default: `*`)
- `-month` - Filter by month: `2024-01`, or `*` (default: `*`)
- `-limit` - Limit number of months to load (default: 0 = all)
- `-make-parts` - Create yearly partitions for races/runners (default: false)
- `-fix-ran` - Auto-fix races.ran to match computed starters (default: false)
- `-v` - Verbose output
- `-timeout` - Transaction timeout in minutes (default: 10)

**Examples**:
```bash
# Load all data
./bin/load_master -v

# Load only GB flat racing for 2024
./bin/load_master -region gb -type flat -month "2024-*" -v

# Load and auto-fix 'ran' discrepancies
./bin/load_master -fix-ran -v

# Load first 12 months only (testing)
./bin/load_master -limit 12 -v
```

---

### 3. Backfill Dates (`backfill_dates`)
Backfills specific date ranges by scraping Racing Post and fetching Betfair data.

**Build**:
```bash
go build -o bin/backfill_dates ./cmd/backfill_dates/
```

**Usage**:
```bash
./bin/backfill_dates \
  -since 2025-10-01 \
  -until 2025-10-14 \
  -data-dir /home/smonaghan/GiddyUp/data
```

**Flags**:
- `-since` - Start date (YYYY-MM-DD, required)
- `-until` - End date (YYYY-MM-DD, default: yesterday)
- `-dry-run` - Don't insert to database, just scrape (default: true)
- `-data-dir` - Data cache directory (default: `/home/smonaghan/GiddyUp/data`)
- `-db` - PostgreSQL connection string
- `-v` - Verbose output

**Examples**:
```bash
# Dry run (scrape only, no database insert)
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14 -dry-run

# Actually insert to database
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14 -dry-run=false

# Backfill yesterday
./bin/backfill_dates -since $(date -d "yesterday" +%Y-%m-%d) -dry-run=false
```

**Warning**: Includes rate limiting (5-8s between races, 15-30s between dates) to avoid being blocked by Racing Post.

---

### 4. Check Missing (`check_missing`)
Detects gaps between expected Betfair data and what's loaded in the database.

**Build**:
```bash
go build -o bin/check_missing ./cmd/check_missing/
```

**Usage**:
```bash
./bin/check_missing \
  -since 2006-01-01 \
  -until 2025-10-14 \
  -betfair-root /home/smonaghan/GiddyUp/data/betfair_stitched
```

**Flags**:
- `-since` - Start date (YYYY-MM-DD, required)
- `-until` - End date (YYYY-MM-DD, default: today)
- `-betfair-root` - Path to Betfair stitched CSV directory
- `-db` - PostgreSQL connection string
- `-limit` - Limit number of missing days to show (default: 0 = all)
- `-dry-run` - Don't backfill, just report (default: true)

**Examples**:
```bash
# Check all time
./bin/check_missing -since 2006-01-01

# Check October 2025 only
./bin/check_missing -since 2025-10-01 -until 2025-10-31

# Show first 10 missing dates
./bin/check_missing -since 2006-01-01 -limit 10
```

Full documentation: [check_missing/README.md](check_missing/README.md)

---

## Building All Tools

```bash
# From backend-api/ directory
go build -o bin/api ./cmd/api/
go build -o bin/load_master ./cmd/load_master/
go build -o bin/backfill_dates ./cmd/backfill_dates/
go build -o bin/check_missing ./cmd/check_missing/

# Or use a script
for cmd in api load_master backfill_dates check_missing; do
  go build -o bin/$cmd ./cmd/$cmd/
done
```

## Common Workflows

### Initial Setup (Empty Database)
```bash
# Option 1: Load from master CSV files (slow but complete)
./bin/load_master -v

# Option 2: Restore from backup (fast)
docker exec -i horse_racing psql -U postgres -d horse_db < ../postgres/db_backup.sql
```

### Daily Update
```bash
# Option 1: Use auto-update service (recommended)
AUTO_UPDATE_ON_STARTUP=true ./bin/api

# Option 2: Manual backfill
./bin/backfill_dates -since $(date -d "yesterday" +%Y-%m-%d) -dry-run=false
```

### Fix Missing Data
```bash
# 1. Detect gaps
./bin/check_missing -since 2025-01-01 -limit 20

# 2. Backfill specific range
./bin/backfill_dates -since 2025-10-09 -until 2025-10-14 -dry-run=false
```

### Development Testing
```bash
# Load small dataset for testing
./bin/load_master -region gb -type flat -month "2025-10-*" -limit 1 -v

# Check if it loaded
docker exec horse_racing psql -U postgres -d horse_db -c "SELECT COUNT(*) FROM racing.races WHERE race_date >= '2025-10-01';"
```

## Troubleshooting

### "connection refused"
Database isn't running:
```bash
cd ../postgres
docker-compose up -d
```

### "out of memory"
Increase Go memory limit:
```bash
GOMAXPROCS=4 GOGC=50 ./bin/load_master
```

### "too many open files"
Increase file descriptor limit:
```bash
ulimit -n 4096
./bin/load_master
```

### "rate limited" or "HTTP 429"
Racing Post is blocking requests. Wait 1-2 hours and try again.

---

**Documentation**: See `/docs/` for detailed guides
**Support**: Check logs in `logs/` directory

