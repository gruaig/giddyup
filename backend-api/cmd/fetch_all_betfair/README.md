# fetch_all_betfair - Live Betfair Price Fetcher

Standalone command to fetch live Betfair prices for a specific date using the Betfair API-NG.

## Overview

This command reuses the same technology as the automatic live price service (`internal/services/liveprices.go`) but runs as a one-shot operation instead of a continuous loop.

## Usage

```bash
# From backend-api directory
./fetch_all_betfair <date>

# Examples
./fetch_all_betfair 2025-10-16
./fetch_all_betfair $(date +%Y-%m-%d)  # today
./fetch_all_betfair --date 2025-10-17
```

## Prerequisites

1. **Race data must exist**: Run `fetch_all` first to populate Sporting Life data
   ```bash
   ./fetch_all 2025-10-16
   ./fetch_all_betfair 2025-10-16
   ```

2. **Betfair credentials**: Environment variables must be set (in `settings.env`):
   ```bash
   BETFAIR_APP_KEY=your_app_key
   BETFAIR_SESSION_TOKEN=your_session_token
   # OR
   BETFAIR_USERNAME=your_username
   BETFAIR_PASSWORD=your_password
   ```

3. **Active markets**: Only fetches prices for races that haven't finished yet (within 30 minutes past off time)

## What It Does

### Pipeline (4 Steps)

1. **Load Races from Database**
   - Queries all races for the specified date
   - Loads all runners with IDs

2. **Discover Betfair Markets**
   - Calls Betfair API-NG `listMarketCatalogue`
   - Filters for UK/IRE horse racing WIN markets
   - Returns market IDs and selection IDs

3. **Match Races to Markets**
   - Uses `internal/betfair/matcher.go` matching logic
   - Matches by: Date + Course + Time (±1min) + Region
   - Creates mapping: `marketID → raceID` and `selectionID → runnerID`

4. **Fetch Live Prices**
   - Calls Betfair API-NG `listMarketBook` for active markets
   - Extracts back price, lay price, VWAP for each runner
   - Inserts into `racing.live_prices` table
   - Mirrors latest prices to `racing.runners` table

### Price Extraction

For each runner, extracts:
- **Back Price**: Best available back odds (highest price to back at)
- **Lay Price**: Best available lay odds (lowest price to lay at)
- **VWAP**: Volume-weighted average price from traded volume
- **Traded Volume**: Total amount matched on Betfair

### Database Updates

#### racing.live_prices
Inserts time-series price data:
```sql
(race_id, runner_id, ts, back_price, lay_price, vwap, traded_vol)
```

#### racing.runners
Mirrors latest prices (non-destructive):
```sql
win_ppwap = latest VWAP
win_ppmax = max(current, latest back)
win_ppmin = min(current, latest lay)
```

## Time Filtering

Only fetches prices for **active** races:
- Race must be scheduled for the specified date
- Current time must be before `off_time + 30 minutes`
- Skips races that have already finished

## Output Example

```
🏇 GiddyUp Betfair Live Prices Fetcher
📅 Date: 2025-10-16

🔌 Connecting to database...
✅ Database connected

📥 [1/4] Loading races from database for 2025-10-16...
✅ Found 53 races in database

🔍 [2/4] Discovering Betfair WIN markets for 2025-10-16...
✅ Found 29 Betfair WIN markets

🔀 [3/4] Matching races to Betfair markets...
✅ Matched 29 races to Betfair markets

💰 [4/4] Fetching live Betfair prices...
   ⏭  Skipping 15 markets (past off time + 30min)
   📊 Fetching prices for 14 active markets...
   ✓ Got 14 market books
   ✓ Inserted prices for 14 markets at 16:15:31
   📋 Mirroring latest prices to runners table...
   ✓ Mirrored to runners table

🎉 SUCCESS!
✅ Updated 14 races with live prices
⏭  Skipped 15 races (past off time)
```

## Integration with Auto-Update

This command uses the **exact same logic** as the automatic live price service:

| Component | Source File | Used By |
|-----------|-------------|---------|
| Price extraction | `internal/services/liveprices.go` | Both |
| Race matching | `internal/betfair/matcher.go` | Both |
| Market discovery | `internal/betfair/rest.go` | Both |
| Database schema | `racing.live_prices` | Both |

**Key difference**: 
- Auto-update runs continuously (every 60 seconds)
- This command runs once on demand

## Use Cases

1. **Backfill prices**: Fetch prices for a specific date on demand
2. **Manual refresh**: Force a price update outside the auto-update schedule
3. **Testing**: Verify Betfair integration without running the full server
4. **Debugging**: Check market matching and price extraction for a specific date

## Building

```bash
# From backend-api directory
go build -o bin/fetch_all_betfair cmd/fetch_all_betfair/main.go
```

Or use the wrapper script (builds automatically):
```bash
./fetch_all_betfair 2025-10-16
```

## Related Commands

- `fetch_all <date>` - Fetch Sporting Life data + Betfair historical CSVs
- `backfill_dates <start> <end>` - Backfill multiple dates
- Auto-update service - Continuous live price updates (when server running)

## Notes

- Only works for today/tomorrow (races with active Betfair markets)
- For historical prices, use `fetch_all` (fetches Betfair CSV BSP data)
- Requires Betfair API-NG credentials (same as auto-update service)
- Safe to run multiple times (upserts, doesn't duplicate data)

