-- Production Hardening - Performance & Robustness
-- Run after loading data to optimize all API endpoints

SET search_path TO racing, public;

\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  GiddyUp API - Production Hardening'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo ''

-- ============================================================
-- 1) Core Indexes (if not already created)
-- ============================================================

\echo '1️⃣  Creating core performance indexes...'

-- Base traversal indexes
CREATE INDEX IF NOT EXISTS ix_runners_horse_date   ON runners  (horse_id, race_date);
CREATE INDEX IF NOT EXISTS ix_runners_trainer_date ON runners  (trainer_id, race_date);
CREATE INDEX IF NOT EXISTS ix_runners_jockey_date  ON runners  (jockey_id, race_date);
CREATE INDEX IF NOT EXISTS ix_races_date_type      ON races    (race_date, race_type);

-- Common filters/joins
CREATE INDEX IF NOT EXISTS ix_runners_raceid       ON runners  (race_id);
CREATE INDEX IF NOT EXISTS ix_races_course         ON races    (course_id, dist_f, surface);

-- Comment FTS (handles NULL safely)
CREATE INDEX IF NOT EXISTS ix_runners_comment_fts
  ON runners USING GIN (to_tsvector('english', coalesce(comment,'')));

\echo '   ✅ Core indexes created'

-- ============================================================
-- 2) Materialized Views for Fast Queries
-- ============================================================

\echo ''
\echo '2️⃣  Creating materialized views...'

-- 2.1 Runner Base (denormalized, ~1.8M rows)
\echo '   Creating mv_runner_base...'

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_runner_base AS
SELECT
  r.runner_id, r.race_id, r.race_date, r.horse_id, r.trainer_id, r.jockey_id,
  r.pos_num, r.win_flag, r.btn,
  r.dec, r.win_bsp, r.win_ppwap,
  r."or", r.rpr, r.draw, r.lbs,
  ra.race_type, ra.class, ra.dist_f, ra.surface, ra.going, ra.course_id
FROM runners r
JOIN races ra ON ra.race_id = r.race_id;

CREATE INDEX IF NOT EXISTS mv_rb_horse_date   ON mv_runner_base (horse_id, race_date DESC);
CREATE INDEX IF NOT EXISTS mv_rb_trainer_date ON mv_runner_base (trainer_id, race_date DESC);
CREATE INDEX IF NOT EXISTS mv_rb_jockey_date  ON mv_runner_base (jockey_id, race_date DESC);
CREATE INDEX IF NOT EXISTS mv_rb_race_type    ON mv_runner_base (race_type, race_date DESC);

\echo '   ✅ mv_runner_base created'

-- 2.2 Draw Bias (precomputed by course×distance×surface)
\echo '   Creating mv_draw_bias_flat...'

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_draw_bias_flat AS
WITH flat AS (
  SELECT 
    rb.course_id,
    rb.surface,
    rb.dist_f,
    rb.draw,
    rb.win_flag
  FROM mv_runner_base rb
  WHERE rb.race_type = 'Flat' 
    AND rb.draw IS NOT NULL
    AND rb.dist_f IS NOT NULL
)
SELECT
  course_id,
  surface,
  round(dist_f::numeric, 1) AS dist_f_bin,
  width_bucket(draw, 1, 40, 4) AS draw_quartile,
  count(*) AS n,
  sum(win_flag::int) AS wins,
  avg(win_flag::int)::numeric(6,4) AS win_rate,
  avg(draw)::numeric(4,1) AS avg_draw
FROM flat
GROUP BY 1, 2, 3, 4;

CREATE INDEX IF NOT EXISTS mv_db_idx ON mv_draw_bias_flat(course_id, surface, dist_f_bin);

\echo '   ✅ mv_draw_bias_flat created'

