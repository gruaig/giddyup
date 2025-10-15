# Backend Developer Guide - Racing Data API

## 🎯 Overview

Build a REST/GraphQL API to serve the **30 racing analytics features** using the PostgreSQL `horse_db` database. Language choice is flexible (Python/FastAPI, Node.js/Express, Go, etc.).

---

## 📊 Database Reference

**Primary Documentation:**
- **Schema**: `postgres/database.md` (complete DDL, indexes, views)
- **API Examples**: `postgres/API_DOCUMENTATION.md` (query patterns)

**Connection Details:**
```
Database: horse_db
Schema: racing
Host: localhost (or production host)
Port: 5432
User: racing_api (read-only recommended)

IMPORTANT: All connections must set search_path:
SET search_path TO racing, public;
```

**Key Tables:**
- `racing.races` - Race metadata (partitioned by month, 2007-2025)
- `racing.runners` - Runner facts + Betfair data (partitioned by month)
- `racing.horses`, `racing.trainers`, `racing.jockeys` - Dimension tables
- `racing.courses`, `racing.owners`, `racing.bloodlines` - Dimension tables

**Verify Schema & Data:**
```sql
-- Connect and set search_path
\c horse_db
SET search_path TO racing, public;

-- List all tables (should see dimensions + partitioned races/runners)
\dt

-- Verify data is loaded (Expected counts as of Oct 2025):
SELECT COUNT(*) FROM races;     -- 168,070 races
SELECT COUNT(*) FROM runners;   -- 1,610,337 runners
SELECT COUNT(*) FROM horses;    -- 141,196 horses
SELECT COUNT(*) FROM trainers;  -- 3,659 trainers
SELECT COUNT(*) FROM jockeys;   -- 4,231 jockeys
SELECT COUNT(*) FROM courses;   -- 89 courses
```

---

## 🏗️ API Architecture Recommendations

### Option A: REST API
- **Framework**: FastAPI (Python), Express (Node.js), Gin (Go)
- **Endpoints**: `/api/v1/{resource}`
- **Pagination**: Cursor-based for large datasets
- **Caching**: Redis for hot queries (profiles, recent races)

### Option B: GraphQL
- **Framework**: Apollo Server, Hasura, PostGraphile
- **Benefits**: Client-driven queries, fewer endpoints
- **Considerations**: N+1 query optimization needed

### Hybrid Approach (Recommended)
- **REST** for standard CRUD and search
- **GraphQL** for complex, nested queries (profiles with splits)
- **WebSocket** for live updates (future racecards)

---

## 🔐 Security & Access

### Read-Only Database User
```sql
-- Create API user with read-only access
CREATE USER racing_api WITH PASSWORD 'secure_password';
GRANT USAGE ON SCHEMA racing TO racing_api;
GRANT SELECT ON ALL TABLES IN SCHEMA racing TO racing_api;
GRANT SELECT ON ALL SEQUENCES IN SCHEMA racing TO racing_api;
ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT SELECT ON TABLES TO racing_api;

-- Set default search_path for this user
ALTER USER racing_api SET search_path TO racing, public;
```

### Connection Setup (Python Example)
```python
import psycopg2

def get_db_connection():
    conn = psycopg2.connect(
        host='localhost',
        port=5432,
        database='horse_db',
        user='racing_api',
        password='secure_password'
    )
    # Always set search_path
    with conn.cursor() as cur:
        cur.execute("SET search_path TO racing, public;")
    return conn
```

### Connection Setup (Node.js Example)
```javascript
const { Pool } = require('pg');

const pool = new Pool({
  host: 'localhost',
  port: 5432,
  database: 'horse_db',
  user: 'racing_api',
  password: 'secure_password',
});

// Set search_path for all connections
pool.on('connect', (client) => {
  client.query('SET search_path TO racing, public;');
});
```

### API Authentication
- **Public endpoints**: Search, race explorer (rate-limited)
- **Protected endpoints**: Exports, saved queries, watchlists (JWT/OAuth)
- **Admin endpoints**: Data quality console, SQL console (admin-only)

---

## 📋 Feature Implementation Guide

### **1. Search & Navigation**

#### Feature 1: Global Search Bar
**Endpoint**: `GET /api/v1/search`

**Query Parameters:**
- `q` (required): Search term
- `type`: Filter by `horse|trainer|jockey|owner|course|race`
- `limit`: Results per type (default: 10)

**Database Query (using trigram similarity):**
```sql
-- Horses
SELECT horse_id, horse_name, similarity(horse_name, $1) as score
FROM racing.horses
WHERE horse_name % $1  -- Trigram match
ORDER BY score DESC
LIMIT 10;

-- Trainers
SELECT trainer_id, trainer_name, similarity(trainer_name, $1) as score
FROM racing.trainers
WHERE trainer_name % $1
ORDER BY score DESC
LIMIT 10;

-- Similar for jockeys, owners, courses
```

**Response Format:**
```json
{
  "horses": [
    {"id": 123, "name": "Frankel", "score": 0.95}
  ],
  "trainers": [...],
  "jockeys": [...],
  "total_results": 15
}
```

---

#### Feature 2: Advanced Text Search in Comments
**Endpoint**: `GET /api/v1/search/comments`

**Query Parameters:**
- `q`: Search phrase
- `date_from`, `date_to`: Date range
- `region`, `type`, `course`: Filters

