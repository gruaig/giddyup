# Racing Database (Postgres) — Schema & Ingestion

This document defines the **production-ready Postgres schema**, **ingestion plan**, and **feature coverage** for the Racing database built from your master CSVs (Racing Post results + Betfair win/place markets). It is designed for **daily incremental loads**, **fast querying**, and **future-proof** analytics.

---

## 1) Goals & Scope

- One **runner-level fact** table (historical results + markets) with a **race meta** table.
- **Dimensional model** for entities (horses, trainers, jockeys, owners, courses, bloodlines).
- **Robust keys** (`race_key`, `runner_key`) with surrogate IDs in dims.
- **Monthly partitioning** for scalable storage and query pruning.
- **Search-first**: trigram name search + full-text search (FTS) on comments.
- **Idempotent ingestion**: re-runnable without duplicates; supports late-arriving corrections.
- **Feature-complete**: supports the 30-feature UI out-of-the-box (see §8).

---

## 2) Source Data & Assumptions

**Master files** (monthly partitions, plain CSV):

- `races_{region}_{code}_{YYYY-MM}.csv`
- `runners_{region}_{code}_{YYYY-MM}.csv`

**Schemas** (column order):

- **races**: `date,region,course,off,race_name,type,class,pattern,rating_band,age_band,sex_rest,dist,dist_f,dist_m,going,surface,ran,race_key`
- **runners**: `race_key,num,pos,draw,ovr_btn,btn,horse,age,sex,lbs,hg,time,secs,dec,jockey,trainer,prize,prize_raw,or,rpr,sire,dam,damsire,owner,comment,win_bsp,win_ppwap,win_morningwap,win_ppmax,win_ppmin,win_ipmax,win_ipmin,win_morning_vol,win_pre_vol,win_ip_vol,win_lose,place_bsp,place_ppwap,place_morningwap,place_ppmax,place_ppmin,place_ipmax,place_ipmin,place_morning_vol,place_pre_vol,place_ip_vol,place_win_lose,runner_key,match_jaccard,match_time_diff_min,match_reason`

**Type conventions**:
- Keep numerics as numeric (ints/floats). Missing → **blank** (NULL), never `-` or `–`.
- Exchange prices: replace sentinels `1`/`1.0` with NULL (minimum valid is 1.01).
- `dist_f` is **numeric** (e.g., `10.0`), `dist` is human-readable (`"1m2f"`).
- `ran` equals actual runner count per race.

---

## 3) Schema Overview

```
         ┌──────────┐      ┌──────────┐
         │ courses  │◀────▶│ meetings │  (optional view)
         └────┬─────┘      └────┬─────┘
              │                 │
         ┌────▼─────┐      ┌────▼─────┐
         │  races   │──────│ runners  │  (facts, partitioned by month)
         └────┬─────┘      └────┬─────┘
              │                 │
┌─────────────▼──────────────┐  ┌───────▼───────────┐
│ horses / trainers / jockeys│  │ owners / bloodlines│ (dims)
└────────────────────────────┘  └────────────────────┘
```

---

## 4) DDL — Schema & Tables

> Requires extensions:
>
> ```sql
> CREATE SCHEMA IF NOT EXISTS racing;
> CREATE EXTENSION IF NOT EXISTS pg_trgm;
> CREATE EXTENSION IF NOT EXISTS unaccent;
> SET search_path TO racing, public;
> ```

### 4.1 Dimensions

