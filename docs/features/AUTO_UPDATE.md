# Auto-Update Service

The GiddyUp API includes an intelligent auto-update service that runs in the background when the server starts.

## Overview

The auto-update service automatically:
1. **Finds the last date** in your database
2. **Backfills missing days** from `last_date + 1` to `yesterday`
3. **Runs in a background goroutine** (non-blocking - server starts immediately)

## How It Works

When enabled, the service:

```
Server starts → Wait 5s → Query MAX(race_date) → Calculate missing days → Backfill each day
```

For each missing date, it:
1. **Scrapes Racing Post** for race results
2. **Fetches & stitches Betfair** BSP/PPWAP prices
3. **Merges** Racing Post + Betfair data
4. **Inserts** into database (with idempotent upserts)
5. **Pauses 15-30s** before next date (rate limiting)

## Configuration

### Enable/Disable

Set environment variable:

```bash
export AUTO_UPDATE_ON_STARTUP=true   # Enable auto-update
export AUTO_UPDATE_ON_STARTUP=false  # Disable (default)
```

### Data Directory

Set the root directory for cached Racing Post and Betfair data:

```bash
export DATA_DIR=/home/smonaghan/GiddyUp/data
```

Default: `/home/smonaghan/GiddyUp/data`

## Example Startup

```bash
# Start server with auto-update enabled
AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

### Expected Output

```
=== GiddyUp API Starting ===
Environment: development
Server Port: 8080
Connecting to database...
✅ Database connection established
✅ Search path set to: racing, public
🔄 Auto-update service enabled
   Data directory: /home/smonaghan/GiddyUp/data
Initializing router and handlers...
✅ GiddyUp API is running on http://localhost:8080
=====================================

[AutoUpdate] 🔍 Checking for missing data...
[AutoUpdate] 📅 Backfilling 3 days (2025-10-12 to 2025-10-14)...
[AutoUpdate] Processing 2025-10-12...
[Scraper] Fetching https://www.racingpost.com/results/2025-10-12
[AutoUpdate] ✅ 2025-10-12: 43 races, 476 runners
[AutoUpdate] ⏸️  Pausing 22s before next date...
...
[AutoUpdate] 🎉 Backfill complete! Success: 3, Failed: 0
```

## Rate Limiting

The auto-update service includes aggressive rate limiting to avoid being blocked by Racing Post:

- **5-8s delay** between races (with jitter)
- **15-30s pause** between dates
- **Rotating user agents** (15+ different browsers)
- **Circuit breaker** (3 consecutive failures = 5 min pause)
- **Exponential backoff** on errors (30s → 120s → 270s)
- **HTTP 429 handling** (wait 5 minutes on rate limit)

## Database Schema

The service uses **idempotent upserts** (`ON CONFLICT DO UPDATE`) so it's safe to run multiple times:

- **Races**: `(race_key, race_date)` unique constraint
- **Runners**: `(runner_key, race_date)` unique constraint
- **Dimensions**: `course_norm`, `horse_norm`, etc. unique constraints

Existing data is updated (not duplicated).

## Manual Backfill

For manual backfilling of specific date ranges, use the dedicated CLI tool:

```bash
# Backfill a specific date range
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14

# Dry-run to see what would be done
./bin/backfill_dates -since 2025-10-01 -until 2025-10-14 -dry-run
```

## Troubleshooting

### Service Not Running

Check that `AUTO_UPDATE_ON_STARTUP=true` is set.

### Rate Limited / Blocked

If you see many 429 or 403 errors, the Racing Post API may have blocked your IP temporarily.

**Solutions**:
- Wait 1-2 hours and try again
- Use a VPN or different IP address
- Increase delays in `internal/scraper/results.go`

### Missing Dates Still Exist

Check logs for specific error messages. Common issues:
- Racing Post API unavailable for that date
- Betfair data not available (normal for some old dates)
- Database connection issues

### Server Slow to Start

The auto-update runs in a **background goroutine**, so the server should start immediately and respond to requests while backfilling continues.

If the server appears slow, check:
- Are there many missing dates? (e.g., 100+ days = ~2-3 hours)
- Is database under heavy load?

## Architecture

```
cmd/api/main.go
    ↓
services.NewAutoUpdateService(db, enabled, dataDir)
    ↓
service.RunInBackground()  ← Spawns goroutine (non-blocking)
    ↓
for each missing date:
    scraper.ResultsScraper.ScrapeDate()        ← Racing Post
    scraper.BetfairStitcher.StitchBetfairForDate()  ← Betfair
    matchAndMerge()                             ← Merge prices
    insertToDatabase()                          ← Upsert to Postgres
```

## Performance

Typical backfill speed:
- **~10-15 races/minute** (with aggressive rate limiting)
- **~1 day = 2-3 minutes** (average 12 races/day)
- **~100 days = 3-5 hours** (with pauses)

The service is designed to be **conservative** to avoid detection and blocking.

## Future Enhancements

Potential improvements:
- [ ] Scheduled daily updates (cron-like)
- [ ] Webhooks to notify on completion
- [ ] Progress API endpoint (`/api/v1/autoupdate/status`)
- [ ] Configurable rate limits via env vars
- [ ] Parallel processing (with careful rate limiting)