**Database Query (FTS):**
```sql
SELECT r.race_id, r.race_date, r.course_id, c.course_name,
       ru.runner_id, h.horse_name, ru.comment,
       ts_rank(ru.comment_fts, plainto_tsquery('english', $1)) as rank
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
JOIN racing.courses c ON c.course_id = r.course_id
WHERE ru.comment_fts @@ plainto_tsquery('english', $1)
  AND r.race_date BETWEEN $2 AND $3
ORDER BY rank DESC, r.race_date DESC
LIMIT 100;
```

---

#### Feature 3: Course & Meeting Pages
**Endpoint**: `GET /api/v1/courses/{course_id}/meetings`

**Database Query:**
```sql
SELECT 
  r.race_date,
  r.meeting_key,
  COUNT(*) as race_count,
  MIN(r.off_time) as first_race,
  MAX(r.off_time) as last_race,
  SUM(r.ran) as total_runners
FROM racing.races r
WHERE r.course_id = $1
  AND r.race_date BETWEEN $2 AND $3
GROUP BY r.race_date, r.meeting_key
ORDER BY r.race_date DESC;
```

---

### **2. Profiles**

#### Feature 4: Horse Profile
**Endpoint**: `GET /api/v1/horses/{horse_id}/profile`

**Sections to Return:**
1. **Career Summary**: Total runs, wins, places, earnings
2. **Recent Form**: Last 10 runs with details
3. **Splits by Going**: SR/ROI per going type
4. **Splits by Distance**: Performance by distance bands
5. **Splits by Course**: Course-specific stats
6. **RPR/OR Trends**: Time-series chart data

**Career Summary Query:**
```sql
SELECT 
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE win_flag) as wins,
  COUNT(*) FILTER (WHERE pos_num <= 3) as places,
  SUM(prize) as total_prize,
  AVG(rpr) FILTER (WHERE rpr IS NOT NULL) as avg_rpr,
  MAX(rpr) as peak_rpr,
  AVG("or") FILTER (WHERE "or" IS NOT NULL) as avg_or
FROM racing.runners
WHERE horse_id = $1;
```

**Going Splits Query:**
```sql
SELECT 
  r.going,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) as sr,
  SUM(CASE WHEN ru.win_bsp > 0 THEN (ru.win_bsp - 1) * ru.win_lose ELSE 0 END) as pl,
  SUM(CASE WHEN ru.win_bsp > 0 THEN ABS(ru.win_lose) ELSE 0 END) as staked,
  ROUND(100.0 * SUM(CASE WHEN ru.win_bsp > 0 THEN (ru.win_bsp - 1) * ru.win_lose ELSE 0 END) / 
        NULLIF(SUM(CASE WHEN ru.win_bsp > 0 THEN ABS(ru.win_lose) ELSE 0 END), 0), 2) as roi
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1
GROUP BY r.going
ORDER BY runs DESC;
```

**Distance Splits Query:**
```sql
-- Distance bands: 5-6f, 7-8f, 9-12f, 13f+
SELECT 
  CASE 
    WHEN r.dist_f < 7 THEN '5-6f'
    WHEN r.dist_f < 9 THEN '7-8f'
    WHEN r.dist_f < 13 THEN '9-12f'
    ELSE '13f+'
  END as dist_band,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  AVG(ru.rpr) FILTER (WHERE ru.rpr IS NOT NULL) as avg_rpr
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1 AND r.dist_f IS NOT NULL
GROUP BY dist_band
ORDER BY MIN(r.dist_f);
```

**RPR Trend Query:**
```sql
SELECT 
  r.race_date,
  ru.rpr,
  ru."or",
  r.class,
  ru.win_flag
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1
  AND (ru.rpr IS NOT NULL OR ru."or" IS NOT NULL)
ORDER BY r.race_date DESC
LIMIT 20;
```

---

#### Feature 5: Trainer Profile
**Endpoint**: `GET /api/v1/trainers/{trainer_id}/profile`

**Rolling Form Query (14/30/90 days):**
```sql
WITH recent_runs AS (
  SELECT 
    ru.runner_id,
    r.race_date,
    ru.win_flag,
    CASE WHEN ru.win_bsp > 0 THEN (ru.win_bsp - 1) * ru.win_lose ELSE 0 END as pl
  FROM racing.runners ru
  JOIN racing.races r ON r.race_id = ru.race_id
  WHERE ru.trainer_id = $1
    AND r.race_date >= CURRENT_DATE - INTERVAL '90 days'
)
SELECT 
  '14d' as period,
  COUNT(*) FILTER (WHERE race_date >= CURRENT_DATE - INTERVAL '14 days') as runs,
  COUNT(*) FILTER (WHERE race_date >= CURRENT_DATE - INTERVAL '14 days' AND win_flag) as wins,
  SUM(pl) FILTER (WHERE race_date >= CURRENT_DATE - INTERVAL '14 days') as pl
FROM recent_runs
UNION ALL
SELECT '30d', COUNT(*) FILTER (WHERE race_date >= CURRENT_DATE - INTERVAL '30 days'), ...
UNION ALL
SELECT '90d', COUNT(*), COUNT(*) FILTER (WHERE win_flag), SUM(pl) FROM recent_runs;
```

**Course Splits Query:**
```sql
SELECT 
  c.course_name,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) as sr
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
WHERE ru.trainer_id = $1
GROUP BY c.course_id, c.course_name
HAVING COUNT(*) >= 10
ORDER BY sr DESC;
```