```sql
-- Name normalizer (for matching & uniqueness)
CREATE OR REPLACE FUNCTION racing.norm_text(t text)
RETURNS text LANGUAGE sql IMMUTABLE AS $$
  SELECT regexp_replace(
           regexp_replace(
             unaccent(lower(coalesce($1,''))),
             '\s*\([a-z]{2,3}\)\s*$', '', 'g'   -- drop (GB)/(IRE)/...
           ),
           '[^a-z0-9\s]', ' ', 'g'
         );
$$;

CREATE TABLE courses (
  course_id   BIGSERIAL PRIMARY KEY,
  course_name TEXT NOT NULL,
  region      TEXT NOT NULL,      -- GB/IRE/...
  course_norm TEXT GENERATED ALWAYS AS (racing.norm_text(course_name)) STORED,
  CONSTRAINT courses_uniq UNIQUE (region, course_norm)
);

CREATE TABLE horses (
  horse_id    BIGSERIAL PRIMARY KEY,
  horse_name  TEXT NOT NULL,
  horse_norm  TEXT GENERATED ALWAYS AS (racing.norm_text(horse_name)) STORED,
  CONSTRAINT horses_uniq UNIQUE (horse_norm)
);

CREATE TABLE horse_alias (
  horse_id   BIGINT REFERENCES horses(horse_id) ON DELETE CASCADE,
  alias      TEXT NOT NULL,
  alias_norm TEXT GENERATED ALWAYS AS (racing.norm_text(alias)) STORED,
  PRIMARY KEY (horse_id, alias_norm)
);

CREATE TABLE trainers (
  trainer_id   BIGSERIAL PRIMARY KEY,
  trainer_name TEXT NOT NULL,
  trainer_norm TEXT GENERATED ALWAYS AS (racing.norm_text(trainer_name)) STORED,
  CONSTRAINT trainers_uniq UNIQUE (trainer_norm)
);

CREATE TABLE jockeys (
  jockey_id   BIGSERIAL PRIMARY KEY,
  jockey_name TEXT NOT NULL,
  jockey_norm TEXT GENERATED ALWAYS AS (racing.norm_text(jockey_name)) STORED,
  CONSTRAINT jockeys_uniq UNIQUE (jockey_norm)
);

CREATE TABLE owners (
  owner_id   BIGSERIAL PRIMARY KEY,
  owner_name TEXT NOT NULL,
  owner_norm TEXT GENERATED ALWAYS AS (racing.norm_text(owner_name)) STORED,
  CONSTRAINT owners_uniq UNIQUE (owner_norm)
);

CREATE TABLE bloodlines (
  blood_id BIGSERIAL PRIMARY KEY,
  sire      TEXT,  sire_norm     TEXT GENERATED ALWAYS AS (racing.norm_text(sire)) STORED,
  dam       TEXT,  dam_norm      TEXT GENERATED ALWAYS AS (racing.norm_text(dam)) STORED,
  damsire   TEXT,  damsire_norm  TEXT GENERATED ALWAYS AS (racing.norm_text(damsire)) STORED,
  CONSTRAINT blood_uniq UNIQUE (sire_norm, dam_norm, damsire_norm)
);
```

### 4.2 Facts (monthly partitions)

> We store `race_date` in **both** tables so each can be range-partitioned by month.

