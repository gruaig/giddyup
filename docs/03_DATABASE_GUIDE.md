# Database Guide - GiddyUp Racing Database

**Complete PostgreSQL schema reference and maintenance guide**

## Quick Reference

- **Database**: `horse_db`
- **Schema**: `racing`
- **Version**: PostgreSQL 16
- **Size**: ~2.5GB
- **Rows**: 226K races, 2.2M runners
- **Date Range**: 2008-2025

## Table of Contents

1. [Schema Overview](#schema-overview)
2. [Core Tables](#core-tables)
3. [Dimension Tables](#dimension-tables)
4. [Materialized Views](#materialized-views)
5. [Indexes](#indexes)
6. [Queries](#common-queries)
7. [Maintenance](#maintenance)

---

## 1. Schema Overview

### Table Relationships

```
┌─────────────┐
│  courses    │──┐
└─────────────┘  │
                 │
┌─────────────┐  │    ┌──────────────┐
│   horses    │──┼───→│    races     │
└─────────────┘  │    │ (partitioned)│
                 │    └──────┬───────┘
┌─────────────┐  │           │
│  trainers   │──┤           │
└─────────────┘  │           │
                 │           │
┌─────────────┐  │           │
│  jockeys    │──┤           │
└─────────────┘  │           ▼
                 │    ┌──────────────┐
┌─────────────┐  │    │   runners    │
│   owners    │──┤    │ (partitioned)│
└─────────────┘  │    └──────────────┘
                 │
┌─────────────┐  │
│ bloodlines  │──┘
└─────────────┘
```

### Key Concepts

**Partitioning**: `races` and `runners` tables are partitioned by year for performance
- Each year is a separate table (e.g., `races_2024_01`, `races_2024_02`, etc.)
- Queries automatically route to correct partition
- Improves query speed and maintenance

**Materialized Views**: Pre-computed denormalized data for fast queries
- `mv_runner_base` - Runner data with race details (2M rows)
- Refresh after data loads

**Generated Columns**: Normalized text for matching
- All dimension tables have `*_norm` columns
- Uses `racing.norm_text()` function
- Enables fuzzy matching

---

## 2. Core Tables

### races (partitioned by race_date)

**Purpose**: Race metadata

| Column | Type | Description |
|--------|------|-------------|
| race_id | BIGINT | Primary key |
| race_key | TEXT | MD5 hash (unique identifier) |
| race_date | DATE | Race date (partition key) |
| region | TEXT | GB or IRE |
| course_id | BIGINT | FK to courses |
| off_time | TIME | Race start time |
| race_name | TEXT | Official race name |
| race_type | TEXT | Flat, Hurdle, Chase, NH Flat |
| class | TEXT | 1-7 (1 = highest) |
| dist_f | DOUBLE PRECISION | Distance in furlongs |
| dist_m | INTEGER | Distance in meters |
| going | TEXT | Ground condition |
| surface | TEXT | Turf, AW, Sand |
| ran | INTEGER | Number of runners |

**Rows**: 226,397  
**Partitions**: 200+ (one per year/month)

**Unique Constraint**: `(race_key, race_date)`

### runners (partitioned by race_date)

**Purpose**: Individual horse performances with Betfair prices

| Column | Type | Description |
|--------|------|-------------|
| runner_id | BIGINT | Primary key |
| runner_key | TEXT | MD5 hash |
| race_id | BIGINT | FK to races |
| race_date | DATE | Partition key |
| horse_id | BIGINT | FK to horses |
| trainer_id | BIGINT | FK to trainers |
| jockey_id | BIGINT | FK to jockeys |
| owner_id | BIGINT | FK to owners |
| blood_id | BIGINT | FK to bloodlines |
| num | INTEGER | Runner number |
| pos_num | INTEGER | Finishing position (1, 2, 3...) |
| pos_raw | TEXT | Position string ("1", "2", "PU", "UR") |
| draw | INTEGER | Stall number |
| btn | DOUBLE PRECISION | Beaten lengths |
| age | INTEGER | Horse age |
| lbs | INTEGER | Weight carried |
| or | INTEGER | Official Rating |
| rpr | INTEGER | Racing Post Rating |
| win_bsp | DOUBLE PRECISION | Betfair Starting Price (WIN) |
| win_ppwap | DOUBLE PRECISION | Pre-play WAP (WIN) |
| place_bsp | DOUBLE PRECISION | Betfair Starting Price (PLACE) |
| dec | DOUBLE PRECISION | Decimal odds |
| win_flag | BOOLEAN | Won the race |
| comment | TEXT | Running comment |

**Plus 20+ additional Betfair fields** (morning WAP, IP max/min, volumes, etc.)

**Rows**: 2,235,311  
**Partitions**: 200+

**Unique Constraint**: `(runner_key, race_date)`

---

## 3. Dimension Tables

### courses

| Column | Type | Notes |
|--------|------|-------|
| course_id | BIGSERIAL | PK |
| course_name | TEXT | Ascot, Aintree, etc. |
| region | TEXT | GB or IRE |
| course_norm | TEXT | Generated (normalized) |

**Rows**: 89  
**Unique**: `(region, course_norm)`

### horses

| Column | Type | Notes |
|--------|------|-------|
| horse_id | BIGSERIAL | PK |
| horse_name | TEXT | Frankel (GB), etc. |
| horse_norm | TEXT | Generated |

**Rows**: 190,892  
**Unique**: `(horse_norm)`

### trainers

**Rows**: 4,361  
**Unique**: `(trainer_norm)`

### jockeys

**Rows**: 5,545  
**Unique**: `(jockey_norm)`

### owners

**Rows**: 44,447  
**Unique**: `(owner_norm)`

### bloodlines

| Column | Type | Notes |
|--------|------|-------|
| blood_id | BIGSERIAL | PK |
| sire | TEXT | Father |
| dam | TEXT | Mother |
| damsire | TEXT | Mother's father |

**Rows**: 121,368  
**Unique**: `(sire_norm, dam_norm, damsire_norm)`

---

## 4. Materialized Views

### mv_runner_base

**Purpose**: Denormalized runner data for fast profile queries

**Query**:
```sql
CREATE MATERIALIZED VIEW racing.mv_runner_base AS
SELECT 
    ru.runner_id, ru.race_id, ru.race_date,
    ru.horse_id, ru.trainer_id, ru.jockey_id,
    ru.pos_num, ru.rpr, ru."or", ru.win_flag, ru.btn,
    r.course_id, r.race_type, r.class, r.dist_f, r.going, r.surface
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.pos_num IS NOT NULL;
```

**Rows**: 2,078,144  
**Refresh**: Manual or daily cron

**Indexes**:
- `(horse_id, race_date DESC)` - For horse profiles
- `(trainer_id, race_date DESC)` - For trainer stats
- `(jockey_id, race_date DESC)` - For jockey stats
- `(race_id)` - For race lookups

**Refresh Command**:
```sql
REFRESH MATERIALIZED VIEW racing.mv_runner_base;
-- Takes ~30-60 seconds
```

---

## 5. Indexes

### Critical Indexes

**races table**:
```sql
idx_races_date (race_date)
idx_races_course (course_id)
idx_races_key (race_key) UNIQUE
```

**runners table**:
```sql
idx_runners_horse_date (horse_id, race_date DESC)
idx_runners_horse_form (horse_id, race_date DESC) INCLUDE (pos_num, rpr, win_flag, ...)
idx_runners_race (race_id)
idx_runners_trainer (trainer_id, race_date)
idx_runners_jockey (jockey_id, race_date)
```

**Search indexes** (trigram):
```sql
idx_horses_name_trgm USING gin (horse_name gin_trgm_ops)
idx_trainers_name_trgm USING gin (trainer_name gin_trgm_ops)
idx_comments_fts USING gin (to_tsvector('english', comment))
```

---

## 6. Common Queries

### Get All Races on a Date

```sql
SELECT 
    r.race_id, r.race_date, c.course_name, r.off_time,
    r.race_name, r.race_type, r.ran
FROM racing.races r
LEFT JOIN racing.courses c ON c.course_id = r.course_id
WHERE r.race_date = '2024-01-01'
ORDER BY r.off_time;
```

### Get Horse Career Stats

```sql
SELECT 
    COUNT(*) as runs,
    COUNT(*) FILTER (WHERE win_flag) as wins,
    AVG(rpr) FILTER (WHERE rpr IS NOT NULL) as avg_rpr,
    MAX(rpr) as peak_rpr
FROM racing.mv_runner_base
WHERE horse_id = 134020;
```

### Find Horses by Name (Fuzzy)

```sql
SELECT 
    horse_id, horse_name,
    similarity(horse_name, 'Franke') as score
FROM racing.horses
WHERE horse_name % 'Franke'  -- % is trigram operator
ORDER BY score DESC
LIMIT 10;
```

### Get Race Winners with Odds

```sql
SELECT 
    h.horse_name, r.race_name, r.race_date,
    ru.win_bsp, ru.dec, ru.btn
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.win_flag = true
    AND r.race_date BETWEEN '2024-01-01' AND '2024-01-31'
ORDER BY r.race_date;
```

---

## 7. Maintenance

### Daily Tasks (Automated)

```bash
# Run auto-update (happens on server startup if enabled)
AUTO_UPDATE_ON_STARTUP=true ./bin/api
```

### Weekly Tasks

```bash
# Refresh materialized views
docker exec horse_racing psql -U postgres -d horse_db -c "
REFRESH MATERIALIZED VIEW racing.mv_runner_base;"

# Vacuum database
docker exec horse_racing psql -U postgres -d horse_db -c "
VACUUM ANALYZE racing.races;
VACUUM ANALYZE racing.runners;"
```

### Monthly Tasks

```bash
# Check database size
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT pg_size_pretty(pg_database_size('horse_db'));"

# Check table sizes
docker exec horse_racing psql -U postgres -d horse_db -c "
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size('racing.' || tablename)) as size
FROM pg_tables 
WHERE schemaname = 'racing' AND tablename NOT LIKE '%_20%'
ORDER BY pg_total_relation_size('racing.' || tablename) DESC;"
```

### Backup & Restore

**Create Backup**:
```bash
docker exec horse_racing pg_dump -U postgres horse_db > postgres/db_backup_$(date +%Y%m%d).sql
# Takes ~2 minutes, creates ~920MB file
```

**Restore Backup**:
```bash
# 1. Drop and recreate database
docker exec horse_racing psql -U postgres -c "DROP DATABASE IF EXISTS horse_db;"
docker exec horse_racing psql -U postgres -c "CREATE DATABASE horse_db;"

# 2. Restore
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/db_backup.sql

# 3. Recreate materialized views
docker exec horse_racing psql -U postgres -d horse_db -c "
CREATE MATERIALIZED VIEW racing.mv_runner_base AS
SELECT ... (see section 4)
"
```

---

## Appendix: Full Schema SQL

See `postgres/init.sql` for complete schema definition including:
- All table structures
- Partition definitions
- Index definitions
- Constraint definitions
- Function definitions (`racing.norm_text`)

**To recreate from scratch**:
```bash
docker exec -i horse_racing psql -U postgres -d horse_db < postgres/init.sql
```

---

**Status**: ✅ Complete  
**Last Updated**: October 15, 2025  
**Schema Version**: 1.0