---

#### Feature 6: Jockey Profile
**Endpoint**: `GET /api/v1/jockeys/{jockey_id}/profile`

**Trainer Combo Query:**
```sql
SELECT 
  t.trainer_name,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) as sr,
  SUM(CASE WHEN ru.win_bsp > 0 THEN (ru.win_bsp - 1) * ru.win_lose ELSE 0 END) / 
    NULLIF(SUM(CASE WHEN ru.win_bsp > 0 THEN ABS(ru.win_lose) ELSE 0 END), 0) * 100 as roi
FROM racing.runners ru
JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
WHERE ru.jockey_id = $1
GROUP BY t.trainer_id, t.trainer_name
HAVING COUNT(*) >= 5
ORDER BY sr DESC;
```

---

#### Feature 7: Sire/Dam Explorer
**Endpoint**: `GET /api/v1/bloodlines/{bloodline_id}/progeny`

**Progeny Performance Query:**
```sql
SELECT 
  h.horse_name,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  SUM(ru.prize) as earnings,
  MAX(ru.rpr) as peak_rpr
FROM racing.runners ru
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.blood_id = $1
GROUP BY h.horse_id, h.horse_name
ORDER BY earnings DESC NULLS LAST;
```

**Progeny Splits by Distance:**
```sql
SELECT 
  CASE 
    WHEN r.dist_f < 7 THEN '5-6f'
    WHEN r.dist_f < 9 THEN '7-8f'
    WHEN r.dist_f < 13 THEN '9-12f'
    ELSE '13f+'
  END as dist_band,
  COUNT(DISTINCT ru.horse_id) as horses,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.blood_id = $1 AND r.dist_f IS NOT NULL
GROUP BY dist_band;
```

---

### **3. Race Exploration**

#### Feature 8: Race Explorer
**Endpoint**: `GET /api/v1/races/search`

**Query Parameters:**
- `date_from`, `date_to`: Date range
- `region`: GB/IRE
- `course_id`: Specific course
- `type`: flat/jumps
- `class`: 1-7
- `pattern`: 'Group 1', 'Listed', etc.
- `handicap`: true/false
- `dist_min`, `dist_max`: Distance range (furlongs)
- `going`: Heavy, Soft, Good, etc.
- `surface`: Turf, AW
- `field_min`, `field_max`: Runner count range

**Database Query:**
```sql
SELECT 
  r.race_id,
  r.race_date,
  r.off_time,
  c.course_name,
  r.race_name,
  r.race_type,
  r.class,
  r.pattern,
  r.dist_f,
  r.going,
  r.surface,
  r.ran,
  r.race_key
FROM racing.races r
JOIN racing.courses c ON c.course_id = r.course_id
WHERE r.race_date BETWEEN $1 AND $2
  AND ($3::text IS NULL OR r.region = $3)
  AND ($4::bigint IS NULL OR r.course_id = $4)
  AND ($5::text IS NULL OR r.race_type = $5)
  AND ($6::text IS NULL OR r.class = $6)
  AND ($7::text IS NULL OR r.pattern = $7)
  AND ($8::boolean IS NULL OR r.is_handicap = $8)
  AND ($9::float IS NULL OR r.dist_f >= $9)
  AND ($10::float IS NULL OR r.dist_f <= $10)
  AND ($11::text IS NULL OR r.going ILIKE '%' || $11 || '%')
  AND ($12::text IS NULL OR r.surface = $12)
  AND ($13::int IS NULL OR r.ran >= $13)
  AND ($14::int IS NULL OR r.ran <= $14)
ORDER BY r.race_date DESC, r.off_time
LIMIT 100 OFFSET $15;
```

---

#### Feature 9: Per-Race Dashboard
**Endpoint**: `GET /api/v1/races/{race_id}`

**Race Details with Runners:**
```sql
-- Race header
SELECT r.*, c.course_name, c.region
FROM racing.races r
JOIN racing.courses c ON c.course_id = r.course_id
WHERE r.race_id = $1;

-- Runners with all details
SELECT 
  ru.runner_id,
  ru.num,
  ru.draw,
  h.horse_name,
  ru.age,
  ru.sex,
  ru.lbs,
  ru."or",
  ru.rpr,
  t.trainer_name,
  j.jockey_name,
  o.owner_name,
  ru.pos_raw,
  ru.pos_num,
  ru.btn,
  ru.comment,
  ru.win_bsp,
  ru.win_ppwap,
  ru.win_ppmax,
  ru.win_ppmin,
  ru.place_bsp,
  ru.dec,
  bl.sire,
  bl.dam
FROM racing.runners ru
JOIN racing.horses h ON h.horse_id = ru.horse_id
LEFT JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
LEFT JOIN racing.jockeys j ON j.jockey_id = ru.jockey_id
LEFT JOIN racing.owners o ON o.owner_id = ru.owner_id
LEFT JOIN racing.bloodlines bl ON bl.blood_id = ru.blood_id
WHERE ru.race_id = $1
ORDER BY ru.pos_num NULLS LAST, ru.num;
```

---

#### Feature 10: Head-to-Head
**Endpoint**: `GET /api/v1/races/{race_id}/head-to-head`

