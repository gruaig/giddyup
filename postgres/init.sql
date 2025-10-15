-- init.sql — horse_db base schema (Postgres)

-- 0) Create database (run as a superuser). If you are already connected to horse_db, you can skip this.
DO $$
BEGIN
  PERFORM 1 FROM pg_database WHERE datname = 'horse_db';
  IF NOT FOUND THEN
    EXECUTE $$CREATE DATABASE horse_db WITH ENCODING 'UTF8' TEMPLATE=template0$$;
  END IF;
END$$;

-- If using psql, connect:
-- \connect horse_db

-- 1) Schema, extensions, search_path
CREATE SCHEMA IF NOT EXISTS racing;
SET search_path TO racing, public;

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE EXTENSION IF NOT EXISTS unaccent;

-- 2) Utility: canonical text normalizer
CREATE OR REPLACE FUNCTION racing.norm_text(t text)
RETURNS text
LANGUAGE sql
IMMUTABLE
AS $$
  SELECT regexp_replace(
           regexp_replace(
             unaccent(lower(coalesce($1,''))),
             '\s*\([a-z]{2,3}\)\s*$', '', 'g'   -- drop (GB)/(IRE)/...
           ),
           '[^a-z0-9\s]', ' ', 'g'
         );
$$;

-- 3) Dimensions
CREATE TABLE IF NOT EXISTS courses (
  course_id   BIGSERIAL PRIMARY KEY,
  course_name TEXT NOT NULL,
  region      TEXT NOT NULL,      -- GB/IRE/...
  course_norm TEXT GENERATED ALWAYS AS (racing.norm_text(course_name)) STORED,
  CONSTRAINT courses_uniq UNIQUE (region, course_norm)
);

CREATE TABLE IF NOT EXISTS horses (
  horse_id    BIGSERIAL PRIMARY KEY,
  horse_name  TEXT NOT NULL,
  horse_norm  TEXT GENERATED ALWAYS AS (racing.norm_text(horse_name)) STORED,
  CONSTRAINT horses_uniq UNIQUE (horse_norm)
);

CREATE TABLE IF NOT EXISTS horse_alias (
  horse_id   BIGINT REFERENCES horses(horse_id) ON DELETE CASCADE,
  alias      TEXT NOT NULL,
  alias_norm TEXT GENERATED ALWAYS AS (racing.norm_text(alias)) STORED,
  PRIMARY KEY (horse_id, alias_norm)
);

CREATE TABLE IF NOT EXISTS trainers (
  trainer_id   BIGSERIAL PRIMARY KEY,
  trainer_name TEXT NOT NULL,
  trainer_norm TEXT GENERATED ALWAYS AS (racing.norm_text(trainer_name)) STORED,
  CONSTRAINT trainers_uniq UNIQUE (trainer_norm)
);

CREATE TABLE IF NOT EXISTS jockeys (
  jockey_id   BIGSERIAL PRIMARY KEY,
  jockey_name TEXT NOT NULL,
  jockey_norm TEXT GENERATED ALWAYS AS (racing.norm_text(jockey_name)) STORED,
  CONSTRAINT jockeys_uniq UNIQUE (jockey_norm)
);

CREATE TABLE IF NOT EXISTS owners (
  owner_id   BIGSERIAL PRIMARY KEY,
  owner_name TEXT NOT NULL,
  owner_norm TEXT GENERATED ALWAYS AS (racing.norm_text(owner_name)) STORED,
  CONSTRAINT owners_uniq UNIQUE (owner_norm)
);

CREATE TABLE IF NOT EXISTS bloodlines (
  blood_id BIGSERIAL PRIMARY KEY,
  sire      TEXT,  sire_norm     TEXT GENERATED ALWAYS AS (racing.norm_text(sire)) STORED,
  dam       TEXT,  dam_norm      TEXT GENERATED ALWAYS AS (racing.norm_text(dam)) STORED,
  damsire   TEXT,  damsire_norm  TEXT GENERATED ALWAYS AS (racing.norm_text(damsire)) STORED,
  CONSTRAINT blood_uniq UNIQUE (sire_norm, dam_norm, damsire_norm)
);