```sql
-- RACES
CREATE TABLE races (
  race_id     BIGSERIAL PRIMARY KEY,
  race_key    TEXT UNIQUE NOT NULL,     -- from master
  race_date   DATE NOT NULL,
  region      TEXT NOT NULL,
  course_id   BIGINT REFERENCES courses(course_id),
  off_time    TIME,
  race_name   TEXT NOT NULL,
  race_type   TEXT NOT NULL,            -- Flat/Hurdle/Chase/NH Flat
  class       TEXT,
  pattern     TEXT,
  rating_band TEXT,
  age_band    TEXT,
  sex_rest    TEXT,
  dist_raw    TEXT,
  dist_f      DOUBLE PRECISION,
  dist_m      INTEGER,
  going       TEXT,
  surface     TEXT,
  ran         INTEGER NOT NULL
) PARTITION BY RANGE (race_date);

-- RUNNERS
CREATE TABLE runners (
  runner_id   BIGSERIAL PRIMARY KEY,
  runner_key  TEXT UNIQUE NOT NULL,     -- from master
  race_id     BIGINT NOT NULL REFERENCES races(race_id) ON DELETE CASCADE,
  race_date   DATE NOT NULL,            -- redundantly stored for partitioning

  -- Dims
  horse_id    BIGINT REFERENCES horses(horse_id),
  trainer_id  BIGINT REFERENCES trainers(trainer_id),
  jockey_id   BIGINT REFERENCES jockeys(jockey_id),
  owner_id    BIGINT REFERENCES owners(owner_id),
  blood_id    BIGINT REFERENCES bloodlines(blood_id),

  -- RP runner fields
  num INTEGER, pos_raw TEXT, draw INTEGER,
  ovr_btn DOUBLE PRECISION, btn DOUBLE PRECISION,
  age INTEGER, sex TEXT, lbs INTEGER, hg TEXT,
  time_raw TEXT, secs DOUBLE PRECISION, dec DOUBLE PRECISION,
  prize DOUBLE PRECISION, prize_raw TEXT,
  "or" INTEGER, rpr INTEGER, comment TEXT,

  -- Betfair WIN
  win_bsp DOUBLE PRECISION, win_ppwap DOUBLE PRECISION, win_morningwap DOUBLE PRECISION,
  win_ppmax DOUBLE PRECISION, win_ppmin DOUBLE PRECISION,
  win_ipmax DOUBLE PRECISION, win_ipmin DOUBLE PRECISION,
  win_morning_vol DOUBLE PRECISION, win_pre_vol DOUBLE PRECISION, win_ip_vol DOUBLE PRECISION,
  win_lose INTEGER,

  -- Betfair PLACE
  place_bsp DOUBLE PRECISION, place_ppwap DOUBLE PRECISION, place_morningwap DOUBLE PRECISION,
  place_ppmax DOUBLE PRECISION, place_ppmin DOUBLE PRECISION,
  place_ipmax DOUBLE PRECISION, place_ipmin DOUBLE PRECISION,
  place_morning_vol DOUBLE PRECISION, place_pre_vol DOUBLE PRECISION, place_ip_vol DOUBLE PRECISION,
  place_win_lose INTEGER,

  -- Diagnostics
  match_jaccard DOUBLE PRECISION,
  match_time_diff_min INTEGER,
  match_reason TEXT,

  -- Generated helpers
  pos_num INTEGER GENERATED ALWAYS AS (
    CASE WHEN pos_raw ~ '^[0-9]+' THEN (regexp_replace(pos_raw,'[^0-9].*',''))::int END
  ) STORED,
  win_flag BOOLEAN GENERATED ALWAYS AS (pos_num = 1) STORED,

  -- Quality constraints (light)
  CONSTRAINT runners_bsp_chk CHECK (win_bsp IS NULL OR win_bsp >= 1.01),
  CONSTRAINT runners_dec_chk CHECK (dec     IS NULL OR dec     >= 1.01)
) PARTITION BY RANGE (race_date);
```

### 4.3 Partitions (helper macro)

```sql
-- Create monthly partitions for a given year (races + runners)
DO $$
DECLARE m int;
BEGIN
  FOR m IN 1..12 LOOP
    EXECUTE format('CREATE TABLE IF NOT EXISTS races_%s PARTITION OF races FOR VALUES FROM (DATE %L) TO (DATE %L);',
                   to_char(make_date(2025,m,1),'YYYY_MM'),
                   make_date(2025,m,1), make_date(2025, m, 1) + INTERVAL '1 month');
    EXECUTE format('CREATE TABLE IF NOT EXISTS runners_%s PARTITION OF runners FOR VALUES FROM (DATE %L) TO (DATE %L);',
                   to_char(make_date(2025,m,1),'YYYY_MM'),
                   make_date(2025,m,1), make_date(2025, m, 1) + INTERVAL '1 month');
  END LOOP;
END$$;
```

> Run yearly for each year you plan to ingest (adjust `2025`). New months can be created on demand during daily loads.

### 4.4 Staging tables

```sql
CREATE TABLE stage_races   (LIKE races INCLUDING ALL);
CREATE TABLE stage_runners (LIKE runners INCLUDING ALL);
-- But load them as TEXT via COPY, then cast in INSERT...SELECT (see §6).
TRUNCATE stage_races; TRUNCATE stage_runners;
```

---

## 5) Indexes & Search