**Previous Meetings Query:**
```sql
-- Find all races where these horses met
WITH target_horses AS (
  SELECT DISTINCT horse_id 
  FROM racing.runners 
  WHERE race_id = $1
)
SELECT 
  r.race_id,
  r.race_date,
  c.course_name,
  r.dist_f,
  r.going,
  json_agg(json_build_object(
    'horse', h.horse_name,
    'pos', ru.pos_num,
    'btn', ru.btn,
    'or', ru."or"
  ) ORDER BY ru.pos_num NULLS LAST) as runners
FROM racing.runners ru
JOIN racing.horses h ON h.horse_id = ru.horse_id
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
WHERE ru.horse_id IN (SELECT horse_id FROM target_horses)
  AND ru.race_id != $1
GROUP BY r.race_id, r.race_date, c.course_name, r.dist_f, r.going
HAVING COUNT(DISTINCT ru.horse_id) >= 2  -- At least 2 horses from target race
ORDER BY r.race_date DESC
LIMIT 20;
```

---

### **4. Market & Value Analytics**

#### Feature 11: Steamers & Drifters
**Endpoint**: `GET /api/v1/market/movers`

**Query Parameters:**
- `date`: Race date
- `min_move`: Minimum % move (default: 20%)
- `type`: steamer/drifter

**Database Query:**
```sql
SELECT 
  r.race_id,
  r.race_date,
  r.off_time,
  c.course_name,
  r.race_name,
  h.horse_name,
  ru.win_ppmax as morning_price,
  ru.win_bsp as bsp,
  ROUND(100.0 * (ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0), 2) as move_pct,
  ru.win_flag,
  ru.pos_num
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE r.race_date = $1
  AND ru.win_ppmax > 0 
  AND ru.win_bsp > 0
  AND ru.win_ppmax != ru.win_bsp
  AND ABS((ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0)) >= $2 / 100.0
  AND CASE 
        WHEN $3 = 'steamer' THEN ru.win_bsp < ru.win_ppmax
        WHEN $3 = 'drifter' THEN ru.win_bsp > ru.win_ppmax
        ELSE true
      END
ORDER BY ABS((ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0)) DESC;
```

---

#### Feature 12: Market Calibration (Win)
**Endpoint**: `GET /api/v1/market/calibration/win`

**BSP Bins Query:**
```sql
WITH bsp_bins AS (
  SELECT 
    CASE 
      WHEN win_bsp < 2 THEN '1.0-2.0'
      WHEN win_bsp < 3 THEN '2.0-3.0'
      WHEN win_bsp < 5 THEN '3.0-5.0'
      WHEN win_bsp < 10 THEN '5.0-10.0'
      WHEN win_bsp < 20 THEN '10.0-20.0'
      ELSE '20.0+'
    END as price_bin,
    win_bsp,
    win_flag
  FROM racing.runners
  WHERE win_bsp > 0
    AND race_date BETWEEN $1 AND $2
)
SELECT 
  price_bin,
  COUNT(*) as runners,
  COUNT(*) FILTER (WHERE win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*), 2) as actual_sr,
  ROUND(100.0 / AVG(win_bsp), 2) as implied_sr,
  ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*) - 100.0 / AVG(win_bsp), 2) as edge
FROM bsp_bins
GROUP BY price_bin
ORDER BY MIN(win_bsp);
```

---

#### Feature 13: Book vs Exchange
**Endpoint**: `GET /api/v1/market/book-vs-exchange`

**Database Query:**
```sql
SELECT 
  r.race_date,
  COUNT(*) as races,
  AVG(ru.dec) FILTER (WHERE ru.dec IS NOT NULL AND ru.win_flag) as avg_sp_winner,
  AVG(ru.win_bsp) FILTER (WHERE ru.win_bsp IS NOT NULL AND ru.win_flag) as avg_bsp_winner,
  SUM((ru.dec - 1) * CASE WHEN ru.win_flag THEN 1 ELSE -1 END) as sp_pl,
  SUM((ru.win_bsp - 1) * ru.win_lose) FILTER (WHERE ru.win_bsp > 0) as bsp_pl
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.race_date BETWEEN $1 AND $2
  AND ru.dec IS NOT NULL
GROUP BY r.race_date
ORDER BY r.race_date;
```

---

#### Feature 14: Place Market Efficiency
**Endpoint**: `GET /api/v1/market/place-efficiency`

**Place Calibration Query:**
```sql
WITH place_bins AS (
  SELECT 
    CASE 
      WHEN place_bsp < 1.5 THEN '1.0-1.5'
      WHEN place_bsp < 2.0 THEN '1.5-2.0'
      WHEN place_bsp < 3.0 THEN '2.0-3.0'
      WHEN place_bsp < 5.0 THEN '3.0-5.0'
      ELSE '5.0+'
    END as price_bin,
    place_bsp,
    CASE WHEN pos_num <= 3 THEN 1 ELSE 0 END as placed  -- Adjust for each way terms
  FROM racing.runners
  WHERE place_bsp > 0
    AND race_date BETWEEN $1 AND $2
)
SELECT 
  price_bin,
  COUNT(*) as runners,
  SUM(placed) as places,
  ROUND(100.0 * SUM(placed) / COUNT(*), 2) as actual_place_rate,
  ROUND(100.0 / AVG(place_bsp), 2) as implied_place_rate
FROM place_bins
GROUP BY price_bin
ORDER BY MIN(place_bsp);
```

---

