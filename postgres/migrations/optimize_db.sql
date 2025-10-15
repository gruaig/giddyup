-- Database Optimization for GiddyUp API
-- Adds composite indexes to speed up profile queries

SET search_path TO racing, public;

\echo 'Creating performance indexes...'

-- Composite indexes for profile queries
\echo '1. Creating index on runners(horse_id, race_date)...'
CREATE INDEX IF NOT EXISTS idx_runners_horse_date 
ON runners(horse_id, race_date DESC);

\echo '2. Creating index on runners(trainer_id, race_date)...'
CREATE INDEX IF NOT EXISTS idx_runners_trainer_date 
ON runners(trainer_id, race_date DESC) 
WHERE trainer_id IS NOT NULL;

\echo '3. Creating index on runners(jockey_id, race_date)...'
CREATE INDEX IF NOT EXISTS idx_runners_jockey_date 
ON runners(jockey_id, race_date DESC) 
WHERE jockey_id IS NOT NULL;

-- Covering index for form queries (includes common columns)
\echo '4. Creating covering index for horse form...'
CREATE INDEX IF NOT EXISTS idx_runners_horse_form
ON runners(horse_id, race_date DESC)
INCLUDE (pos_num, pos_raw, win_flag, btn, "or", rpr, win_bsp, dec, secs);

-- Index for going/distance splits
\echo '5. Creating index for splits analysis...'
CREATE INDEX IF NOT EXISTS idx_races_course_date_type
ON races(course_id, race_date, race_type)
INCLUDE (going, dist_f, class);

\echo ''
\echo 'Running ANALYZE to update statistics...'
ANALYZE runners;
ANALYZE races;

\echo ''
\echo '✅ Optimization complete!'
\echo ''
\echo 'Index Sizes:'
SELECT 
    schemaname,
    tablename,
    indexname,
    pg_size_pretty(pg_relation_size(indexname::regclass)) AS size
FROM pg_indexes
WHERE schemaname = 'racing'
  AND indexname LIKE 'idx_runners%'
ORDER BY pg_relation_size(indexname::regclass) DESC;