-- 4) Facts (partitioned by month on race_date)
CREATE TABLE IF NOT EXISTS races (
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

CREATE TABLE IF NOT EXISTS runners (
  runner_id   BIGSERIAL PRIMARY KEY,
  runner_key  TEXT UNIQUE NOT NULL,     -- from master
  race_id     BIGINT NOT NULL REFERENCES races(race_id) ON DELETE CASCADE,
  race_date   DATE NOT NULL,            -- duplicated for partitioning

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

  -- Sanity checks
  CONSTRAINT runners_bsp_chk CHECK (win_bsp IS NULL OR win_bsp >= 1.01),
  CONSTRAINT runners_dec_chk CHECK (dec     IS NULL OR dec     >= 1.01)
) PARTITION BY RANGE (race_date);

-- 5) Partition helper (create monthly partitions for a year)
CREATE OR REPLACE FUNCTION racing.create_partitions_for_year(p_year int)
RETURNS void
LANGUAGE plpgsql
AS $$
DECLARE
  m int;
  p_start date;
  p_end   date;
  races_part   text;
  runners_part text;
BEGIN
  FOR m IN 1..12 LOOP
    p_start := make_date(p_year, m, 1);
    p_end   := (p_start + INTERVAL '1 month')::date;

    races_part   := format('races_%s',   to_char(p_start, 'YYYY_MM'));
    runners_part := format('runners_%s', to_char(p_start, 'YYYY_MM'));

    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I PARTITION OF races FOR VALUES FROM (%L) TO (%L);',
      races_part, p_start, p_end
    );

    EXECUTE format(
      'CREATE TABLE IF NOT EXISTS %I PARTITION OF runners FOR VALUES FROM (%L) TO (%L);',
      runners_part, p_start, p_end
    );
  END LOOP;
END$$;

-- Example (uncomment and adjust years you need):
-- SELECT racing.create_partitions_for_year(2007);
-- SELECT racing.create_partitions_for_year(2008);
-- ...
-- SELECT racing.create_partitions_for_year(2025);

-- 6) Indexes & search
CREATE INDEX IF NOT EXISTS races_date_idx           ON races (race_date);
CREATE INDEX IF NOT EXISTS races_course_date_idx    ON races (course_id, race_date);
CREATE INDEX IF NOT EXISTS races_type_going_idx     ON races (race_type, going);
CREATE INDEX IF NOT EXISTS races_distf_idx          ON races (dist_f);

CREATE INDEX IF NOT EXISTS runners_date_idx         ON runners (race_date);
CREATE INDEX IF NOT EXISTS runners_raceid_idx       ON runners (race_id);
CREATE INDEX IF NOT EXISTS runners_dims_idx         ON runners (horse_id, trainer_id, jockey_id);
CREATE INDEX IF NOT EXISTS runners_draw_idx         ON runners (draw) WHERE draw IS NOT NULL;
CREATE INDEX IF NOT EXISTS runners_price_idx        ON runners (win_bsp, dec);

CREATE INDEX IF NOT EXISTS runners_comment_fts ON runners USING GIN (to_tsvector('english', comment));

CREATE INDEX IF NOT EXISTS horses_trgm   ON horses  USING GIN (horse_name   gin_trgm_ops);
CREATE INDEX IF NOT EXISTS trainers_trgm ON trainers USING GIN (trainer_name gin_trgm_ops);
CREATE INDEX IF NOT EXISTS jockeys_trgm  ON jockeys  USING GIN (jockey_name  gin_trgm_ops);
CREATE INDEX IF NOT EXISTS owners_trgm   ON owners   USING GIN (owner_name   gin_trgm_ops);
CREATE INDEX IF NOT EXISTS courses_trgm  ON courses  USING GIN (course_name  gin_trgm_ops);

-- 7) Staging tables for CSV loads (TEXT columns to COPY raw, then cast on insert)
DROP TABLE IF EXISTS stage_races;
CREATE TABLE stage_races (
  date        TEXT, region TEXT, course TEXT, off TEXT, race_name TEXT, "type" TEXT, class TEXT,
  pattern     TEXT, rating_band TEXT, age_band TEXT, sex_rest TEXT,
  dist        TEXT, dist_f TEXT, dist_m TEXT, going TEXT, surface TEXT, ran TEXT, race_key TEXT
);