```sql
-- Common filters
CREATE INDEX races_date_idx           ON races (race_date);
CREATE INDEX races_course_date_idx    ON races (course_id, race_date);
CREATE INDEX races_type_going_idx     ON races (race_type, going);
CREATE INDEX races_distf_idx          ON races (dist_f);

CREATE INDEX runners_raceid_idx       ON runners (race_id);
CREATE INDEX runners_date_idx         ON runners (race_date);
CREATE INDEX runners_dims_idx         ON runners (horse_id, trainer_id, jockey_id);
CREATE INDEX runners_draw_idx         ON runners (draw) WHERE draw IS NOT NULL;
CREATE INDEX runners_price_idx        ON runners (win_bsp, dec);

-- Performance indexes for API profile queries (CRITICAL for <1s response times)
-- Without these, profile queries take 30+ seconds; with them: <1 second
CREATE INDEX idx_runners_horse_date 
  ON runners(horse_id, race_date DESC);

CREATE INDEX idx_runners_trainer_date 
  ON runners(trainer_id, race_date DESC) 
  WHERE trainer_id IS NOT NULL;

CREATE INDEX idx_runners_jockey_date 
  ON runners(jockey_id, race_date DESC) 
  WHERE jockey_id IS NOT NULL;

-- Covering index for horse form queries (avoids table lookups)
CREATE INDEX idx_runners_horse_form
  ON runners(horse_id, race_date DESC)
  INCLUDE (pos_num, pos_raw, win_flag, btn, "or", rpr, win_bsp, dec, secs);

-- Index for race filtering and splits analysis
CREATE INDEX idx_races_course_date_type
  ON races(course_id, race_date, race_type)
  INCLUDE (going, dist_f, class);

-- Full-text search on comments
CREATE INDEX runners_comment_fts ON runners USING GIN (to_tsvector('english', comment));

-- Trigram search on names (global search)
CREATE INDEX horses_trgm   ON horses  USING GIN (horse_name   gin_trgm_ops);
CREATE INDEX trainers_trgm ON trainers USING GIN (trainer_name gin_trgm_ops);
CREATE INDEX jockeys_trgm  ON jockeys USING GIN (jockey_name  gin_trgm_ops);
CREATE INDEX owners_trgm   ON owners  USING GIN (owner_name   gin_trgm_ops);
CREATE INDEX courses_trgm  ON courses USING GIN (course_name  gin_trgm_ops);
```

---

## 6) Helper Views & Materialized Views

### 6.1 Lightweight views

```sql
-- Shortcut with meeting_key
CREATE OR REPLACE VIEW v_runners AS
SELECT r.runner_id, r.runner_key, r.race_id, r.race_date,
       ra.region, ra.race_name, ra.race_type, ra.going, ra.surface, ra.dist_f, ra.ran, ra.off_time,
       ra.course_id,
       md5(ra.race_date::text || '|' || ra.region || '|' || c.course_name) AS meeting_key,
       r.*
FROM runners r
JOIN races ra   ON ra.race_id = r.race_id
JOIN courses c  ON c.course_id = ra.course_id;

-- Pre-off microstructure
CREATE OR REPLACE VIEW v_market AS
SELECT runner_id, race_id, race_date, num, pos_num, win_flag,
       win_bsp, dec,
       (win_ppmax - win_ppmin) AS pre_span,
       CASE WHEN win_ppmax > 0 THEN win_ppmin / win_ppmax END AS pre_ratio,
       ln(1 + coalesce(win_pre_vol,0)) AS log_pre_vol
FROM runners;
```

### 6.2 Materialized views (refresh after loads)

> Define and index the heavy hitters. Examples below are templates you can refine.

