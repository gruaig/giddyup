# Auto-Update Service - Example Logs (Verbose)

## Server Startup

```
[2025-10-14 23:15:00.123] INFO:  === GiddyUp API Starting ===
[2025-10-14 23:15:00.124] INFO:  Environment: development
[2025-10-14 23:15:00.124] INFO:  Server Port: 8000
[2025-10-14 23:15:00.124] INFO:  Connecting to database...
[2025-10-14 23:15:00.125] INFO:  Database: postgres@localhost:5432/horse_db
[2025-10-14 23:15:00.156] INFO:  ✅ Database connection established
[2025-10-14 23:15:00.156] INFO:  ✅ Search path set to: racing, public
[2025-10-14 23:15:00.156] INFO:  🔄 Auto-update service enabled
[2025-10-14 23:15:00.156] INFO:     Data directory: /home/smonaghan/GiddyUp/data
[2025-10-14 23:15:00.157] INFO:  Initializing router and handlers...
[2025-10-14 23:15:00.158] INFO:  ✅ GiddyUp API is running on http://localhost:8000
[2025-10-14 23:15:00.158] INFO:  Health check: http://localhost:8000/health
[2025-10-14 23:15:00.158] INFO:  API endpoints: http://localhost:8000/api/v1/*
[2025-10-14 23:15:00.158] INFO:  =====================================
[2025-10-14 23:15:00.158] INFO:  Starting HTTP server on port 8000...
```

## Background Auto-Update Process

