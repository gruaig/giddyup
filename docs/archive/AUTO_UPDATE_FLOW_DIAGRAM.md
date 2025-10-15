# Auto-Update Service Flow Diagram

```
┌─────────────────────────────────────────────────────────────────────┐
│                         SERVER STARTUP                               │
└─────────────────────────────────────────────────────────────────────┘
                                  │
                                  ▼
                    ┌─────────────────────────┐
                    │  Load Config & Connect  │
                    │      to Database        │
                    └─────────────────────────┘
                                  │
                                  ▼
         ┌────────────────────────────────────────────┐
         │  AUTO_UPDATE_ON_STARTUP = true?            │
         └────────────────────────────────────────────┘
                     │                    │
                     │ NO                 │ YES
                     ▼                    ▼
            ┌─────────────┐    ┌──────────────────────┐
            │   Skip      │    │ NewAutoUpdateService │
            │ Auto-Update │    │ (db, true, dataDir)  │
            └─────────────┘    └──────────────────────┘
                     │                    │
                     │                    ▼
                     │          ┌──────────────────────┐
                     │          │ RunInBackground()    │
                     │          │ ↓ spawns goroutine   │
                     │          │ ↓ returns immediately│
                     │          └──────────────────────┘
                     │                    │
                     │                    │
┌────────────────────┼────────────────────┘
│                    │
│  ┌─────────────────▼─────────────────┐
│  │   Setup Router & Start Server     │
│  │   ✅ API is now responsive         │
│  └───────────────────────────────────┘
│
│  ┌───────────────────────────────────────────────────────────────┐
│  │                    BACKGROUND GOROUTINE                         │
│  │                    (runs in parallel)                           │
│  └───────────────────────────────────────────────────────────────┘
│                              │
│                              ▼
│                   ┌──────────────────┐
│                   │  Sleep 5 seconds │
│                   │ (let server start)│
│                   └──────────────────┘
│                              │
│                              ▼
│               ┌──────────────────────────┐
│               │ Query MAX(race_date)     │
│               │ from racing.races        │
│               └──────────────────────────┘
│                              │
│                              ▼
│            ┌─────────────────────────────────┐
│            │ last_date = 2025-10-08          │
│            │ yesterday = 2025-10-14          │
│            │ missing = [2025-10-09 to -14]   │
│            └─────────────────────────────────┘
│                              │
│                              ▼
│                ┌─────────────────────────┐
│                │ FOR EACH missing date   │
│                │ (2025-10-09 to -14)     │
│                └─────────────────────────┘
│                              │
│           ┌──────────────────┴────────────────────┐
│           │                                       │
│           ▼                                       ▼
│   ┌──────────────────┐                 ┌────────────────────┐
│   │  backfillDate()  │                 │  (if last date)    │
│   └──────────────────┘                 │  → No pause        │
│           │                             └────────────────────┘
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 1. Scrape Racing Post                    │
│   │    • ResultsScraper.ScrapeDate()        │
│   │    • Get races + runners                │
│   │    • 5-8s delay between races           │
│   │    • Circuit breaker on failures        │
│   └──────────────────────────────────────────┘
│           │
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 2. Fetch & Stitch Betfair                │
│   │    • BetfairStitcher.StitchBetfairForDate()│
│   │    • Download WIN + PLACE files         │
│   │    • Merge into single stitched file    │
│   │    • For both 'uk' and 'ire' regions    │
│   └──────────────────────────────────────────┘
│           │
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 3. Match & Merge                         │
│   │    • matchAndMerge(rpRaces, bfRaces)    │
│   │    • Match by (date, race_name, time)   │
│   │    • Merge BSP/PPWAP into runners       │
│   └──────────────────────────────────────────┘
│           │
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 4. Insert to Database                    │
│   │    • insertToDatabase(dateStr, races)   │
│   │    • Upsert courses, horses, etc.       │
│   │    • Upsert races (ON CONFLICT DO UPDATE)│
│   │    • Upsert runners (ON CONFLICT DO UPDATE)│
│   └──────────────────────────────────────────┘
│           │
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 5. Log Success                           │
│   │    ✅ 2025-10-09: 43 races, 476 runners  │
│   └──────────────────────────────────────────┘
│           │
│           ▼
│   ┌──────────────────────────────────────────┐
│   │ 6. Rate Limit Pause                      │
│   │    ⏸️  Pausing 15-30s (random jitter)    │
│   └──────────────────────────────────────────┘
│           │
│           └──────────┐
│                      │
│         ┌────────────▼────────────┐
│         │ Next date in loop?      │
│         └────────────┬────────────┘
│                      │
│           YES ◄──────┘        NO
│            │                   │
│            └─────┐             ▼
│                  │   ┌──────────────────────┐
│                  │   │ 🎉 Backfill complete! │
│                  │   │ Success: 6, Failed: 0 │
│                  │   └──────────────────────┘
│                  │              │
│                  ▼              ▼
│         ┌────────────────────────────┐
│         │   Background task ends     │
│         │   (goroutine exits)        │
│         └────────────────────────────┘
│
│
▼
┌───────────────────────────────────────────────┐
│  SERVER CONTINUES RUNNING                      │
│  • API accepts requests                        │
│  • Database now up-to-date                     │
│  • Auto-update completed in background         │
└───────────────────────────────────────────────┘
```