```sql
-- Horse history with days-since-run
CREATE MATERIALIZED VIEW mv_horse_history AS
SELECT r.horse_id, r.runner_id, r.race_id, r.race_date,
       r.pos_num, r.win_flag, r.win_bsp, r.place_bsp, r.dec,
       r."or", r.rpr, ra.course_id, ra.race_type, ra.going, ra.surface, ra.dist_f,
       r.num, r.draw,
       r.race_date - lag(r.race_date) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS dsr
FROM runners r JOIN races ra USING (race_id);
CREATE INDEX ON mv_horse_history (horse_id, race_date);

-- Rolling trainer form windows (example: last 90 runs)
CREATE MATERIALIZED VIEW mv_trainer_form AS
WITH base AS (
  SELECT trainer_id, race_date, win_flag,
         row_number() OVER (PARTITION BY trainer_id ORDER BY race_date DESC) AS rn
  FROM runners
)
SELECT trainer_id,
       count(*) FILTER (WHERE rn <= 90) AS last90_runs,
       sum(win_flag::int) FILTER (WHERE rn <= 90) AS last90_wins
FROM base GROUP BY trainer_id;

-- Draw bias (Flat): win rate by (course, dist bucket, going bucket, draw percentile)
CREATE MATERIALIZED VIEW mv_draw_bias_flat AS
WITH flat AS (
  SELECT r.runner_id, ra.course_id, ra.dist_f,
         CASE WHEN ra.dist_f < 7 THEN '<7f' WHEN ra.dist_f <= 9 THEN '7-9f' ELSE '>9f' END AS dist_bin,
         CASE WHEN ra.going ILIKE '%soft%' THEN 'soft' WHEN ra.going ILIKE '%heavy%' THEN 'heavy'
              WHEN ra.going ILIKE '%firm%' THEN 'firm' ELSE 'std' END AS going_bin,
         r.draw, ra.ran,
         ntile(4) OVER (PARTITION BY ra.course_id, ra.dist_f ORDER BY r.draw) AS draw_quartile,
         r.win_flag
  FROM runners r JOIN races ra USING (race_id)
  WHERE ra.race_type = 'Flat' AND r.draw IS NOT NULL AND ra.ran >= 6
)
SELECT course_id, dist_bin, going_bin, draw_quartile,
       count(*) AS n, sum(win_flag::int) AS wins,
       avg(win_flag::int)::numeric(6,4) AS win_rate
FROM flat GROUP BY 1,2,3,4;
CREATE INDEX ON mv_draw_bias_flat (course_id);
```

> Refresh pattern: `REFRESH MATERIALIZED VIEW CONCURRENTLY mv_horse_history;` (add indexes first).

---

## 7) Ingestion — Daily Loader (Python + SQL)

**Daily flow** (per region/code/month):

1. Discover new/changed **master CSVs** for the day/month.
2. `COPY` into `stage_races` / `stage_runners` (as TEXT columns).
3. **Upsert dims** (`courses`, `horses`, `trainers`, `jockeys`, `owners`, `bloodlines`).
4. **Upsert races** (casted types; `race_key` unique).
5. **Upsert runners** (casted types; `runner_key` unique). Populate `race_date` from the file date; ensure it matches the race.
6. **Commit**, `ANALYZE` affected partitions.
7. **Refresh** materialized views (fast path: only those used in the UI; nightly full refresh).

**CSV → staging (fastest with psql):**

```sql
\copy racing.stage_races   FROM '/path/races_ire_flat_2023-09.csv'   CSV HEADER
\copy racing.stage_runners FROM '/path/runners_ire_flat_2023-09.csv' CSV HEADER
```

**Dimension upserts:**

```sql
INSERT INTO racing.courses (course_name, region)
SELECT DISTINCT course, region FROM racing.stage_races
ON CONFLICT (region, course_norm) DO NOTHING;

INSERT INTO racing.horses (horse_name)
SELECT DISTINCT horse FROM racing.stage_runners WHERE horse <> ''
ON CONFLICT (horse_norm) DO NOTHING;
-- Repeat for trainers, jockeys, owners, bloodlines
```

**Races upsert (cast & clean):**

```sql
WITH casted AS (
  SELECT
    race_key,
    (date)::date AS race_date,
    region,
    (SELECT course_id FROM racing.courses c
      WHERE c.region = stage_races.region
        AND racing.norm_text(c.course_name) = racing.norm_text(stage_races.course)
      LIMIT 1) AS course_id,
    (off)::time AS off_time,
    race_name, "type" AS race_type, class, pattern, rating_band, age_band, sex_rest,
    dist AS dist_raw,
    NULLIF(replace(dist_f,'f',''),'')::double precision AS dist_f,
    NULLIF(dist_m,'')::int AS dist_m,
    going, surface,
    NULLIF(ran,'')::int AS ran
  FROM racing.stage_races
)
INSERT INTO racing.races (...columns...)
SELECT * FROM casted
ON CONFLICT (race_key) DO UPDATE
SET going = COALESCE(EXCLUDED.going, races.going),
    ran   = COALESCE(EXCLUDED.ran, races.ran);
```

**Runners upsert (cast & clean; remove sentinel `1` prices):**