```
[2025-10-14 23:15:05.160] [AutoUpdate] 🔍 Checking for missing data...
[2025-10-14 23:15:05.165] [AutoUpdate] 📅 Backfilling 3 days (2025-10-12 to 2025-10-14)...
[2025-10-14 23:15:05.165] [AutoUpdate] Processing 2025-10-12...

[2025-10-14 23:15:05.165] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-12...
[2025-10-14 23:15:05.167] [Scraper] Fetching https://www.racingpost.com/results/2025-10-12
[2025-10-14 23:15:05.856] [Scraper] Found 12 race URLs for 2025-10-12
[2025-10-14 23:15:05.856] [Scraper] Scraping race 1/12: /results/35/southwell/2025-10-12/850723
[2025-10-14 23:15:06.234] [Scraper]   ✓ Southwell 12:30 - 12 runners
[2025-10-14 23:15:06.234] [Scraper] Rate limit: sleeping 7.234s before next race
[2025-10-14 23:15:13.468] [Scraper] Scraping race 2/12: /results/38/lingfield/2025-10-12/850724
[2025-10-14 23:15:13.789] [Scraper]   ✓ Lingfield 13:00 - 9 runners
[2025-10-14 23:15:13.789] [Scraper] Rate limit: sleeping 5.891s before next race
[2025-10-14 23:15:19.680] [Scraper] Scraping race 3/12: /results/35/southwell/2025-10-12/850725
[2025-10-14 23:15:20.123] [Scraper]   ✓ Southwell 13:30 - 14 runners
[2025-10-14 23:15:20.123] [Scraper] Rate limit: sleeping 6.456s before next race
...
[2025-10-14 23:16:42.345] [Scraper] Scraping race 12/12: /results/35/southwell/2025-10-12/850734
[2025-10-14 23:16:42.678] [Scraper]   ✓ Southwell 18:30 - 11 runners
[2025-10-14 23:16:42.678] [AutoUpdate]   ✓ Got 12 races from Racing Post

[2025-10-14 23:16:42.678] [AutoUpdate]   [2/4] Fetching Betfair data...
[2025-10-14 23:16:42.679] [Betfair] Fetching BSP data for 2025-10-12 (uk, WIN)
[2025-10-14 23:16:43.123] [Betfair]   ✓ Downloaded WIN file (2.3MB)
[2025-10-14 23:16:43.124] [Betfair] Fetching BSP data for 2025-10-12 (uk, PLACE)
[2025-10-14 23:16:43.567] [Betfair]   ✓ Downloaded PLACE file (2.1MB)
[2025-10-14 23:16:43.568] [Betfair] Stitching 2025-10-12 (uk): 12 WIN races + 12 PLACE races
[2025-10-14 23:16:43.645] [Betfair]   ✓ Stitched 12 races with 142 runners
[2025-10-14 23:16:43.646] [Betfair] Fetching BSP data for 2025-10-12 (ire, WIN)
[2025-10-14 23:16:44.012] [Betfair]   ✓ Downloaded WIN file (1.8MB)
[2025-10-14 23:16:44.013] [Betfair] Fetching BSP data for 2025-10-12 (ire, PLACE)
[2025-10-14 23:16:44.398] [Betfair]   ✓ Downloaded PLACE file (1.7MB)
[2025-10-14 23:16:44.399] [Betfair] Stitching 2025-10-12 (ire): 8 WIN races + 8 PLACE races
[2025-10-14 23:16:44.456] [Betfair]   ✓ Stitched 8 races with 89 runners
[2025-10-14 23:16:44.456] [AutoUpdate]   ✓ Got 20 Betfair races (UK: 12, IRE: 8)

[2025-10-14 23:16:44.456] [AutoUpdate]   [3/4] Matching Racing Post with Betfair data...
[2025-10-14 23:16:44.457] [AutoUpdate]     ✓ Matched Southwell @ 12:30: 12/12 runners with Betfair prices
[2025-10-14 23:16:44.457] [AutoUpdate]     ✓ Matched Lingfield @ 13:00: 9/9 runners with Betfair prices
[2025-10-14 23:16:44.458] [AutoUpdate]     ✓ Matched Southwell @ 13:30: 14/14 runners with Betfair prices
[2025-10-14 23:16:44.458] [AutoUpdate]     ✓ Matched Lingfield @ 13:30: 7/7 runners with Betfair prices
[2025-10-14 23:16:44.459] [AutoUpdate]     ✓ Matched Southwell @ 14:00: 11/11 runners with Betfair prices
[2025-10-14 23:16:44.459] [AutoUpdate]     ✓ Matched Lingfield @ 14:00: 8/8 runners with Betfair prices
[2025-10-14 23:16:44.460] [AutoUpdate]     ✓ Matched Southwell @ 14:30: 13/13 runners with Betfair prices
[2025-10-14 23:16:44.460] [AutoUpdate]     ✓ Matched Lingfield @ 14:30: 6/6 runners with Betfair prices
[2025-10-14 23:16:44.461] [AutoUpdate]     ✓ Matched Southwell @ 15:00: 10/10 runners with Betfair prices
[2025-10-14 23:16:44.461] [AutoUpdate]     ✓ Matched Lingfield @ 15:00: 9/9 runners with Betfair prices
[2025-10-14 23:16:44.462] [AutoUpdate]     ✓ Matched Southwell @ 15:30: 12/12 runners with Betfair prices
[2025-10-14 23:16:44.462] [AutoUpdate]     ✓ Matched Southwell @ 16:00: 11/11 runners with Betfair prices
[2025-10-14 23:16:44.463] [AutoUpdate]   Summary: 12/12 races matched, 142 total runners with Betfair prices
[2025-10-14 23:16:44.463] [AutoUpdate]   ✓ Merged 12 races

[2025-10-14 23:16:44.463] [AutoUpdate]   [4/4] Inserting to database...
[2025-10-14 23:16:44.567] [AutoUpdate]   ✓ Inserted 12 races, 142 runners
[2025-10-14 23:16:44.567] [AutoUpdate] ✅ 2025-10-12: 12 races, 142 runners
[2025-10-14 23:16:44.567] [AutoUpdate] ⏸️  Pausing 23s before next date to avoid rate limiting...

[2025-10-14 23:17:07.568] [AutoUpdate] Processing 2025-10-13...
[2025-10-14 23:17:07.568] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-13...
[2025-10-14 23:17:07.569] [Scraper] Fetching https://www.racingpost.com/results/2025-10-13
[2025-10-14 23:17:07.834] [Scraper] Found 14 race URLs for 2025-10-13
[2025-10-14 23:17:07.834] [Scraper] Scraping race 1/14: /results/21/newcastle/2025-10-13/850748
[2025-10-14 23:17:08.145] [Scraper]   ✓ Newcastle 12:15 - 10 runners
[2025-10-14 23:17:08.145] [Scraper] Rate limit: sleeping 6.789s before next race
...
[2025-10-14 23:18:45.234] [AutoUpdate]   ✓ Got 14 races from Racing Post
[2025-10-14 23:18:45.234] [AutoUpdate]   [2/4] Fetching Betfair data...
[2025-10-14 23:18:46.123] [AutoUpdate]   ✓ Got 22 Betfair races (UK: 14, IRE: 8)
[2025-10-14 23:18:46.123] [AutoUpdate]   [3/4] Matching Racing Post with Betfair data...
[2025-10-14 23:18:46.134] [AutoUpdate]   Summary: 14/14 races matched, 168 total runners with Betfair prices
[2025-10-14 23:18:46.134] [AutoUpdate]   ✓ Merged 14 races
[2025-10-14 23:18:46.134] [AutoUpdate]   [4/4] Inserting to database...
[2025-10-14 23:18:46.278] [AutoUpdate]   ✓ Inserted 14 races, 168 runners
[2025-10-14 23:18:46.278] [AutoUpdate] ✅ 2025-10-13: 14 races, 168 runners
[2025-10-14 23:18:46.278] [AutoUpdate] ⏸️  Pausing 18s before next date to avoid rate limiting...

[2025-10-14 23:19:04.279] [AutoUpdate] Processing 2025-10-14...
[2025-10-14 23:19:04.279] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-14...
[2025-10-14 23:19:04.280] [Scraper] Fetching https://www.racingpost.com/results/2025-10-14
[2025-10-14 23:19:04.567] [Scraper] Found 17 race URLs for 2025-10-14
...
[2025-10-14 23:20:52.456] [AutoUpdate]   ✓ Got 17 races from Racing Post
[2025-10-14 23:20:52.456] [AutoUpdate]   [2/4] Fetching Betfair data...
[2025-10-14 23:20:53.567] [AutoUpdate]   ✓ Got 25 Betfair races (UK: 17, IRE: 8)
[2025-10-14 23:20:53.567] [AutoUpdate]   [3/4] Matching Racing Post with Betfair data...
[2025-10-14 23:20:53.589] [AutoUpdate]   Summary: 17/17 races matched, 201 total runners with Betfair prices
[2025-10-14 23:20:53.589] [AutoUpdate]   ✓ Merged 17 races
[2025-10-14 23:20:53.589] [AutoUpdate]   [4/4] Inserting to database...
[2025-10-14 23:20:53.745] [AutoUpdate]   ✓ Inserted 17 races, 201 runners
[2025-10-14 23:20:53.745] [AutoUpdate] ✅ 2025-10-14: 17 races, 201 runners

[2025-10-14 23:20:53.745] [AutoUpdate] 🎉 Backfill complete! Success: 3, Failed: 0
```

