# fetch_all - Standalone Data Fetcher

Fetches complete race data for a specific date from Sporting Life, merges with Betfair stitched CSV data (BSP, PPWAP, etc.), and upserts to the database.

## Usage

```bash
cd backend-api

# Fetch data for a specific date
./fetch_all 2024-10-15

# With flags
./fetch_all --date 2024-10-15

# Force refresh (delete existing data first)
./fetch_all --date 2024-10-15 --force

# Or use the binary directly
./bin/fetch_all 2024-10-15
```

## What It Does

1. **Fetches from Sporting Life API**
   - Uses SportingLifeAPIV2 (2-endpoint merge)
   - Gets jockey, trainer, owner, form, odds, Betfair selection IDs

2. **Fetches Betfair Stitched Data**
   - Loads CSV files from `/data/betfair_stitched/`
   - Includes BSP, PPWAP, morning WAP, PP max/min
   - Processes both UK and IRE regions

3. **Matches and Merges**
   - Matches races by course, time, and race name
   - Merges runners by horse name
   - Combines Sporting Life + Betfair data

4. **Upserts to Database**
   - Inserts/updates dimension tables (courses, horses, jockeys, trainers, owners)
   - Inserts/updates races and runners
   - Uses UPSERT (ON CONFLICT DO UPDATE)

## Options

- `--date YYYY-MM-DD` - Date to fetch (required)
- `--force` - Delete existing data before fetching

## Environment Variables

Required (from `settings.env`):
- `DB_HOST` - Database host (default: localhost)
- `DB_PORT` - Database port (default: 5432)
- `DB_NAME` - Database name (default: horse_db)
- `DB_USER` - Database user (default: postgres)
- `DB_PASSWORD` - Database password (default: password)
- `DATA_DIR` - Data directory (default: /home/smonaghan/GiddyUp/data)

## Examples

### Fetch yesterday's data
```bash
./fetch_all $(date -d "yesterday" +%Y-%m-%d)
```

### Fetch last 7 days
```bash
for i in {0..6}; do
    DATE=$(date -d "$i days ago" +%Y-%m-%d)
    echo "Fetching $DATE..."
    ./fetch_all $DATE
    sleep 2
done
```

### Fetch specific month (October 2024)
```bash
for day in {01..31}; do
    ./fetch_all 2024-10-$day || true
done
```

### Force refresh a date
```bash
./fetch_all 2024-10-15 --force
```

## Output

```
🏇 GiddyUp Data Fetcher
📅 Date: 2024-10-15
🔄 Force refresh: false

🔌 Connecting to database...
✅ Database connected

📥 [1/4] Fetching race data from Sporting Life for 2024-10-15...
✅ Got 47 UK/IRE races from Sporting Life

📥 [2/4] Fetching Betfair CSV data...
   • Stitching UK Betfair data...
   • Stitching IRE Betfair data...
✅ Got 42 Betfair races (UK: 35, IRE: 7)

🔀 [3/4] Matching and merging Sporting Life ↔ Betfair...
   • Matched 41/47 races with Betfair data
✅ Merged 47 races

💾 [4/4] Inserting to database...
   • Upserting courses, horses, jockeys, trainers, owners...
   • Looking up foreign key IDs...

🎉 SUCCESS!
✅ Inserted 47 races with 531 runners for 2024-10-15
```

## Building

```bash
cd backend-api
go build -o bin/fetch_all cmd/fetch_all/main.go
```

## Comparison with Other Tools

| Tool | Purpose | Use Case |
|------|---------|----------|
| `fetch_all` | Single date, complete pipeline | Backfill specific dates, one-off fetches |
| `backfill_dates` | Date range, missing data detection | Bulk backfill, fill gaps |
| `autoupdate` (server) | Auto-update on startup | Production, always-on server |

## Notes

- **Idempotent**: Safe to run multiple times (uses UPSERT)
- **Requires Betfair CSV files**: Must exist in `/data/betfair_stitched/`
- **Sporting Life**: Works for any date (historical or recent)
- **Performance**: ~40-50 seconds per date (with Betfair matching)
- **Caching**: Uses Sporting Life cache if available

## Troubleshooting

### "No Betfair data found"
- Check that CSV files exist in `/data/betfair_stitched/{region}/{type}/{date}.csv`
- Some dates may not have Betfair data available

### "Database connection failed"
- Verify `settings.env` is in parent directory
- Check database is running: `psql -U postgres -d horse_db`

### "Date already exists"
- Use `--force` flag to refresh existing data
- Or manually delete: `DELETE FROM racing.races WHERE race_date = 'YYYY-MM-DD'`

## See Also

- [02_API_DOCUMENTATION.md](../../docs/02_API_DOCUMENTATION.md) - API endpoints
- [06_SPORTING_LIFE_API.md](../../docs/06_SPORTING_LIFE_API.md) - Data source details
- [backfill_dates](../backfill_dates/) - Bulk backfill tool