```sql
WITH maps AS (
  SELECT race_id, race_key, race_date FROM racing.races
),
casted AS (
  SELECT
    r.runner_key,
    m.race_id,
    m.race_date,
    (SELECT horse_id   FROM racing.horses   h WHERE h.horse_norm   = racing.norm_text(r.horse)   LIMIT 1) AS horse_id,
    (SELECT trainer_id FROM racing.trainers t WHERE t.trainer_norm = racing.norm_text(r.trainer) LIMIT 1) AS trainer_id,
    (SELECT jockey_id  FROM racing.jockeys  j WHERE j.jockey_norm  = racing.norm_text(r.jockey)  LIMIT 1) AS jockey_id,
    (SELECT owner_id   FROM racing.owners   o WHERE o.owner_norm   = racing.norm_text(r.owner)   LIMIT 1) AS owner_id,
    (SELECT blood_id   FROM racing.bloodlines b
      WHERE b.sire_norm = racing.norm_text(r.sire)
        AND b.dam_norm = racing.norm_text(r.dam)
        AND b.damsire_norm = racing.norm_text(r.damsire)
      LIMIT 1) AS blood_id,
    NULLIF(num,'')::int AS num,
    NULLIF(pos,'') AS pos_raw,
    NULLIF(draw,'')::int AS draw,
    NULLIF(ovr_btn,'')::double precision AS ovr_btn,
    NULLIF(btn,'')::double precision AS btn,
    NULLIF(age,'')::int AS age,
    NULLIF(sex,'') AS sex,
    NULLIF(lbs,'')::int AS lbs,
    NULLIF(hg,'') AS hg,
    NULLIF(time,'') AS time_raw,
    NULLIF(secs,'')::double precision AS secs,
    NULLIF(dec,'')::double precision AS dec,
    NULLIF(regexp_replace(prize,'[^0-9\.]','','g'),'')::double precision AS prize,
    prize AS prize_raw,
    NULLIF("or",'')::int AS "or",
    NULLIF(rpr,'')::int AS rpr,
    comment,
    NULLIF(NULLIF(win_bsp,'1'),'')::double precision AS win_bsp,
    NULLIF(NULLIF(win_ppwap,'1'),'')::double precision AS win_ppwap,
    NULLIF(NULLIF(win_morningwap,'1'),'')::double precision AS win_morningwap,
    NULLIF(NULLIF(win_ppmax,'1'),'')::double precision AS win_ppmax,
    NULLIF(NULLIF(win_ppmin,'1'),'')::double precision AS win_ppmin,
    NULLIF(NULLIF(win_ipmax,'1'),'')::double precision AS win_ipmax,
    NULLIF(NULLIF(win_ipmin,'1'),'')::double precision AS win_ipmin,
    NULLIF(win_morning_vol,'')::double precision AS win_morning_vol,
    NULLIF(win_pre_vol,'')::double precision AS win_pre_vol,
    NULLIF(win_ip_vol,'')::double precision AS win_ip_vol,
    NULLIF(win_lose,'')::int AS win_lose,
    NULLIF(NULLIF(place_bsp,'1'),'')::double precision AS place_bsp,
    NULLIF(NULLIF(place_ppwap,'1'),'')::double precision AS place_ppwap,
    NULLIF(NULLIF(place_morningwap,'1'),'')::double precision AS place_morningwap,
    NULLIF(NULLIF(place_ppmax,'1'),'')::double precision AS place_ppmax,
    NULLIF(NULLIF(place_ppmin,'1'),'')::double precision AS place_ppmin,
    NULLIF(NULLIF(place_ipmax,'1'),'')::double precision AS place_ipmax,
    NULLIF(NULLIF(place_ipmin,'1'),'')::double precision AS place_ipmin,
    NULLIF(place_morning_vol,'')::double precision AS place_morning_vol,
    NULLIF(place_pre_vol,'')::double precision AS place_pre_vol,
    NULLIF(place_ip_vol,'')::double precision AS place_ip_vol,
    NULLIF(place_win_lose,'')::int AS place_win_lose,
    NULLIF(match_jaccard,'')::double precision AS match_jaccard,
    NULLIF(match_time_diff_min,'')::int AS match_time_diff_min,
    NULLIF(match_reason,'') AS match_reason
  FROM racing.stage_runners r
  JOIN maps m ON m.race_key = r.race_key
)
INSERT INTO racing.runners (...columns...)
SELECT * FROM casted
ON CONFLICT (runner_key) DO UPDATE
SET pos_raw = COALESCE(EXCLUDED.pos_raw, runners.pos_raw),
    secs    = COALESCE(EXCLUDED.secs, runners.secs),
    dec     = COALESCE(EXCLUDED.dec, runners.dec),
    prize   = COALESCE(EXCLUDED.prize, runners.prize),
    "or"    = COALESCE(EXCLUDED."or", runners."or"),
    rpr     = COALESCE(EXCLUDED.rpr, runners.rpr),
    win_bsp = COALESCE(EXCLUDED.win_bsp, runners.win_bsp),
    win_ppwap = COALESCE(EXCLUDED.win_ppwap, runners.win_ppwap),
    win_morningwap = COALESCE(EXCLUDED.win_morningwap, runners.win_morningwap),
    win_ppmax = COALESCE(EXCLUDED.win_ppmax, runners.win_ppmax),
    win_ppmin = COALESCE(EXCLUDED.win_ppmin, runners.win_ppmin),
    win_ipmax = COALESCE(EXCLUDED.win_ipmax, runners.win_ipmax),
    win_ipmin = COALESCE(EXCLUDED.win_ipmin, runners.win_ipmin),
    win_morning_vol = COALESCE(EXCLUDED.win_morning_vol, runners.win_morning_vol),
    win_pre_vol = COALESCE(EXCLUDED.win_pre_vol, runners.win_pre_vol),
    win_ip_vol = COALESCE(EXCLUDED.win_ip_vol, runners.win_ip_vol),
    win_lose = COALESCE(EXCLUDED.win_lose, runners.win_lose),
    place_bsp = COALESCE(EXCLUDED.place_bsp, runners.place_bsp),
    place_ppwap = COALESCE(EXCLUDED.place_ppwap, runners.place_ppwap),
    place_morningwap = COALESCE(EXCLUDED.place_morningwap, runners.place_morningwap),
    place_ppmax = COALESCE(EXCLUDED.place_ppmax, runners.place_ppmax),
    place_ppmin = COALESCE(EXCLUDED.place_ppmin, runners.place_ppmin),
    place_ipmax = COALESCE(EXCLUDED.place_ipmax, runners.place_ipmax),
    place_ipmin = COALESCE(EXCLUDED.place_ipmin, runners.place_ipmin),
    place_morning_vol = COALESCE(EXCLUDED.place_morning_vol, runners.place_morning_vol),
    place_pre_vol = COALESCE(EXCLUDED.place_pre_vol, runners.place_pre_vol),
    place_ip_vol = COALESCE(EXCLUDED.place_ip_vol, runners.place_ip_vol),
    place_win_lose = COALESCE(EXCLUDED.place_win_lose, runners.place_win_lose),
    match_jaccard = COALESCE(EXCLUDED.match_jaccard, runners.match_jaccard),
    match_time_diff_min = COALESCE(EXCLUDED.match_time_diff_min, runners.match_time_diff_min),
    match_reason = COALESCE(EXCLUDED.match_reason, runners.match_reason);
```