## Key Timings

```
T+0ms    : Server starts
T+50ms   : Database connected
T+100ms  : Auto-update goroutine spawned (returns immediately)
T+150ms  : Router initialized
T+200ms  : HTTP server starts listening
T+250ms  : ✅ API is responsive (can accept requests)

T+5000ms : Background: Query last_date from database
T+5010ms : Background: Calculate missing dates (6 days)
T+5020ms : Background: Start backfilling 2025-10-09

T+~3min  : Background: 2025-10-09 complete (43 races)
T+~6min  : Background: 2025-10-10 complete (47 races)
...
T+~18min : Background: 🎉 All 6 dates complete
```

## Concurrent Operations

While auto-update runs in the background:

```
┌─────────────────────┐          ┌──────────────────────┐
│   Main Thread       │          │  Background Thread   │
│                     │          │                      │
│  • Handle HTTP      │          │  • Scrape Racing Post│
│    requests         │          │  • Fetch Betfair     │
│                     │          │  • Insert to DB      │
│  • Execute queries  │  ◄────►  │  • Rate limiting     │
│                     │ (shared) │    (sleep)           │
│  • Return results   │   DB     │                      │
│                     │          │  • Log progress      │
│  • Log access       │          │                      │
└─────────────────────┘          └──────────────────────┘
```

**Shared Resource**: PostgreSQL database
- Both threads can read/write concurrently
- Postgres handles locking and concurrency
- Auto-update uses transactions for consistency

## Rate Limiting Strategy

```
Racing Post Requests per Minute
│
│  🔴 Blocked (>20 req/min)
│  ─────────────────────────────
│
│  🟡 Risky (15-20 req/min)
│  ─────────────────────────────
│
│  🟢 Safe (<10 req/min)
│  ─────────────────────────────
│                              ▲
│                              │ Our target: 6-8 req/min
│                              │
└──────────────────────────────┴─────────────► Time

Strategy:
• 5-8s delay between races = 7-12 races/min
• But: Circuit breaker pauses = reduces avg to ~6 races/min
• Plus: 15-30s pause between dates = further reduces rate
• Result: Very safe, under Racing Post radar
```

## Error Handling Flow

```
┌──────────────────────┐
│  Scrape Race         │
└──────────────────────┘
           │
           ▼
    ┌──────────┐
    │ Success? │
    └──────────┘
           │
     ┌─────┴─────┐
     │           │
    YES          NO
     │           │
     ▼           ▼
   Return    ┌──────────────┐
   Race      │ HTTP 429?    │
   Data      └──────────────┘
                    │
              ┌─────┴─────┐
              │           │
             YES          NO
              │           │
              ▼           ▼
      ┌────────────┐  ┌──────────────┐
      │ Wait 5 min │  │ HTTP 403?    │
      └────────────┘  └──────────────┘
              │               │
              │         ┌─────┴─────┐
              │         │           │
              │        YES          NO
              │         │           │
              │         ▼           ▼
              │   ┌──────────┐  ┌──────────────┐
              │   │  FATAL   │  │ Retry with   │
              │   │  ERROR   │  │ backoff      │
              │   └──────────┘  │ (30s,120s,   │
              │                 │  270s)       │
              │                 └──────────────┘
              │                         │
              └─────────────────────────┘
                        │
                   Retry (3x max)
```