#### Feature 15: In-Play Collapse/Surge
**Endpoint**: `GET /api/v1/market/inplay-moves`

**Database Query:**
```sql
SELECT 
  r.race_id,
  r.race_date,
  c.course_name,
  r.race_name,
  h.horse_name,
  ru.win_bsp,
  ru.win_ipmin as ip_low,
  ru.win_ipmax as ip_high,
  ROUND(100.0 * (ru.win_ipmin - ru.win_bsp) / NULLIF(ru.win_bsp, 0), 2) as surge_pct,
  ROUND(100.0 * (ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0), 2) as collapse_pct,
  ru.win_flag,
  ru.pos_num
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE r.race_date BETWEEN $1 AND $2
  AND ru.win_bsp > 0
  AND (ru.win_ipmin > 0 OR ru.win_ipmax > 0)
  AND (ABS((ru.win_ipmin - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) > 0.2 
       OR ABS((ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) > 0.2)
ORDER BY ABS((ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) DESC;
```

---

### **5. Bias & Form**

#### Feature 16: Draw Bias Analyzer
**Endpoint**: `GET /api/v1/bias/draw`

**Query Parameters:**
- `course_id`: Required
- `dist_min`, `dist_max`: Distance range
- `going`: Optional filter
- `min_runners`: Minimum field size (default: 10)

**Database Query:**
```sql
WITH draw_stats AS (
  SELECT 
    ru.draw,
    r.ran as field_size,
    r.going,
    COUNT(*) as runs,
    COUNT(*) FILTER (WHERE ru.pos_num <= 3) as top3,
    COUNT(*) FILTER (WHERE ru.win_flag) as wins,
    AVG(ru.pos_num) FILTER (WHERE ru.pos_num IS NOT NULL) as avg_pos
  FROM racing.runners ru
  JOIN racing.races r ON r.race_id = ru.race_id
  WHERE r.course_id = $1
    AND r.race_type = 'flat'
    AND r.ran >= $2
    AND ($3::float IS NULL OR r.dist_f >= $3)
    AND ($4::float IS NULL OR r.dist_f <= $4)
    AND ($5::text IS NULL OR r.going ILIKE '%' || $5 || '%')
    AND ru.draw IS NOT NULL
  GROUP BY ru.draw, r.ran, r.going
)
SELECT 
  draw,
  SUM(runs) as total_runs,
  ROUND(100.0 * SUM(wins) / NULLIF(SUM(runs), 0), 2) as win_rate,
  ROUND(100.0 * SUM(top3) / NULLIF(SUM(runs), 0), 2) as top3_rate,
  ROUND(AVG(avg_pos), 2) as avg_position,
  json_object_agg(going, json_build_object(
    'runs', runs,
    'wins', wins,
    'avg_pos', ROUND(avg_pos, 2)
  )) as going_splits
FROM draw_stats
GROUP BY draw
ORDER BY draw;
```

---

#### Feature 17: Form & Splits Panel
**Endpoint**: `GET /api/v1/horses/{horse_id}/splits`

**Distance Buckets:**
```sql
SELECT 
  CASE 
    WHEN r.dist_f < 6 THEN '5f'
    WHEN r.dist_f < 7 THEN '6f'
    WHEN r.dist_f < 8 THEN '7f'
    WHEN r.dist_f < 10 THEN '8-9f'
    WHEN r.dist_f < 12 THEN '10-11f'
    WHEN r.dist_f < 14 THEN '12-13f'
    WHEN r.dist_f < 16 THEN '14-15f'
    ELSE '16f+'
  END as dist_bucket,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  AVG(ru.rpr) FILTER (WHERE ru.rpr IS NOT NULL) as avg_rpr
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1 AND r.dist_f IS NOT NULL
GROUP BY dist_bucket
ORDER BY MIN(r.dist_f);
```

**Going Buckets:**
```sql
SELECT 
  CASE 
    WHEN r.going ILIKE '%heavy%' THEN 'Heavy'
    WHEN r.going ILIKE '%soft%' THEN 'Soft'
    WHEN r.going ILIKE '%good to soft%' THEN 'Good to Soft'
    WHEN r.going ILIKE '%good to firm%' THEN 'Good to Firm'
    WHEN r.going ILIKE '%firm%' THEN 'Firm'
    ELSE 'Good'
  END as going_bucket,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1
GROUP BY going_bucket;
```

**Class Buckets:**
```sql
SELECT 
  r.class,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  AVG(ru.rpr) FILTER (WHERE ru.rpr IS NOT NULL) as avg_rpr
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1 AND r.class IS NOT NULL
GROUP BY r.class
ORDER BY r.class;
```

---

#### Feature 18: Recency Effects (Days Since Run)
**Endpoint**: `GET /api/v1/analysis/recency`