-- 2.3 Last-Next pairs (already created, verify exists)
\echo '   Verifying mv_last_next...'

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_matviews WHERE schemaname = 'racing' AND matviewname = 'mv_last_next') THEN
    RAISE NOTICE 'mv_last_next does not exist - creating now...';
    -- Create it (reuse from earlier)
    CREATE MATERIALIZED VIEW mv_last_next AS
    WITH ordered AS (
      SELECT
        r.horse_id, r.runner_id, r.race_id, r.race_date,
        r.pos_num, r.btn, r."or" AS or_now,
        ra.race_type, ra.class, ra.dist_f, ra.surface, ra.going, ra.course_id,
        r.win_bsp, r.dec, r.win_ppwap, r.win_flag,
        LEAD(r.runner_id) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_runner_id,
        LEAD(r.race_id) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_race_id,
        LEAD(r.race_date) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_date,
        LEAD(r.pos_num) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_pos,
        LEAD(r.win_flag) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_win,
        LEAD(r."or") OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_or
      FROM runners r
      JOIN races ra ON ra.race_id = r.race_id
      WHERE r.pos_num IS NOT NULL
    )
    SELECT
      o.horse_id, o.runner_id AS last_runner_id, o.race_id AS last_race_id,
      o.race_date AS last_date, o.pos_num AS last_pos, o.btn AS last_btn,
      o.or_now AS last_or, o.race_type AS last_race_type, o.class AS last_class,
      o.dist_f AS last_dist_f, o.surface AS last_surface, o.going AS last_going,
      o.course_id AS last_course_id,
      o.next_runner_id, o.next_race_id, o.next_date, o.next_pos, o.next_win, o.next_or,
      (o.next_date - o.race_date) AS dsr_next
    FROM ordered o
    WHERE o.next_race_id IS NOT NULL;
    
    CREATE INDEX IF NOT EXISTS mvln_dates_idx ON mv_last_next (last_date, next_date);
    CREATE INDEX IF NOT EXISTS mvln_filter_idx ON mv_last_next (last_pos, last_btn, dsr_next);
    CREATE INDEX IF NOT EXISTS mvln_horse_idx ON mv_last_next (horse_id);
  ELSE
    RAISE NOTICE 'mv_last_next already exists';
  END IF;
END $$;

\echo '   ✅ mv_last_next verified'

-- ============================================================
-- 3) Update Statistics
-- ============================================================

\echo ''
\echo '3️⃣  Updating statistics...'

ANALYZE runners;
ANALYZE races;
ANALYZE mv_runner_base;
ANALYZE mv_draw_bias_flat;
ANALYZE mv_last_next;

\echo '   ✅ Statistics updated'

-- ============================================================
-- 4) Performance Verification
-- ============================================================

\echo ''
\echo '4️⃣  Performance verification...'

-- Check index sizes
\echo '   Index sizes:'
SELECT 
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) AS size
FROM pg_indexes
WHERE schemaname = 'racing'
  AND (indexname LIKE 'ix_%' OR indexname LIKE 'mv_%' OR indexname LIKE 'mvln_%')
ORDER BY pg_relation_size(indexname::regclass) DESC
LIMIT 10;

\echo ''
\echo '   Materialized view sizes:'
SELECT 
    matviewname,
    pg_size_pretty(pg_relation_size(matviewname::regclass)) AS size
FROM pg_matviews
WHERE schemaname = 'racing'
ORDER BY pg_relation_size(matviewname::regclass) DESC;

-- ============================================================
-- 5) Refresh Instructions
-- ============================================================

\echo ''
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  ✅ Production hardening complete!'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo ''
\echo 'After loading new data, refresh materialized views:'
\echo ''
\echo '  REFRESH MATERIALIZED VIEW CONCURRENTLY mv_runner_base;'
\echo '  REFRESH MATERIALIZED VIEW CONCURRENTLY mv_draw_bias_flat;'
\echo '  REFRESH MATERIALIZED VIEW CONCURRENTLY mv_last_next;'
\echo ''
\echo 'Expected API performance after optimization:'
\echo '  - Horse profile: < 500ms'
\echo '  - Trainer profile: < 500ms'
\echo '  - Jockey profile: < 500ms'
\echo '  - Comment FTS: < 300ms'
\echo '  - Draw bias: < 400ms'
\echo '  - Market endpoints: < 200ms'
\echo '  - Angle backtest: < 100ms'
\echo ''