**Post-load maintenance:**

```sql
VACUUM ANALYZE racing.races;
VACUUM ANALYZE racing.runners;
REFRESH MATERIALIZED VIEW CONCURRENTLY racing.mv_horse_history;
REFRESH MATERIALIZED VIEW CONCURRENTLY racing.mv_draw_bias_flat;
REFRESH MATERIALIZED VIEW CONCURRENTLY racing.mv_trainer_form;
```

---

## 8) Feature Coverage (30 features supported)

**Search & Navigation**
1. Global search bar across horses/trainers/jockeys/owners/courses/race names (trigram indexes).
2. Advanced text search in comments (FTS with filters).
3. Course & meeting pages (group by `meeting_key`).

**Profiles**
4. Horse profile: career, splits by going/distance/course/surface/class; RPR/OR trends.
5. Trainer profile: SR/ROI, rolling 14/30/90 form; course & distance splits.
6. Jockey profile: SR/ROI, trainer combos; recency form.
7. Sire/Dam explorer: progeny performance splits.

**Race Exploration**
8. Race explorer: filters on date/region/course/type/class/pattern/handicap, distance/going/surface, field size.
9. Per-race dashboard: runner cards (OR/RPR, weights, draw, comments, BSP & place BSP, microstructure).
10. Head-to-head & field matchups via `race_key`.