## Summary Timeline

```
T+0ms     : Server starts
T+158ms   : ✅ API is responsive (can accept HTTP requests immediately)

T+5000ms  : Background: Start checking for missing data
T+5165ms  : Background: Found 3 missing days (2025-10-12 to 2025-10-14)
T+5165ms  : Background: Start processing 2025-10-12
T+102,567ms : Background: ✅ 2025-10-12 complete (12 races, 142 runners)
T+125,568ms : Background: Start processing 2025-10-13 (after 23s pause)
T+221,278ms : Background: ✅ 2025-10-13 complete (14 races, 168 runners)
T+239,279ms : Background: Start processing 2025-10-14 (after 18s pause)
T+348,745ms : Background: ✅ 2025-10-14 complete (17 races, 201 runners)
T+348,745ms : Background: 🎉 Backfill complete! Success: 3, Failed: 0

Total API downtime: 0ms (server was responsive the entire time)
Total backfill time: ~5 minutes 48 seconds
```

## Key Features in Logs

### 1. **Step-by-Step Progress**
Each date shows all 4 steps:
- `[1/4] Scraping Racing Post`
- `[2/4] Fetching Betfair data`
- `[3/4] Matching Racing Post with Betfair data`
- `[4/4] Inserting to database`

### 2. **Detailed Match Information**
For each race that matches Betfair data:
```
[AutoUpdate]     ✓ Matched Southwell @ 12:30: 12/12 runners with Betfair prices
```

### 3. **Summary Statistics**
After matching:
```
[AutoUpdate]   Summary: 12/12 races matched, 142 total runners with Betfair prices
```

### 4. **Rate Limiting Visibility**
Shows pauses between dates:
```
[AutoUpdate] ⏸️  Pausing 23s before next date to avoid rate limiting...
```

### 5. **Per-Date Success**
Clear success message with counts:
```
[AutoUpdate] ✅ 2025-10-12: 12 races, 142 runners
```

### 6. **Final Summary**
Overall completion status:
```
[AutoUpdate] 🎉 Backfill complete! Success: 3, Failed: 0
```

## Error Example

If something fails, you'll see detailed errors:

```
[2025-10-14 23:15:05.165] [AutoUpdate] Processing 2025-10-12...
[2025-10-14 23:15:05.165] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-12...
[2025-10-14 23:15:05.167] [Scraper] Fetching https://www.racingpost.com/results/2025-10-12
[2025-10-14 23:15:05.856] [Scraper] ⚠️  HTTP 429 (Too Many Requests), waiting 5m0s before retry
[2025-10-14 23:20:05.857] [Scraper] Retrying after rate limit pause...
[2025-10-14 23:20:06.123] [Scraper] Found 12 race URLs for 2025-10-12
[2025-10-14 23:20:06.123] [Scraper] Scraping race 1/12: /results/35/southwell/2025-10-12/850723
...
```

Or if a date completely fails:

```
[2025-10-14 23:15:05.165] [AutoUpdate] Processing 2025-10-12...
[2025-10-14 23:15:05.165] [AutoUpdate]   [1/4] Scraping Racing Post for 2025-10-12...
[2025-10-14 23:15:05.234] [AutoUpdate] ❌ Failed 2025-10-12: scrape failed: HTTP 403 Forbidden
[2025-10-14 23:15:05.234] [AutoUpdate] Processing 2025-10-13...
...
[2025-10-14 23:20:53.745] [AutoUpdate] 🎉 Backfill complete! Success: 2, Failed: 1
```

## Viewing Logs in Real-Time

```bash
# Start server with auto-update
AUTO_UPDATE_ON_STARTUP=true ./bin/api

# Or run in background and tail logs
AUTO_UPDATE_ON_STARTUP=true ./bin/api > logs/api.log 2>&1 &
tail -f logs/api.log
```

## Database Already Up-to-Date

If no backfill is needed:

```
[2025-10-14 23:15:05.160] [AutoUpdate] 🔍 Checking for missing data...
[2025-10-14 23:15:05.165] [AutoUpdate] ✅ Database is up to date (last: 2025-10-13, yesterday: 2025-10-13)
```