**DSR Buckets Query:**
```sql
WITH runs_with_dsr AS (
  SELECT 
    ru.runner_id,
    r.race_date,
    ru.horse_id,
    ru.win_flag,
    ru.pos_num,
    LAG(r.race_date) OVER (PARTITION BY ru.horse_id ORDER BY r.race_date) as prev_run_date,
    r.race_date - LAG(r.race_date) OVER (PARTITION BY ru.horse_id ORDER BY r.race_date) as dsr
  FROM racing.runners ru
  JOIN racing.races r ON r.race_id = ru.race_id
  WHERE r.race_date BETWEEN $1 AND $2
)
SELECT 
  CASE 
    WHEN dsr < 14 THEN '0-13 days'
    WHEN dsr < 28 THEN '14-27 days'
    WHEN dsr < 56 THEN '28-55 days'
    WHEN dsr < 90 THEN '56-89 days'
    WHEN dsr < 180 THEN '90-179 days'
    ELSE '180+ days'
  END as dsr_bucket,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*), 2) as sr,
  ROUND(AVG(pos_num) FILTER (WHERE pos_num IS NOT NULL), 2) as avg_pos
FROM runs_with_dsr
WHERE dsr IS NOT NULL
GROUP BY dsr_bucket
ORDER BY MIN(dsr);
```

---

#### Feature 19: Trainer Change Impact
**Endpoint**: `GET /api/v1/analysis/trainer-change`

**Database Query:**
```sql
WITH trainer_changes AS (
  SELECT 
    ru.horse_id,
    r.race_date,
    ru.trainer_id,
    LAG(ru.trainer_id) OVER (PARTITION BY ru.horse_id ORDER BY r.race_date) as prev_trainer_id,
    ru.win_flag,
    ru.pos_num,
    ru.rpr
  FROM racing.runners ru
  JOIN racing.races r ON r.race_id = ru.race_id
)
SELECT 
  h.horse_name,
  t_old.trainer_name as old_trainer,
  t_new.trainer_name as new_trainer,
  COUNT(*) FILTER (WHERE prev_trainer_id IS NULL) as runs_before,
  COUNT(*) FILTER (WHERE prev_trainer_id = trainer_id) as runs_after,
  AVG(rpr) FILTER (WHERE prev_trainer_id != trainer_id AND prev_trainer_id IS NOT NULL) as avg_rpr_before_change,
  AVG(rpr) FILTER (WHERE prev_trainer_id = trainer_id) as avg_rpr_after_change
FROM trainer_changes tc
JOIN racing.horses h ON h.horse_id = tc.horse_id
LEFT JOIN racing.trainers t_old ON t_old.trainer_id = tc.prev_trainer_id
LEFT JOIN racing.trainers t_new ON t_new.trainer_id = tc.trainer_id
WHERE tc.prev_trainer_id IS NOT NULL 
  AND tc.prev_trainer_id != tc.trainer_id
GROUP BY h.horse_id, h.horse_name, t_old.trainer_name, t_new.trainer_name
HAVING COUNT(*) >= 5;
```

---

#### Feature 20: Jumping Incidents
**Endpoint**: `GET /api/v1/jumps/incidents`

**Database Query:**
```sql
SELECT 
  r.race_id,
  r.race_date,
  c.course_name,
  r.race_name,
  h.horse_name,
  j.jockey_name,
  t.trainer_name,
  ru.pos_raw,
  ru.comment,
  CASE ru.pos_raw
    WHEN 'F' THEN 'Fell'
    WHEN 'UR' THEN 'Unseated Rider'
    WHEN 'PU' THEN 'Pulled Up'
    WHEN 'BD' THEN 'Brought Down'
    WHEN 'RR' THEN 'Refused to Race'
    WHEN 'CO' THEN 'Carried Out'
    ELSE 'Other'
  END as incident_type
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
LEFT JOIN racing.jockeys j ON j.jockey_id = ru.jockey_id
LEFT JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
WHERE r.race_type = 'jumps'
  AND ru.pos_raw IN ('F', 'UR', 'PU', 'BD', 'RR', 'CO')
  AND r.race_date BETWEEN $1 AND $2
ORDER BY r.race_date DESC;
```

---

### **6. Workflow & Operations**

#### Feature 21: Watchlists
**Application Tables (not in racing schema):**
```sql
-- App schema (separate from racing)
CREATE TABLE app.watchlists (
  watchlist_id SERIAL PRIMARY KEY,
  user_id INT NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE app.watchlist_items (
  item_id SERIAL PRIMARY KEY,
  watchlist_id INT REFERENCES app.watchlists,
  entity_type TEXT NOT NULL,  -- 'horse', 'trainer', 'jockey'
  entity_id BIGINT NOT NULL,
  added_at TIMESTAMPTZ DEFAULT NOW()
);
```

**API Endpoints:**
- `POST /api/v1/watchlists` - Create watchlist
- `GET /api/v1/watchlists` - List user's watchlists
- `POST /api/v1/watchlists/{id}/items` - Add item
- `GET /api/v1/watchlists/{id}/performance` - Get performance of items

**Performance Query:**
```sql
-- Get recent performance of horses in watchlist
SELECT 
  h.horse_name,
  COUNT(*) as recent_runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  MAX(r.race_date) as last_run,
  MAX(ru.rpr) as peak_rpr_14d
FROM app.watchlist_items wi
JOIN racing.horses h ON h.horse_id = wi.entity_id
JOIN racing.runners ru ON ru.horse_id = h.horse_id
JOIN racing.races r ON r.race_id = ru.race_id
WHERE wi.watchlist_id = $1
  AND wi.entity_type = 'horse'
  AND r.race_date >= CURRENT_DATE - INTERVAL '14 days'
GROUP BY h.horse_id, h.horse_name;
```

---

#### Feature 22: Saved Queries
**Application Tables:**
```sql
CREATE TABLE app.saved_queries (
  query_id SERIAL PRIMARY KEY,
  user_id INT NOT NULL,
  name TEXT NOT NULL,
  endpoint TEXT NOT NULL,
  params JSONB NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
);
```

