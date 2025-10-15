# Missing Data Checker

A data verification tool that identifies missing racing data by comparing Betfair files against database contents, with optional backfill capabilities.

## Overview

This tool scans your Betfair directory structure to determine what racing data should exist, queries your database to see what's actually loaded, and identifies gaps. It can then optionally backfill missing data by scraping Racing Post and inserting into the database.

## Features

🔍 **Smart Discovery**: Scans Betfair CSV files to build expected data calendar  
💾 **Database Comparison**: Queries loaded races to identify what's missing  
📊 **Gap Analysis**: Shows missing data breakdown by region/type  
🚀 **Automatic Backfill**: Can scrape and insert missing race data  
⚡ **Concurrent Processing**: Multi-worker backfill for speed  
🎯 **Flexible Filtering**: Date ranges, limits, region/type filters  
🔄 **Idempotent**: Safe to re-run, handles conflicts gracefully

## Usage

```bash
# Build the tool
go build -o bin/check_missing ./cmd/check_missing

# Discover gaps only (no changes)
./bin/check_missing -dry-run -show-sample

# Check specific date range  
./bin/check_missing -dry-run -since 2024-01-01 -until 2024-12-31

# Backfill up to 10 missing days (default limit)
./bin/check_missing

# Backfill without limit from 2020-2022 with 5 workers
./bin/check_missing -since 2020-01-01 -until 2022-12-31 -limit 0 -workers 5

# Quiet mode
./bin/check_missing -v=false
```

## Command Line Options

| Flag | Default | Description |
|------|---------|-------------|
| `-db` | `host=localhost...` | Postgres connection string |
| `-betfair-root` | `/home/smonaghan/rpscrape/data/betfair_stitched` | Betfair CSV directory |
| `-since` | `2006-01-01` | Lower date bound (YYYY-MM-DD) |
| `-until` | today | Upper date bound (YYYY-MM-DD) |
| `-limit` | `10` | Max missing days to backfill (0 = no limit) |
| `-dry-run` | `false` | Discover only, don't scrape/insert |
| `-show-sample` | `false` | Show sample of missing days by region/type |
| `-workers` | `3` | Number of concurrent workers for backfill |
| `-v` | `true` | Verbose logging |

## How It Works

### 1. Discovery Phase
- Scans Betfair directory structure recursively
- Matches files like: `gb_flat_2024-01-01_1400.csv`
- Extracts: region (GB/IRE), type (flat/jumps), date
- Builds calendar of expected racing days

### 2. Database Query Phase  
- Queries `racing.races` table for existing data
- Groups by date, region, race_type
- Normalizes race types for comparison

### 3. Gap Analysis
- Compares expected vs loaded calendars
- Identifies missing days by region/type
- Optionally shows sample breakdown

### 4. Backfill Phase (if enabled)
- Scrapes Racing Post for missing dates
- Upserts into database with conflict handling
- Uses prepared statements for performance
- Processes concurrently with worker pool

## Data Flow

```
Betfair Files → Expected Calendar
     +
Database Races → Loaded Calendar  
     ↓
Gap Analysis → Missing Days List
     ↓  
Racing Post Scrape → Race Results
     ↓
Database Upsert → Filled Gaps
```

## Race Type Mapping

| Betfair | Database | Notes |
|---------|----------|-------|
| `flat` | `Flat` | Direct mapping |
| `jumps` | `Hurdle/Chase` | Umbrella term for jump racing |

## Key Generation

Uses same logic as main pipeline:
- **Race Key**: MD5 of `date|region|course|time|name|type` (normalized)
- **Runner Key**: MD5 of `race_key|horse|num_or_draw` (normalized)

## Database Schema

Expects PostgreSQL with:
- Schema: `racing`
- Tables: `races`, `runners`, `courses`, `horses`, `jockeys`, `trainers`
- Unique constraints on normalized fields
- Function: `racing.norm_text()` for name normalization

## Error Handling

- **Missing Betfair Directory**: Fails fast with clear message
- **Scrape Errors**: Logged and skipped, processing continues  
- **Database Errors**: Transaction rollback, detailed error messages
- **Network Issues**: Retries with exponential backoff (via scraper)

## Performance Notes

- **Concurrent Workers**: Default 3, increase for faster backfill
- **Prepared Statements**: Reused for dimension upserts
- **Transactions**: One per day for consistency
- **Memory Usage**: Processes one day at a time

## Example Output

```bash
🔍 Scanning betfair_root=/data/betfair_stitched range=2024-01-01..2024-01-31
📅 Expected days from Betfair: 76
💾 Loaded days in DB: 0
❌ Missing days (BF expected but DB empty): 76

📊 Missing data sample by region/type:
   GB Flat: 29 days ([2024-01-01 2024-01-02 2024-01-03 ... 2024-01-30 2024-01-31])
   GB Hurdle/Chase: 25 days ([2024-01-01 2024-01-02 2024-01-04 ... 2024-01-30 2024-01-31])
   IRE Flat: 5 days ([2024-01-12 2024-01-16 2024-01-19 2024-01-26 2024-01-31])
   IRE Hurdle/Chase: 17 days ([2024-01-01 2024-01-06 2024-01-07 ... 2024-01-29 2024-01-30])

🚀 Starting backfill of 76 days with 3 workers...
[1/76] 🔄 Backfilling 2024-01-01 GB Flat
  ✅ Inserted 8 races, 89 runners for 2024-01-01 (GB Flat)
[2/76] 🔄 Backfilling 2024-01-01 GB Hurdle/Chase
  ✅ Inserted 3 races, 24 runners for 2024-01-01 (GB Hurdle/Chase)
...

🎉 BACKFILL COMPLETE  
   Days processed: 76
   Races inserted: 1,247
   Runners inserted: 12,891
   Errors: 2
   Time elapsed: 4m32s
   Avg per day: 16.4 races, 169.6 runners
```

## Integration

Can be integrated into:
- **Cron Jobs**: Daily gap checking
- **CI/CD Pipelines**: Data validation steps  
- **Monitoring**: Alert on missing data
- **Admin Tools**: Manual backfill operations

## Troubleshooting

**No missing days found**: Check Betfair directory path and file naming convention

**High error count**: Verify Racing Post scraper configuration and rate limits

**Slow backfill**: Increase workers, check network latency, verify database performance

**Type mismatches**: Review race type normalization logic for your data
