# PostgreSQL Database Setup

## Quick Start

### 1. Start PostgreSQL (Docker)
```bash
docker run -d --network=host --name=horse_racing \
  -e POSTGRES_PASSWORD=password \
  postgres:18.0-alpine3.22
```

### 2. Initialize Database
```bash
# Drop and recreate (clean start)
docker exec horse_racing dropdb -U postgres horse_db --if-exists
docker exec horse_racing createdb -U postgres horse_db

# Run init script
docker cp postgres/init_clean.sql horse_racing:/tmp/
docker exec horse_racing psql -U postgres -d horse_db -f /tmp/init_clean.sql
```

### 3. Load Master Data
```bash
# Test on one month
python3 load_master_to_postgres.py --region gb --type flat --month 2024-01

# Load all data
python3 load_master_to_postgres.py

# Load specific subset
python3 load_master_to_postgres.py --region gb --type flat --limit 10
```

## Database Connection

- **Host**: localhost
- **Port**: 5432
- **Database**: horse_db
- **User**: postgres
- **Password**: password
- **Schema**: racing

## Schema Overview

### Dimension Tables
- `courses` - Racing courses/tracks
- `horses` - All horses
- `trainers` - All trainers
- `jockeys` - All jockeys
- `owners` - All owners
- `bloodlines` - Sire/Dam/Damsire combinations

### Fact Tables (Partitioned by race_date)
- `races` - Race metadata
- `runners` - Runner-level data (RP + Betfair)

### Staging Tables
- `stage_races` - Temporary races data
- `stage_runners` - Temporary runners data

## Key Features

1. **Monthly Partitioning**: Tables partitioned by `race_date` for performance
2. **Text Normalization**: `norm_text()` function for fuzzy matching
3. **Trigram Search**: GIN indexes on names for fast fuzzy search
4. **Full-Text Search**: On runner comments
5. **Idempotent Loads**: Re-runnable without duplicates (ON CONFLICT)
6. **Data Validation**: Automated quality checks on every load

## Loader Usage

```bash
# Initialize and load
python3 load_master_to_postgres.py --init --region gb --type flat --month 2024-01

# Load all historical data
python3 load_master_to_postgres.py

# Load specific months
python3 load_master_to_postgres.py --region gb --type jumps --limit 50
```

## Verify Data

```sql
SET search_path TO racing;

-- Counts
SELECT COUNT(*) FROM races;
SELECT COUNT(*) FROM runners;
SELECT COUNT(*) FROM horses;

-- Sample race
SELECT r.race_name, r.going, r.dist_f, r.ran, 
       COUNT(ru.runner_id) as actual_runners
FROM races r
LEFT JOIN runners ru ON r.race_id = ru.race_id
GROUP BY r.race_id, r.race_name, r.going, r.dist_f, r.ran
LIMIT 5;

-- Betfair coverage
SELECT 
  COUNT(*) as total_runners,
  COUNT(win_bsp) as with_win_bsp,
  COUNT(place_bsp) as with_place_bsp
FROM runners;
```

## Files

- `init_clean.sql` - Clean database initialization (use this!) ⭐
- `init.sql` - Original init (deprecated, use init_clean.sql)
- `database.md` - Full schema documentation
- `OPTIMIZATION_NOTES.md` - Performance optimization guide (NEW)
- `CHANGELOG.md` - Schema version history (NEW)
- `../load_master_to_postgres.py` - Python loader script
- `../backend-api/` - Go/Gin REST API using this database