**Market & Value Analytics**
11. Steamers & drifters (pre-off move via `win_ppmin/max`, `win_bsp`).
12. Market calibration (win): implied vs actual using BSP bins.
13. Book vs exchange comparison: `dec` vs `win_bsp` and ROI.
14. Place market efficiency: `place_bsp` calibration & ROI.
15. In-play collapse/surge: `win_ipmin`, `win_ipmax` signals.

**Bias & Form**
16. Draw bias analyzer (flat): draw × field size × going heatmaps.
17. Form & splits panel: distance/going/course/class buckets; days-since-run.
18. Recency effects (DSR buckets).
19. Trainer change impact (before/after yard switch).
20. Jumping incidents: F/UR/PU/BD/RR via `pos_raw` + comment parsing.

**Workflow & Ops**
21. Watchlists (app tables, not shown here) + historical performance.
22. Saved queries & pins (app tables).
23. Exports of any filtered results (CSV/Parquet).
24. Backfill & data-quality console (manifest + unmatched diagnostics surfaced in UI).
25. SQL console / Query builder (read-only user with limited schema exposure).

**Additional analytical features**
26. Pace / running-style tags from comments; SR/ROI by style.
27. Market movement analyzer: pre_span, pre_ratio, log_pre_vol.
28. Course leaders (trainer/jockey) by period.
29. Distance & going ladders per horse/trainer.
30. Meeting summary dashboards with liquidity metrics (pre/off volumes).

All above are directly supported by the data modeled here (historical). Live **racecards/entries** can be added later without schema changes.

---

## 9) Roles & Security (recommended)

```sql
CREATE ROLE racing_owner NOINHERIT;
CREATE ROLE racing_loader NOINHERIT;
CREATE ROLE racing_readonly NOINHERIT;

GRANT USAGE ON SCHEMA racing TO racing_loader, racing_readonly;
GRANT SELECT ON ALL TABLES IN SCHEMA racing TO racing_readonly;
ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT SELECT ON TABLES TO racing_readonly;

GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA racing TO racing_loader;
ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT INSERT, UPDATE, DELETE ON TABLES TO racing_loader;
```

Use `racing_loader` for the Python importer; the UI connects as `racing_readonly`.

---

## 10) Data Quality Checks (post-ingest)

```sql
-- Races vs runners consistency
SELECT COUNT(*) FROM races;                             -- total races
SELECT COUNT(DISTINCT race_id) FROM runners;            -- should match races count

-- ran equals runner count per race
SELECT ra.race_id, ra.ran, COUNT(*) AS runners
FROM races ra JOIN runners ru USING (race_id)
GROUP BY 1,2 HAVING COUNT(*) <> ra.ran;

-- No sentinel prices remain
SELECT COUNT(*) FROM runners WHERE win_bsp = 1 OR dec = 1;

-- Unique keys sanity
SELECT COUNT(*) - COUNT(DISTINCT race_key)   FROM races;   -- 0
SELECT COUNT(*) - COUNT(DISTINCT runner_key) FROM runners; -- 0
```

---

## 11) Maintenance & Refresh

- Create partitions for new months proactively (or on first insert).
- `VACUUM ANALYZE` after bulk loads.
- Refresh heavy MVs nightly, or on demand after large batches.
- Monitor index bloat; reindex selectively if needed.

---

## 12) Roadmap (non-breaking additions)

- Add `entries` / `racecards` tables for upcoming races (no changes to existing facts).
- Attach external IDs (official horse/trainer/jockey IDs) to dims; keep current norms as fallbacks.
- Add sectionals/pace data keyed by `runner_id` in separate tables.

---

**Done.** This `database.md` is the single source of truth for schema, ingestion, and supported features. Adjust partitions and MVs as your volume grows, but the core model will hold steady.