**API Endpoints:**
- `POST /api/v1/saved-queries` - Save a query
- `GET /api/v1/saved-queries` - List saved queries
- `POST /api/v1/saved-queries/{id}/execute` - Re-run saved query

---

#### Feature 23: Exports
**Endpoint**: `POST /api/v1/export`

**Request Body:**
```json
{
  "query_type": "race_search",
  "params": {...},
  "format": "csv",  // or "parquet"
  "email": "user@example.com"  // Optional: async export
}
```

**Implementation:**
- For CSV: Stream results directly with `text/csv` content-type
- For Parquet: Use Arrow/PyArrow to write binary format
- Large exports: Queue job, email download link when ready

---

#### Feature 24: Data Quality Console
**Endpoint**: `GET /api/v1/admin/data-quality`

**Manifest Summary:**
```sql
-- Aggregate manifest data from master/ directory
-- This would read manifest.json files and surface:
SELECT 
  region,
  race_type,
  year_month,
  race_count,
  runner_count,
  matched_count,
  unmatched_count,
  validation_status
FROM master_manifests  -- Application table tracking manifests
ORDER BY year_month DESC;
```

**Unmatched Diagnostics:**
```sql
-- Surface unmatched races from master/unmatched_*.csv files
SELECT 
  date,
  off,
  event_name,
  best_jaccard,
  time_diff_min,
  reason
FROM master_unmatched  -- Application table with unmatched data
WHERE date >= CURRENT_DATE - INTERVAL '30 days'
ORDER BY date DESC;
```

---

#### Feature 25: SQL Console (Read-Only)
**Endpoint**: `POST /api/v1/admin/query`

**Security:**
- Create read-only DB user
- Parse and validate SQL (only SELECT allowed)
- Add query timeout (30 seconds)
- Rate limit per user
- Log all queries for audit

**Implementation:**
```python
# Pseudocode
def execute_readonly_query(sql: str, user_id: int):
    # Validate
    if not sql.strip().upper().startswith('SELECT'):
        raise ValueError("Only SELECT queries allowed")
    
    # Add safety limits
    safe_sql = f"SET statement_timeout = 30000; {sql} LIMIT 1000"
    
    # Execute with read-only user
    with readonly_connection() as conn:
        result = conn.execute(safe_sql)
        return result.fetchall()
```

---

### **7. Additional Analytics**

#### Feature 26: Pace/Running Style Tags
**Endpoint**: `GET /api/v1/analysis/pace`

**Comment Parsing for Style:**
```sql
SELECT 
  h.horse_name,
  COUNT(*) FILTER (WHERE ru.comment ILIKE '%led%' OR ru.comment ILIKE '%front%') as led_count,
  COUNT(*) FILTER (WHERE ru.comment ILIKE '%prominent%' OR ru.comment ILIKE '%chased%') as prominent_count,
  COUNT(*) FILTER (WHERE ru.comment ILIKE '%held up%' OR ru.comment ILIKE '%behind%') as held_up_count,
  CASE 
    WHEN COUNT(*) FILTER (WHERE ru.comment ILIKE '%led%' OR ru.comment ILIKE '%front%') > 
         COUNT(*) * 0.5 THEN 'Front Runner'
    WHEN COUNT(*) FILTER (WHERE ru.comment ILIKE '%prominent%') > 
         COUNT(*) * 0.4 THEN 'Prominent'
    ELSE 'Hold Up'
  END as running_style
FROM racing.runners ru
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE ru.horse_id = $1
  AND ru.comment IS NOT NULL
GROUP BY h.horse_id, h.horse_name;
```

**SR/ROI by Style:**
```sql
-- Requires pre-computed running_style column or view
SELECT 
  running_style,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*), 2) as sr
FROM racing.runners_with_style  -- View with style classification
GROUP BY running_style;
```

---

#### Feature 27: Market Movement Analyzer
**Endpoint**: `GET /api/v1/market/movement-analysis`

**Pre-play Metrics:**
```sql
SELECT 
  r.race_id,
  r.race_date,
  c.course_name,
  h.horse_name,
  ru.win_ppmax,
  ru.win_ppmin,
  ru.win_bsp,
  ru.win_ppmax - ru.win_ppmin as pre_span,
  ROUND(ru.win_ppmax / NULLIF(ru.win_ppmin, 0), 2) as pre_ratio,
  LOG(NULLIF(ru.win_pre_vol, 0)) as log_pre_vol,
  ru.win_flag
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
JOIN racing.courses c ON c.course_id = r.course_id
JOIN racing.horses h ON h.horse_id = ru.horse_id
WHERE r.race_date BETWEEN $1 AND $2
  AND ru.win_ppmax > 0 
  AND ru.win_ppmin > 0
  AND ru.win_pre_vol > 0
ORDER BY pre_span DESC;
```

---

#### Feature 28: Course Leaders
**Endpoint**: `GET /api/v1/analysis/course-leaders`

**Trainer Leaders by Course:**
```sql
SELECT 
  t.trainer_name,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) as sr,
  SUM(ru.prize) as earnings
FROM racing.runners ru
JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
JOIN racing.races r ON r.race_id = ru.race_id
WHERE r.course_id = $1
  AND r.race_date BETWEEN $2 AND $3
GROUP BY t.trainer_id, t.trainer_name
HAVING COUNT(*) >= 10
ORDER BY wins DESC
LIMIT 20;
```

