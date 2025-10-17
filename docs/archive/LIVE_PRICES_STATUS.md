# Live Prices Implementation - Status Report

**Date:** 2025-10-15  
**Status:** ✅ Core Implementation Complete

---

## What Was Built

### 1. UK/IRE Race Filtering ✅
- Fixed scraper to only fetch UK/IRE courses (60+ UK, 20+ IRE)
- Filters at URL discovery stage (no wasted scraping)
- International races automatically skipped
- **Verified:** Oct 1-8 have 100% Betfair BSP matching

### 2. Racecard Scraper ✅
- `internal/scraper/racecards.go`
- Fetches today's race cards from Racing Post
- Parses: course, off-time, race name, runners, jockey, trainer
- Sets `prelim=true` and `ran=0` for preliminary data
- **Tested:** Successfully scraped 15 UK/IRE races for Oct 15

### 3. Custom Betfair Client ✅
- `internal/betfair/auth.go` - Login with username/password
- `internal/betfair/rest.go` - ListMarketCatalogue, ListMarketBook
- `internal/betfair/types.go` - Minimal types (no bloat)
- `internal/betfair/matcher.go` - RP ↔ BF market matching with ±1min tolerance

### 4. Live Prices Service ✅
- `internal/services/liveprices.go`
- Fetches prices every 60 seconds (configurable)
- Stores in `racing.live_prices` table with timestamp
- Mirrors latest prices to `racing.runners` (non-destructive)
- Calculates: best back, best lay, VWAP from traded volume

### 5. Auto-Update Integration ✅
- Modified `internal/services/autoupdate.go`
- On server start:
  1. Backfills missing historical dates
  2. Fetches today's racecards (if 6am-11pm)
  3. Discovers Betfair markets
  4. Matches RP ↔ BF
  5. Starts live price updates in background

### 6. Database Schema ✅
- Added `prelim boolean` column to `racing.races`
- Created `racing.live_prices` table
- Indices for efficient price queries
- Migration: `postgres/migrations/009_live_prices.sql`

### 7. Configuration ✅
- `settings.env` in project root
- Contains all credentials and settings
- Usage: `source settings.env` before running

---

## Current Issues

### 🔴 Temporary IP Ban
- Racing Post `/results` endpoint → 403 Forbidden
- `/racecards` endpoint → 200 OK ✅
- Expected to clear in 12-24 hours
- **Impact:** Can't backfill Oct 9-14 until ban lifts

### ⚠️ Racecard Parser Incomplete
- Course name and off-time: ✅ Working
- Race name: ✅ Working
- Runners: ✅ Getting 14 runners per race
- **Still need:** Distance, going, class extraction
- **Still need:** Better runner detail parsing

---

## How It Works

### Server Startup Flow
```
1. Server starts
2. Auto-update service triggers (if AUTO_UPDATE_ON_STARTUP=true)
3. Backfills Oct 9 → Oct 14 (currently blocked by IP ban)
4. Fetches today's (Oct 15) racecards → inserts with prelim=true
5. Discovers Betfair WIN markets for today (GB/IE)
6. Matches races by venue + time (±1 min)
7. Starts background goroutine updating prices every 60s
```

### Live Price Updates
```
Every 60 seconds:
1. Fetch MarketBook for all matched markets
2. Extract best back/lay prices + VWAP
3. Insert into racing.live_prices with timestamp
4. Mirror latest prices to racing.runners (COALESCE, non-destructive)
```

### Tomorrow Morning
```
1. IP ban lifts
2. Auto-update runs backfill for Oct 9-14 (results + BSP)
3. Re-runs Oct 15 with -source=results
4. Flips prelim=false, fills positions/RPR/OR/official BSP
```

---

## Database Status

**Oct 1-8:** 331 races, 100% BSP prices ✅  
**Oct 9-10:** Deleted (had bad data from old scraper)  
**Oct 11-14:** Pending (IP ban) - Betfair data ready  
**Oct 15:** Ready for racecards + live prices  

---

## Betfair Credentials

**App Key:** `Gs1Zut6sZQxncj6V` (1-min delayed)  
**Session Token:** `4v7GaQq24grMhmREdPRFUheb3b3bpowU6JP5QUo7jHw=`  
**Username:** `colfish`  
**Password:** (user needs to add to settings.env)

---

## Testing Commands

```bash
# Load settings
source settings.env

# Test racecard scraper
cd backend-api
go run cmd/test_racecards/main.go

# Start API server (auto-fetches today + starts live prices)
AUTO_UPDATE_ON_STARTUP=true ENABLE_LIVE_PRICES=true bin/api

# Check live prices in database
psql -h localhost -U postgres -d horse_db -c "
  SELECT COUNT(*), MAX(ts) 
  FROM racing.live_prices 
  WHERE ts > NOW() - INTERVAL '5 minutes'
"
```

---

## Next Steps

1. **Wait for IP ban to lift** (12-24 hours)
2. **Test live prices flow** when ban clears
3. **Add Betfair password** to settings.env
4. **Improve racecard parser** (distance, going, class)
5. **Test full cycle:** racecards → live prices → results backfill

---

## Code Quality

- ✅ All code in Go (no Python dependencies)
- ✅ Reuses existing normalization/matching logic
- ✅ Non-destructive UPSERTs
- ✅ Proper error handling and logging
- ✅ Clean separation of concerns
- ✅ Builds without errors

