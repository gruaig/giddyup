# PostgreSQL API Documentation for UI Developers

## 📋 Table of Contents
1. [Overview](#overview)
2. [Database Schema](#database-schema)
3. [API Endpoint Patterns](#api-endpoint-patterns)
4. [Common Queries](#common-queries)
5. [Search & Filters](#search--filters)
6. [Performance Optimization](#performance-optimization)
7. [Data Types Reference](#data-types-reference)

---

## Overview

### Connection Details
```
Host: localhost
Port: 5432
Database: horse_db
Schema: racing
User: racing_readonly (for API)
Password: [contact admin]
```

### Connection String
```
postgresql://racing_readonly:password@localhost:5432/horse_db?options=-c%20search_path=racing
```

### Available Data
- **Date Range**: 2006-01-01 to 2025-12-31
- **Regions**: GB (Great Britain), IRE (Ireland)
- **Race Types**: Flat, Hurdle, Chase, NH Flat
- **Total Races**: ~500,000+
- **Total Runners**: ~5,000,000+
- **Betfair Coverage**: 2007-01-01 onwards

---

## Database Schema

### Entity Relationship Diagram

```
┌─────────────┐
│   courses   │
│─────────────│
│ course_id PK│◄────┐
│ course_name │     │
│ region      │     │
└─────────────┘     │
                    │
┌─────────────┐     │    ┌─────────────────────────────────────┐
│   horses    │     │    │              races                  │
│─────────────│     │    │─────────────────────────────────────│
│ horse_id  PK│◄──┐ │    │ race_id        PK (partitioned)    │
│ horse_name  │   │ └────│ course_id      FK                   │
└─────────────┘   │      │ race_key       UNIQUE               │
                  │      │ race_date      (partition key)      │
┌─────────────┐   │      │ region                              │
│  trainers   │   │      │ off_time                            │
│─────────────│   │      │ race_name, race_type                │
│ trainer_id PK│◄─┤      │ going, surface, dist_f              │
│ trainer_name│   │      │ class, pattern, ran                 │
└─────────────┘   │      └─────────────┬───────────────────────┘
                  │                    │
┌─────────────┐   │                    │
│   jockeys   │   │                    │
│─────────────│   │                    ▼
│ jockey_id PK│◄─┤      ┌──────────────────────────────────────┐
│ jockey_name │   │      │           runners                    │
└─────────────┘   │      │──────────────────────────────────────│
                  │      │ runner_id      PK (partitioned)      │
┌─────────────┐   │      │ race_id        FK                    │
│   owners    │   │      │ runner_key     UNIQUE                │
│─────────────│   │      │ race_date      (partition key)       │
│ owner_id  PK│◄─┤      ├──────────────────────────────────────│
│ owner_name  │   └──────│ horse_id       FK                    │
└─────────────┘          │ trainer_id     FK                    │
                  ┌──────│ jockey_id      FK                    │
┌─────────────┐   │      │ owner_id       FK                    │
│ bloodlines  │   │      │ blood_id       FK                    │
│─────────────│   │      ├──────────────────────────────────────│
│ blood_id  PK│───┘      │ num, pos_raw, draw                   │
│ sire        │          │ age, sex, lbs                        │
│ dam         │          │ time_raw, secs, dec                  │
│ damsire     │          │ prize, or, rpr                       │
└─────────────┘          │ jockey, trainer, comment             │
                         ├──────────────────────────────────────│
                         │ Betfair WIN Market:                  │
                         │   win_bsp, win_ppwap, win_ppmax      │
                         │   win_ppmin, win_ipmax, win_ipmin    │
                         │   win_*_vol, win_lose                │
                         ├──────────────────────────────────────│
                         │ Betfair PLACE Market:                │
                         │   place_bsp, place_ppwap, etc.       │
                         ├──────────────────────────────────────│
                         │ Generated:                           │
                         │   pos_num (int from pos_raw)         │
                         │   win_flag (true if pos_num = 1)     │
                         └──────────────────────────────────────┘
```

### Table Summary

#### Dimensions (Slowly Changing)
| Table | Primary Key | Description | Row Count |
|-------|-------------|-------------|-----------|
| courses | course_id | Racing venues | ~100 |
| horses | horse_id | All horses | ~500,000 |
| trainers | trainer_id | All trainers | ~20,000 |
| jockeys | jockey_id | All jockeys | ~15,000 |
| owners | owner_id | All owners | ~200,000 |
| bloodlines | blood_id | Sire/Dam/Damsire | ~400,000 |

#### Facts (Partitioned Monthly)
| Table | Primary Key | Partition Key | Description | Row Count |
|-------|-------------|---------------|-------------|-----------|
| races | race_id, race_date | race_date | Race metadata | ~500,000 |
| runners | runner_id, race_date | race_date | Runner facts + Betfair | ~5,000,000 |

---

## API Endpoint Patterns

### Recommended REST API Structure

```
GET  /api/races                      # List races (paginated)
GET  /api/races/:race_id             # Single race details
GET  /api/races/:race_id/runners     # Race runners + markets

GET  /api/horses                     # List horses (search)
GET  /api/horses/:horse_id           # Horse profile
GET  /api/horses/:horse_id/form      # Horse race history

GET  /api/trainers                   # List trainers
GET  /api/trainers/:trainer_id       # Trainer profile
GET  /api/trainers/:trainer_id/stats # Trainer statistics

GET  /api/jockeys                    # List jockeys
GET  /api/jockeys/:jockey_id         # Jockey profile

GET  /api/search                     # Global fuzzy search
GET  /api/search/courses             # Course search
GET  /api/search/horses              # Horse search

GET  /api/analytics/market           # Market analytics
GET  /api/analytics/bias             # Draw bias analysis
```

---

## Common Queries

### 1. List Races (Paginated)

**Endpoint**: `GET /api/races?date=2024-01-13&limit=20&offset=0`

```sql
SELECT 
  r.race_id,
  r.race_key,
  r.race_date,
  r.off_time,
  r.race_name,
  r.race_type,
  c.course_name,
  r.region,
  r.going,
  r.surface,
  r.dist_f,
  r.dist_raw,
  r.class,
  r.ran
FROM races r
LEFT JOIN courses c ON c.course_id = r.course_id
WHERE r.race_date = $1  -- '2024-01-13'
ORDER BY r.off_time
LIMIT $2 OFFSET $3;  -- 20, 0
```

**Response Example**:
```json
{
  "total": 354,
  "page": 1,
  "per_page": 20,
  "data": [
    {
      "race_id": 12345,
      "race_key": "a3f5d8...",
      "race_date": "2024-01-13",
      "off_time": "14:00:00",
      "race_name": "Handicap Stakes",
      "race_type": "Flat",
      "course_name": "Chelmsford",
      "region": "GB",
      "going": "Standard",
      "distance_f": 7.0,
      "distance": "7f",
      "class": "4",
      "runners": 12
    }
  ]
}
```

---

### 2. Single Race with Runners

**Endpoint**: `GET /api/races/:race_id/runners`

```sql
WITH race_info AS (
  SELECT 
    r.*,
    c.course_name
  FROM races r
  LEFT JOIN courses c ON c.course_id = r.course_id
  WHERE r.race_id = $1  -- race_id parameter
),
runner_details AS (
  SELECT 
    ru.runner_id,
    ru.num,
    ru.pos_raw,
    ru.pos_num,
    ru.win_flag,
    ru.draw,
    h.horse_name,
    ru.age,
    ru.sex,
    ru.lbs,
    j.jockey_name,
    t.trainer_name,
    o.owner_name,
    ru.or,
    ru.rpr,
    ru.comment,
    -- Betfair WIN
    ru.win_bsp,
    ru.win_ppwap,
    ru.win_ppmax,
    ru.win_ppmin,
    ru.win_lose,
    -- Betfair PLACE
    ru.place_bsp,
    ru.place_ppwap,
    -- Derived
    (ru.win_ppmax - ru.win_ppmin) AS price_movement,
    ru.secs AS finish_time
  FROM runners ru
  LEFT JOIN horses h ON h.horse_id = ru.horse_id
  LEFT JOIN jockeys j ON j.jockey_id = ru.jockey_id
  LEFT JOIN trainers t ON t.trainer_id = ru.trainer_id
  LEFT JOIN owners o ON o.owner_id = ru.owner_id
  WHERE ru.race_id = $1
  ORDER BY ru.pos_num NULLS LAST, ru.num
)
SELECT 
  (SELECT json_build_object(
    'race_id', race_id,
    'race_name', race_name,
    'course', course_name,
    'date', race_date,
    'time', off_time,
    'distance', dist_f,
    'going', going,
    'class', class,
    'total_runners', ran
  ) FROM race_info) AS race,
  json_agg(runner_details.*) AS runners
FROM runner_details;
```

---

### 3. Horse Profile & Form

**Endpoint**: `GET /api/horses/:horse_id/form?limit=20`

```sql
SELECT 
  r.race_date,
  r.race_name,
  c.course_name,
  r.race_type,
  r.going,
  r.dist_f,
  ru.pos_num,
  ru.pos_raw,
  ru.btn,
  ru.or,
  ru.rpr,
  ru.win_bsp,
  ru.dec,
  t.trainer_name,
  j.jockey_name,
  -- Days since last run
  r.race_date - LAG(r.race_date) OVER (ORDER BY r.race_date) AS days_since_run
FROM runners ru
JOIN races r ON r.race_id = ru.race_id
LEFT JOIN courses c ON c.course_id = r.course_id
LEFT JOIN trainers t ON t.trainer_id = ru.trainer_id
LEFT JOIN jockeys j ON j.jockey_id = ru.jockey_id
WHERE ru.horse_id = $1  -- horse_id parameter
ORDER BY r.race_date DESC
LIMIT $2;  -- 20
```

---

### 4. Trainer Statistics

**Endpoint**: `GET /api/trainers/:trainer_id/stats?period=90`

```sql
WITH recent_runs AS (
  SELECT 
    ru.win_flag,
    ru.pos_num,
    r.race_type,
    r.going,
    r.class,
    c.course_name
  FROM runners ru
  JOIN races r ON r.race_id = ru.race_id
  LEFT JOIN courses c ON c.course_id = r.course_id
  WHERE ru.trainer_id = $1
    AND r.race_date >= CURRENT_DATE - INTERVAL '90 days'
)
SELECT 
  COUNT(*) AS runs,
  COUNT(*) FILTER (WHERE win_flag) AS wins,
  ROUND(COUNT(*) FILTER (WHERE win_flag)::numeric / COUNT(*) * 100, 2) AS strike_rate,
  COUNT(*) FILTER (WHERE pos_num <= 3) AS placed,
  -- By race type
  json_object_agg(
    race_type,
    json_build_object(
      'runs', COUNT(*),
      'wins', COUNT(*) FILTER (WHERE win_flag)
    )
  ) AS by_race_type
FROM recent_runs;
```

---

### 5. Global Fuzzy Search

**Endpoint**: `GET /api/search?q=frankel&limit=10`

```sql
-- Search across horses, trainers, jockeys, courses
(
  SELECT 
    'horse' AS type,
    horse_id AS id,
    horse_name AS name,
    similarity(horse_name, $1) AS score
  FROM horses
  WHERE horse_name % $1  -- Trigram similarity
  ORDER BY score DESC
  LIMIT 5
)
UNION ALL
(
  SELECT 
    'trainer' AS type,
    trainer_id AS id,
    trainer_name AS name,
    similarity(trainer_name, $1) AS score
  FROM trainers
  WHERE trainer_name % $1
  ORDER BY score DESC
  LIMIT 5
)
UNION ALL
(
  SELECT 
    'jockey' AS type,
    jockey_id AS id,
    jockey_name AS name,
    similarity(jockey_name, $1) AS score
  FROM jockeys
  WHERE jockey_name % $1
  ORDER BY score DESC
  LIMIT 5
)
ORDER BY score DESC
LIMIT $2;  -- 10
```

**Setup Required**:
```sql
SET pg_trgm.similarity_threshold = 0.3;
```

---

### 6. Market Analytics - Steamers & Drifters

**Endpoint**: `GET /api/analytics/market?date=2024-01-13&min_move=20`

```sql
SELECT 
  r.race_name,
  r.off_time,
  c.course_name,
  h.horse_name,
  ru.win_ppmax AS opening_price,
  ru.win_bsp AS bsp,
  (ru.win_ppmax - ru.win_bsp) AS drift,
  ROUND(((ru.win_bsp - ru.win_ppmax) / ru.win_ppmax * 100)::numeric, 2) AS drift_pct,
  ru.win_flag,
  ru.pos_num
FROM runners ru
JOIN races r ON r.race_id = ru.race_id
LEFT JOIN courses c ON c.course_id = r.course_id
LEFT JOIN horses h ON h.horse_id = ru.horse_id
WHERE r.race_date = $1
  AND ru.win_ppmax IS NOT NULL
  AND ru.win_bsp IS NOT NULL
  AND ABS(ru.win_ppmax - ru.win_bsp) >= ($2 / 100.0)  -- min 20% move
ORDER BY ABS(drift) DESC;
```

---

### 7. Draw Bias Analysis

**Endpoint**: `GET /api/analytics/bias?course_id=5&distance_min=5&distance_max=7`

```sql
WITH draw_stats AS (
  SELECT 
    ru.draw,
    NTILE(4) OVER (ORDER BY ru.draw) AS draw_quartile,
    COUNT(*) AS runs,
    COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
    ROUND(AVG(ru.pos_num)::numeric, 2) AS avg_position
  FROM runners ru
  JOIN races r ON r.race_id = ru.race_id
  WHERE r.course_id = $1  -- course_id
    AND r.race_type = 'Flat'
    AND r.dist_f BETWEEN $2 AND $3  -- 5f to 7f
    AND ru.draw IS NOT NULL
    AND r.ran >= 8  -- Minimum field size
  GROUP BY ru.draw
)
SELECT 
  draw_quartile,
  SUM(runs) AS total_runs,
  SUM(wins) AS total_wins,
  ROUND(SUM(wins)::numeric / SUM(runs) * 100, 2) AS win_rate,
  ROUND(AVG(avg_position), 2) AS avg_finish_pos
FROM draw_stats
GROUP BY draw_quartile
ORDER BY draw_quartile;
```

---

### 8. Head-to-Head Comparison

**Endpoint**: `GET /api/horses/compare?horse1=123&horse2=456`

```sql
WITH horse_stats AS (
  SELECT 
    ru.horse_id,
    h.horse_name,
    COUNT(*) AS runs,
    COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
    ROUND(AVG(ru.or)::numeric, 1) AS avg_or,
    ROUND(AVG(ru.rpr)::numeric, 1) AS avg_rpr,
    ROUND(AVG(ru.win_bsp)::numeric, 2) AS avg_bsp,
    MAX(r.race_date) AS last_run
  FROM runners ru
  JOIN races r ON r.race_id = ru.race_id
  LEFT JOIN horses h ON h.horse_id = ru.horse_id
  WHERE ru.horse_id IN ($1, $2)  -- horse1, horse2
  GROUP BY ru.horse_id, h.horse_name
)
SELECT 
  json_agg(horse_stats.*) AS comparison
FROM horse_stats;
```

---

## Search & Filters

### Recommended Filter Parameters

```typescript
interface RaceFilters {
  date_from?: string;      // '2024-01-01'
  date_to?: string;        // '2024-12-31'
  region?: 'GB' | 'IRE';
  race_type?: 'Flat' | 'Hurdle' | 'Chase' | 'NH Flat';
  course_id?: number;
  going?: string[];        // ['Good', 'Soft']
  class?: string[];        // ['1', '2', '3']
  min_distance?: number;   // 5.0 (furlongs)
  max_distance?: number;   // 12.0
  min_runners?: number;    // 8
  handicap?: boolean;      // race_name LIKE '%Handicap%'
}
```

### Example Filter Query

```sql
SELECT r.*, c.course_name
FROM races r
LEFT JOIN courses c ON c.course_id = r.course_id
WHERE 1=1
  AND ($1::date IS NULL OR r.race_date >= $1)
  AND ($2::date IS NULL OR r.race_date <= $2)
  AND ($3::text IS NULL OR r.region = $3)
  AND ($4::text IS NULL OR r.race_type = $4)
  AND ($5::int IS NULL OR r.course_id = $5)
  AND ($6::text[] IS NULL OR r.going = ANY($6))
  AND ($7::int IS NULL OR r.dist_f >= $7)
  AND ($8::int IS NULL OR r.dist_f <= $8)
ORDER BY r.race_date DESC, r.off_time
LIMIT 100;
```

---

## Performance Optimization

### 1. Always Use Indexes

**Indexed Columns**:
- `races(race_date)` - Primary partition key
- `races(course_id, race_date)` - Course filtering
- `races(race_type, going)` - Type/going combos
- `runners(race_id)` - Join key
- `runners(horse_id, trainer_id, jockey_id)` - Dimension lookups
- All `*_name` columns - Trigram GIN indexes

### 2. Limit Date Ranges

```sql
-- ❌ BAD: Scans all partitions
SELECT * FROM races WHERE race_type = 'Flat';

-- ✅ GOOD: Uses partition pruning
SELECT * FROM races 
WHERE race_date BETWEEN '2024-01-01' AND '2024-01-31'
  AND race_type = 'Flat';
```

### 3. Use Pagination

```sql
-- Always include LIMIT and OFFSET
SELECT * FROM races
WHERE race_date = '2024-01-13'
ORDER BY off_time
LIMIT 20 OFFSET 0;
```

### 4. Denormalize for Performance

Pre-join frequently accessed data:

```sql
CREATE MATERIALIZED VIEW mv_race_cards AS
SELECT 
  r.*,
  c.course_name,
  json_agg(
    json_build_object(
      'horse', h.horse_name,
      'jockey', j.jockey_name,
      'trainer', t.trainer_name,
      'num', ru.num,
      'draw', ru.draw,
      'win_bsp', ru.win_bsp
    )
  ) AS runners
FROM races r
LEFT JOIN courses c ON c.course_id = r.course_id
LEFT JOIN runners ru ON ru.race_id = r.race_id
LEFT JOIN horses h ON h.horse_id = ru.horse_id
LEFT JOIN jockeys j ON j.jockey_id = ru.jockey_id
LEFT JOIN trainers t ON t.trainer_id = ru.trainer_id
WHERE r.race_date >= CURRENT_DATE - INTERVAL '7 days'
GROUP BY r.race_id, c.course_name;

CREATE INDEX ON mv_race_cards(race_date);
```

Refresh daily:
```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY mv_race_cards;
```

### 5. Connection Pooling

Use a connection pool in your API:
```javascript
// Node.js example
const { Pool } = require('pg');
const pool = new Pool({
  host: 'localhost',
  database: 'horse_db',
  user: 'racing_readonly',
  password: 'password',
  max: 20,  // Max connections
  idleTimeoutMillis: 30000,
  connectionTimeoutMillis: 2000,
});
```

---

## Data Types Reference

### Race Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| race_id | BIGINT | Primary key | 123456 |
| race_key | TEXT | MD5 hash (stable) | "a3f5d8..." |
| race_date | DATE | Race date | 2024-01-13 |
| off_time | TIME | Start time | 14:30:00 |
| race_name | TEXT | Race name | "Handicap Stakes" |
| race_type | TEXT | Type | Flat, Hurdle, Chase |
| class | TEXT | Class | "1", "2", ..., "6" |
| going | TEXT | Going | Good, Soft, Heavy, Firm |
| dist_f | DOUBLE | Distance (furlongs) | 7.0, 10.0, 16.0 |
| ran | INTEGER | Number of runners | 12 |

### Runner Fields

| Field | Type | Description | Example |
|-------|------|-------------|---------|
| runner_id | BIGINT | Primary key | 987654 |
| runner_key | TEXT | MD5 hash (stable) | "b7e9f2..." |
| num | INTEGER | Cloth number | 5 |
| pos_raw | TEXT | Position | "1", "2", "UR", "PU" |
| pos_num | INTEGER | Numeric position | 1, 2, 3 (NULL if non-finisher) |
| win_flag | BOOLEAN | Won race | true/false |
| draw | INTEGER | Stall position | 3 |
| age | INTEGER | Horse age | 4 |
| sex | TEXT | Horse sex | "C", "F", "G", "H" |
| lbs | INTEGER | Weight carried | 141 |
| or | INTEGER | Official Rating | 95 |
| rpr | INTEGER | Racing Post Rating | 102 |

### Betfair Market Fields

| Field | Type | Description | NULL if... |
|-------|------|-------------|------------|
| win_bsp | DOUBLE | Betfair Starting Price (WIN) | No market/pre-2007 |
| win_ppwap | DOUBLE | Pre-play VWAP | No pre-play market |
| win_ppmax | DOUBLE | Pre-play max price | No pre-play market |
| win_ppmin | DOUBLE | Pre-play min price | No pre-play market |
| win_ipmax | DOUBLE | In-play max price | No in-play trading |
| win_ipmin | DOUBLE | In-play min price | No in-play trading |
| win_lose | INTEGER | Result (1=win, 0=lose) | Non-runner |
| place_bsp | DOUBLE | BSP (PLACE) | No place market |

**Note**: Sentinel value `1.0` has been removed. NULL means no market data.

---

## Example API Responses

### Race List Response
```json
{
  "total": 354,
  "page": 1,
  "per_page": 20,
  "filters": {
    "date": "2024-01-13",
    "region": "GB",
    "race_type": "Flat"
  },
  "data": [
    {
      "race_id": 12345,
      "race_key": "a3f5d8e7b2c1...",
      "date": "2024-01-13",
      "time": "14:00:00",
      "course": "Chelmsford",
      "race_name": "Handicap Stakes",
      "distance": "7f",
      "going": "Standard",
      "class": "4",
      "runners": 12,
      "has_betfair": true
    }
  ]
}
```

### Horse Profile Response
```json
{
  "horse_id": 123,
  "name": "Frankel",
  "stats": {
    "runs": 14,
    "wins": 14,
    "strike_rate": 100,
    "avg_or": 147,
    "avg_rpr": 142
  },
  "bloodlines": {
    "sire": "Galileo",
    "dam": "Kind",
    "damsire": "Danehill"
  },
  "recent_form": [
    {
      "date": "2012-10-20",
      "course": "Ascot",
      "race": "Champion Stakes",
      "distance": "10f",
      "pos": 1,
      "btn": 0,
      "or": 147,
      "rpr": 145,
      "bsp": 1.15
    }
  ]
}
```

---

## Security & Access Control

### Read-Only User Setup
```sql
-- Create read-only role for API
CREATE ROLE racing_readonly NOINHERIT LOGIN PASSWORD 'your_secure_password';
GRANT USAGE ON SCHEMA racing TO racing_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA racing TO racing_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT SELECT ON TABLES TO racing_readonly;
```

### Rate Limiting (Application Level)
Implement rate limiting in your API:
- **Anonymous**: 100 requests/minute
- **Authenticated**: 1000 requests/minute
- **Premium**: 10,000 requests/minute

---

## Testing Queries

### Health Check
```sql
-- Returns row count if database is healthy
SELECT 'races' AS table, COUNT(*) FROM races
UNION ALL
SELECT 'runners', COUNT(*) FROM runners;
```

### Data Coverage Check
```sql
-- Check Betfair coverage by year
SELECT 
  EXTRACT(YEAR FROM race_date) AS year,
  COUNT(*) AS total_runners,
  COUNT(win_bsp) AS with_betfair,
  ROUND(COUNT(win_bsp)::numeric / COUNT(*) * 100, 2) AS coverage_pct
FROM runners
GROUP BY year
ORDER BY year DESC;
```

---

## Support & Contact

- **Database Issues**: Contact DevOps team
- **Schema Questions**: See `database.md`
- **API Design**: Contact API team lead
- **Performance**: Review indexes in `init_clean.sql`

---

**Last Updated**: 2025-10-13  
**Schema Version**: 1.0  
**Maintainer**: Data Engineering Team

