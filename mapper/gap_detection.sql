-- Gap Detection SQL Queries
-- Use these to find missing/problematic data
-- Run with: psql -U postgres -d giddyup -f gap_detection.sql

SET search_path TO racing, public;

-- ============================================================
-- 1. Entries Present Today (Racecards)
-- ============================================================

SELECT 
    ra.race_date AS date, 
    ra.course_id, 
    c.course_name, 
    COUNT(r.runner_id) AS entries
FROM races ra
JOIN courses c USING (course_id)
JOIN runners r USING (race_id)
WHERE ra.race_date = CURRENT_DATE
  AND r.pos_raw IS NULL  -- Unraced (racecard entries)
GROUP BY 1, 2, 3
ORDER BY date, course_id;

-- If zero rows: No racecards loaded for today

-- ============================================================
-- 2. Races Whose Off-Time Passed But Have No Results
-- ============================================================

SELECT 
    ra.race_id, 
    ra.race_date, 
    c.course_name, 
    ra.ran,
    SUM(CASE WHEN r.pos_num IS NOT NULL THEN 1 ELSE 0 END) AS have_results
FROM races ra
JOIN courses c USING (course_id)
LEFT JOIN runners r USING (race_id)
WHERE ra.race_date = CURRENT_DATE
GROUP BY ra.race_id, ra.race_date, c.course_name, ra.ran
HAVING COALESCE(ra.ran, 0) > 0 
   AND SUM(CASE WHEN r.pos_num IS NOT NULL THEN 1 ELSE 0 END) = 0
ORDER BY ra.race_id;

-- Shows races that should have results but don't

-- ============================================================
-- 3. Runner Count Mismatch
-- ============================================================

SELECT 
    ra.race_id, 
    ra.race_date, 
    c.course_name, 
    ra.ran AS declared_runners,
    COUNT(r.runner_id) AS actual_runners,
    ra.ran - COUNT(r.runner_id) AS difference
FROM races ra
JOIN courses c USING (course_id)
JOIN runners r USING (race_id)
WHERE ra.race_date BETWEEN CURRENT_DATE - INTERVAL '7 days' AND CURRENT_DATE
  AND r.pos_raw IS NOT NULL  -- Only completed races
GROUP BY ra.race_id, ra.race_date, c.course_name, ra.ran
HAVING ra.ran IS NOT NULL AND ra.ran <> COUNT(r.runner_id)
ORDER BY ABS(ra.ran - COUNT(r.runner_id)) DESC;

-- Shows races where declared runners != actual runners

-- ============================================================
-- 4. Horses in Today's Entries Not Resolvable
-- ============================================================

SELECT DISTINCT ON (r.horse_name)
    r.horse_name, 
    r.race_id,
    ra.race_date,
    c.course_name
FROM runners r
JOIN races ra USING (race_id)
JOIN courses c ON c.course_id = ra.course_id
WHERE ra.race_date = CURRENT_DATE
  AND r.pos_raw IS NULL  -- Racecard entries
  AND r.horse_id IS NULL  -- Not resolved
  AND r.horse_name IS NOT NULL
  AND r.horse_name != ''
ORDER BY r.horse_name, r.race_id;

-- Shows horses in today's cards that couldn't be matched

-- ============================================================
-- 5. Yesterday's Winners Missing
-- ============================================================

SELECT 
    ra.race_id, 
    c.course_name, 
    ra.race_date,
    ra.race_name
FROM races ra
JOIN courses c USING (course_id)
LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num = 1
WHERE ra.race_date = CURRENT_DATE - INTERVAL '1 day'
  AND r.runner_id IS NULL  -- No winner found
  AND ra.ran IS NOT NULL AND ra.ran > 0  -- Race had runners
ORDER BY ra.race_id;

-- Shows yesterday's races without winners

-- ============================================================
-- 6. Data Coverage Report (by month)
-- ============================================================