---

#### Feature 29: Distance & Going Ladders
**Endpoint**: `GET /api/v1/horses/{horse_id}/ladders`

**Distance Ladder:**
```sql
SELECT 
  r.dist_f,
  r.dist_raw,
  COUNT(*) as runs,
  COUNT(*) FILTER (WHERE ru.win_flag) as wins,
  AVG(ru.rpr) FILTER (WHERE ru.rpr IS NOT NULL) as avg_rpr,
  ARRAY_AGG(r.race_date ORDER BY r.race_date DESC) as dates
FROM racing.runners ru
JOIN racing.races r ON r.race_id = ru.race_id
WHERE ru.horse_id = $1 AND r.dist_f IS NOT NULL
GROUP BY r.dist_f, r.dist_raw
ORDER BY r.dist_f;
```

---

#### Feature 30: Meeting Summary with Liquidity
**Endpoint**: `GET /api/v1/meetings/{meeting_key}/summary`

**Database Query:**
```sql
SELECT 
  r.race_id,
  r.race_name,
  r.off_time,
  r.ran,
  SUM(ru.win_pre_vol) as total_win_pre_vol,
  SUM(ru.win_ip_vol) as total_win_ip_vol,
  AVG(ru.win_bsp) FILTER (WHERE ru.win_flag) as winner_bsp
FROM racing.races r
JOIN racing.runners ru ON ru.race_id = r.race_id
WHERE r.meeting_key = $1
GROUP BY r.race_id, r.race_name, r.off_time, r.ran
ORDER BY r.off_time;
```

---

## 🚀 Implementation Checklist

### Phase 1: Core Infrastructure (Week 1-2)
- [ ] Set up API framework (FastAPI/Express/etc.)
- [ ] Database connection pool
- [ ] Read-only user with proper grants
- [ ] Authentication & authorization
- [ ] Rate limiting & caching (Redis)
- [ ] Error handling & logging

### Phase 2: Search & Profiles (Week 2-3)
- [ ] Global search endpoint
- [ ] Horse/Trainer/Jockey profiles
- [ ] Course & meeting pages
- [ ] Comment FTS

### Phase 3: Race Exploration (Week 3-4)
- [ ] Race explorer with filters
- [ ] Per-race dashboard
- [ ] Head-to-head analysis

### Phase 4: Market Analytics (Week 4-5)
- [ ] Steamers/drifters
- [ ] Market calibration
- [ ] Book vs exchange
- [ ] Place efficiency
- [ ] In-play analysis

### Phase 5: Bias & Form (Week 5-6)
- [ ] Draw bias analyzer
- [ ] Form splits
- [ ] Recency effects
- [ ] Trainer change impact
- [ ] Jumping incidents

### Phase 6: Workflow Features (Week 6-7)
- [ ] Watchlists (app tables)
- [ ] Saved queries
- [ ] Export functionality
- [ ] Data quality console
- [ ] SQL console (admin)

### Phase 7: Advanced Analytics (Week 7-8)
- [ ] Pace/style analysis
- [ ] Market movement
- [ ] Course leaders
- [ ] Distance/going ladders
- [ ] Meeting summaries

---

## 📊 Performance Optimization

### Indexing Strategy
All key indexes are already defined in `postgres/init_clean.sql`:
- Trigram (GIN) on name columns
- B-tree on date, foreign keys
- Hash on race_key, runner_key
- Composite indexes on (horse_id, race_date), (trainer_id, race_date), etc.

### Query Optimization Tips
1. **Use CTEs** for complex queries (better optimization)
2. **EXPLAIN ANALYZE** all slow queries
3. **Partition pruning**: Always filter by date for partitioned tables
4. **Connection pooling**: 20-50 connections per instance
5. **Prepared statements**: Reuse query plans

### Caching Strategy
- **Hot data** (recent races, top horses): 5 min TTL
- **Profiles**: 1 hour TTL, invalidate on new data
- **Static data** (courses, dimensions): 24 hour TTL
- **Analytics** (calibration, leaders): 1 day TTL

---

## 📝 API Documentation

**Use OpenAPI/Swagger** for auto-generated docs:
```yaml
openapi: 3.0.0
info:
  title: Racing Data API
  version: 1.0.0
paths:
  /api/v1/search:
    get:
      summary: Global search
      parameters:
        - name: q
          in: query
          required: true
          schema:
            type: string
      responses:
        200:
          description: Search results
```

---

## 🎯 Next Steps

1. **Choose your stack** (Python/FastAPI recommended for speed)
2. **Set up database connection** with read-only user
3. **Implement authentication** (JWT or OAuth)
4. **Build core endpoints** (search, profiles, race explorer)
5. **Add caching layer** (Redis)
6. **Document with Swagger/OpenAPI**
7. **Load test** with realistic traffic patterns
8. **Deploy** with CI/CD pipeline

**Estimated Timeline**: 8-10 weeks for full implementation

---

## 📚 Resources

- Database Schema: `postgres/database.md`
- API Examples: `postgres/API_DOCUMENTATION.md`
- Performance: `PERFORMANCE_OPTIMIZATION.md`
- Project Overview: `README.md`

**Questions?** Review the database schema first, then reach out to the data team.