DROP TABLE IF EXISTS stage_runners;
CREATE TABLE stage_runners (
  race_key TEXT, num TEXT, pos TEXT, draw TEXT, ovr_btn TEXT, btn TEXT, horse TEXT, age TEXT, sex TEXT,
  lbs TEXT, hg TEXT, time TEXT, secs TEXT, dec TEXT, jockey TEXT, trainer TEXT, prize TEXT, prize_raw TEXT,
  "or" TEXT, rpr TEXT, sire TEXT, dam TEXT, damsire TEXT, owner TEXT, comment TEXT,
  win_bsp TEXT, win_ppwap TEXT, win_morningwap TEXT, win_ppmax TEXT, win_ppmin TEXT,
  win_ipmax TEXT, win_ipmin TEXT, win_morning_vol TEXT, win_pre_vol TEXT, win_ip_vol TEXT, win_lose TEXT,
  place_bsp TEXT, place_ppwap TEXT, place_morningwap TEXT, place_ppmax TEXT, place_ppmin TEXT,
  place_ipmax TEXT, place_ipmin TEXT, place_morning_vol TEXT, place_pre_vol TEXT, place_ip_vol TEXT, place_win_lose TEXT,
  runner_key TEXT, match_jaccard TEXT, match_time_diff_min TEXT, match_reason TEXT
);

-- 8) Helper views
CREATE OR REPLACE VIEW v_runners AS
SELECT
  r.runner_id, r.runner_key, r.race_id, r.race_date,
  ra.region, ra.race_name, ra.race_type, ra.going, ra.surface, ra.dist_f, ra.ran, ra.off_time,
  ra.course_id,
  md5(ra.race_date::text || '|' || ra.region || '|' || c.course_name) AS meeting_key,
  r.*
FROM runners r
JOIN races   ra ON ra.race_id  = r.race_id
JOIN courses c  ON c.course_id = ra.course_id;

CREATE OR REPLACE VIEW v_market AS
SELECT runner_id, race_id, race_date, num, pos_num, win_flag,
       win_bsp, dec,
       (win_ppmax - win_ppmin) AS pre_span,
       CASE WHEN win_ppmax > 0 THEN win_ppmin / win_ppmax END AS pre_ratio,
       ln(1 + coalesce(win_pre_vol,0)) AS log_pre_vol
FROM runners;

-- 9) (Optional) Example materialized views (define now; refresh after loads)
CREATE MATERIALIZED VIEW IF NOT EXISTS mv_horse_history AS
SELECT r.horse_id, r.runner_id, r.race_id, r.race_date,
       r.pos_num, r.win_flag, r.win_bsp, r.place_bsp, r.dec,
       r."or", r.rpr, ra.course_id, ra.race_type, ra.going, ra.surface, ra.dist_f,
       r.num, r.draw,
       r.race_date - lag(r.race_date) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS dsr
FROM runners r
JOIN races ra USING (race_id);
CREATE INDEX IF NOT EXISTS mv_horse_history_horse_date_idx ON mv_horse_history (horse_id, race_date);

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_draw_bias_flat AS
WITH flat AS (
  SELECT r.runner_id, ra.course_id, ra.dist_f,
         CASE WHEN ra.dist_f < 7 THEN '<7f' WHEN ra.dist_f <= 9 THEN '7-9f' ELSE '>9f' END AS dist_bin,
         CASE WHEN ra.going ILIKE '%soft%' THEN 'soft'
              WHEN ra.going ILIKE '%heavy%' THEN 'heavy'
              WHEN ra.going ILIKE '%firm%' THEN 'firm' ELSE 'std' END AS going_bin,
         r.draw, ra.ran,
         ntile(4) OVER (PARTITION BY ra.course_id, ra.dist_f ORDER BY r.draw) AS draw_quartile,
         r.win_flag
  FROM runners r
  JOIN races ra USING (race_id)
  WHERE ra.race_type = 'Flat' AND r.draw IS NOT NULL AND ra.ran >= 6
)
SELECT course_id, dist_bin, going_bin, draw_quartile,
       count(*) AS n, sum(win_flag::int) AS wins,
       avg(win_flag::int)::numeric(6,4) AS win_rate
FROM flat
GROUP BY 1,2,3,4;
CREATE INDEX IF NOT EXISTS mv_draw_bias_flat_course_idx ON mv_draw_bias_flat (course_id);

-- 10) Roles (optional; adjust as needed)
-- CREATE ROLE racing_loader NOINHERIT;
-- CREATE ROLE racing_readonly NOINHERIT;
-- GRANT USAGE ON SCHEMA racing TO racing_loader, racing_readonly;
-- GRANT SELECT ON ALL TABLES IN SCHEMA racing TO racing_readonly;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT SELECT ON TABLES TO racing_readonly;
-- GRANT INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA racing TO racing_loader;
-- ALTER DEFAULT PRIVILEGES IN SCHEMA racing GRANT INSERT, UPDATE, DELETE ON TABLES TO racing_loader;

-- 11) Done
-- After running init.sql:
--  - call racing.create_partitions_for_year(Y) for all years you intend to load
--  - COPY into stage tables, then upsert into dims/facts via your Python script