SELECT 
    TO_CHAR(race_date, 'YYYY-MM') AS month,
    COUNT(*) AS races,
    COUNT(DISTINCT course_id) AS courses,
    SUM(CASE WHEN ran IS NOT NULL THEN 1 ELSE 0 END) AS with_runners,
    SUM(CASE WHEN ran IS NULL THEN 1 ELSE 0 END) AS missing_runners
FROM races
WHERE race_date >= '2024-01-01'
GROUP BY 1
ORDER BY 1 DESC;

-- Shows monthly data coverage

-- ============================================================
-- 7. Unresolved Dimensions (Last 7 Days)
-- ============================================================

WITH unresolved AS (
    SELECT 
        'horse' AS type,
        r.horse_name AS name,
        COUNT(*) AS occurrences,
        MIN(ra.race_date) AS first_seen,
        MAX(ra.race_date) AS last_seen
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    WHERE ra.race_date >= CURRENT_DATE - INTERVAL '7 days'
      AND r.horse_id IS NULL
      AND r.horse_name IS NOT NULL
      AND r.horse_name != ''
    GROUP BY r.horse_name

    UNION ALL

    SELECT 
        'trainer' AS type,
        r.trainer_name AS name,
        COUNT(*) AS occurrences,
        MIN(ra.race_date) AS first_seen,
        MAX(ra.race_date) AS last_seen
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    WHERE ra.race_date >= CURRENT_DATE - INTERVAL '7 days'
      AND r.trainer_id IS NULL
      AND r.trainer_name IS NOT NULL
      AND r.trainer_name != ''
    GROUP BY r.trainer_name

    UNION ALL

    SELECT 
        'jockey' AS type,
        r.jockey_name AS name,
        COUNT(*) AS occurrences,
        MIN(ra.race_date) AS first_seen,
        MAX(ra.race_date) AS last_seen
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    WHERE ra.race_date >= CURRENT_DATE - INTERVAL '7 days'
      AND r.jockey_id IS NULL
      AND r.jockey_name IS NOT NULL
      AND r.jockey_name != ''
    GROUP BY r.jockey_name
)
SELECT *
FROM unresolved
ORDER BY type, occurrences DESC;

-- Shows horses/trainers/jockeys that couldn't be resolved

-- ============================================================
-- 8. Missing Betfair Data (Last 30 Days)
-- ============================================================

SELECT 
    ra.race_date,
    c.course_name,
    COUNT(*) AS total_runners,
    SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS with_bsp,
    COUNT(*) - SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS missing_bsp,
    ROUND(100.0 * SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) / COUNT(*), 1) AS bsp_coverage_pct
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN courses c ON c.course_id = ra.course_id
WHERE ra.race_date >= CURRENT_DATE - INTERVAL '30 days'
  AND r.pos_num IS NOT NULL  -- Only completed races
GROUP BY ra.race_date, c.course_name
HAVING SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) < COUNT(*)
ORDER BY ra.race_date DESC, c.course_name;

-- Shows races with missing Betfair data

-- ============================================================
-- 9. Daily Summary (Last 14 Days)
-- ============================================================

SELECT 
    race_date,
    COUNT(*) AS races,
    SUM(CASE WHEN ran IS NOT NULL THEN ran ELSE 0 END) AS declared_runners,
    COUNT(DISTINCT r.runner_id) AS actual_runners,
    ROUND(100.0 * COUNT(DISTINCT CASE WHEN r.win_bsp IS NOT NULL THEN r.runner_id END) / 
          NULLIF(COUNT(DISTINCT r.runner_id), 0), 1) AS betfair_coverage_pct
FROM races ra
LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
WHERE race_date >= CURRENT_DATE - INTERVAL '14 days'
GROUP BY race_date
ORDER BY race_date DESC;

-- Daily overview of data quality

-- ============================================================
-- 10. Find Duplicate Race Keys
-- ============================================================

SELECT 
    race_key,
    COUNT(*) AS count,
    STRING_AGG(race_id::text, ', ') AS race_ids
FROM races
GROUP BY race_key
HAVING COUNT(*) > 1
ORDER BY count DESC;

-- Shows duplicate race keys (should be unique)

