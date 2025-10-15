-- Comprehensive Data Verification Script
-- Samples each month across all years and tests random horses
-- Run: docker exec -it horse_racing psql -U postgres -d horse_db -f /path/to/this/file

SET search_path TO racing, public;

\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  COMPREHENSIVE DATA VERIFICATION'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo ''

-- ============================================================
-- 1. Monthly Coverage Report (All Years)
-- ============================================================

\echo '📊 MONTHLY COVERAGE REPORT'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

SELECT 
    TO_CHAR(race_date, 'YYYY-MM') AS month,
    COUNT(*) AS total_races,
    COUNT(DISTINCT course_id) AS unique_courses,
    SUM(CASE WHEN ran IS NOT NULL AND ran > 0 THEN 1 ELSE 0 END) AS races_with_runners,
    SUM(CASE WHEN ran IS NULL OR ran = 0 THEN 1 ELSE 0 END) AS races_missing_runners,
    ROUND(100.0 * SUM(CASE WHEN ran IS NOT NULL AND ran > 0 THEN 1 ELSE 0 END) / COUNT(*), 1) AS coverage_pct
FROM races
WHERE race_date >= '2015-01-01'
GROUP BY 1
ORDER BY 1 DESC
LIMIT 50;

\echo ''

-- ============================================================
-- 2. Sample Random Horses Per Month (Last 24 Months)
-- ============================================================

\echo '🐎 RANDOM HORSE SAMPLING (Last 24 Months)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH monthly_horses AS (
    SELECT DISTINCT
        TO_CHAR(ra.race_date, 'YYYY-MM') AS month,
        r.horse_id,
        h.horse_name,
        COUNT(*) OVER (PARTITION BY TO_CHAR(ra.race_date, 'YYYY-MM'), r.horse_id) AS runs_in_month
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    JOIN horses h ON h.horse_id = r.horse_id
    WHERE ra.race_date >= CURRENT_DATE - INTERVAL '24 months'
      AND r.pos_num IS NOT NULL
),
sampled AS (
    SELECT 
        month,
        horse_id,
        horse_name,
        runs_in_month,
        ROW_NUMBER() OVER (PARTITION BY month ORDER BY RANDOM()) AS rn
    FROM monthly_horses
)
SELECT 
    month,
    horse_name,
    runs_in_month,
    horse_id
FROM sampled
WHERE rn <= 3  -- 3 random horses per month
ORDER BY month DESC, horse_name;

\echo ''

-- ============================================================
-- 3. Data Quality Issues
-- ============================================================

\echo '⚠️  DATA QUALITY ISSUES'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

-- 3.1 Missing Runner Counts
\echo '❌ Races Missing Runner Count (ran IS NULL)'
SELECT 
    race_date,
    course_id,
    race_key,
    COUNT(*) OVER (PARTITION BY race_date) AS affected_races_that_day
FROM races
WHERE ran IS NULL
  AND race_date >= '2024-01-01'
ORDER BY race_date DESC
LIMIT 20;

\echo ''

-- 3.2 Runner Count Mismatches
\echo '⚠️  Runner Count Mismatches (ran != actual runners)'
SELECT 
    ra.race_date,
    ra.race_key,
    ra.ran AS declared,
    COUNT(r.runner_id) AS actual,
    ra.ran - COUNT(r.runner_id) AS difference
FROM races ra
LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
WHERE ra.race_date >= '2024-01-01'
  AND ra.ran IS NOT NULL
GROUP BY ra.race_id, ra.race_date, ra.race_key, ra.ran
HAVING ra.ran != COUNT(r.runner_id)
ORDER BY ABS(ra.ran - COUNT(r.runner_id)) DESC
LIMIT 20;

\echo ''

-- 3.3 Unresolved Horses (horse_id IS NULL)
\echo '❓ Unresolved Horses (Last 30 Days)'
SELECT 
    ra.race_date,
    r.horse_name,
    COUNT(*) AS occurrences
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
WHERE ra.race_date >= CURRENT_DATE - INTERVAL '30 days'
  AND r.horse_id IS NULL
  AND r.horse_name IS NOT NULL
  AND r.horse_name != ''
GROUP BY ra.race_date, r.horse_name
ORDER BY occurrences DESC, ra.race_date DESC
LIMIT 20;

\echo ''

-- ============================================================
-- 4. Yearly Sample Testing
-- ============================================================

\echo '📅 YEARLY SAMPLE TESTING'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH yearly_stats AS (
    SELECT 
        EXTRACT(YEAR FROM race_date)::int AS year,
        COUNT(*) AS total_races,
        SUM(CASE WHEN ran IS NOT NULL THEN ran ELSE 0 END) AS declared_runners,
        COUNT(DISTINCT r.runner_id) AS actual_runners,
        COUNT(DISTINCT ra.course_id) AS unique_courses,
        MIN(race_date) AS first_race,
        MAX(race_date) AS last_race
    FROM races ra
    LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
    WHERE race_date >= '2015-01-01'
    GROUP BY 1
)
SELECT 
    year,
    total_races,
    declared_runners,
    actual_runners,
    unique_courses,
    first_race::text,
    last_race::text,
    CASE 
        WHEN declared_runners > 0 
        THEN ROUND(100.0 * actual_runners / declared_runners, 1)
        ELSE 0 
    END AS runner_match_pct
FROM yearly_stats
ORDER BY year DESC;

\echo ''

-- ============================================================
-- 5. Sample 5 Horses Per Year and Check All Their Races
-- ============================================================

\echo '🔍 DETAILED HORSE VERIFICATION (5 per year, 2020-2024)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH yearly_horses AS (
    SELECT DISTINCT
        EXTRACT(YEAR FROM ra.race_date)::int AS year,
        r.horse_id,
        h.horse_name,
        COUNT(*) AS total_runs,
        SUM(CASE WHEN r.win_flag THEN 1 ELSE 0 END) AS wins
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    JOIN horses h ON h.horse_id = r.horse_id
    WHERE ra.race_date >= '2020-01-01'
      AND r.pos_num IS NOT NULL
      AND r.horse_id IS NOT NULL
    GROUP BY 1, 2, 3
    HAVING COUNT(*) >= 5  -- Horses with at least 5 runs
),
sampled_horses AS (
    SELECT 
        year,
        horse_id,
        horse_name,
        total_runs,
        wins,
        ROW_NUMBER() OVER (PARTITION BY year ORDER BY RANDOM()) AS rn
    FROM yearly_horses
)
SELECT 
    year,
    horse_name,
    total_runs,
    wins,
    ROUND(100.0 * wins / total_runs, 1) AS win_rate,
    horse_id
FROM sampled_horses
WHERE rn <= 5
ORDER BY year DESC, horse_name;

\echo ''

-- ============================================================
-- 6. Missing Betfair Data Analysis
-- ============================================================

\echo '💰 BETFAIR DATA COVERAGE'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

SELECT 
    TO_CHAR(ra.race_date, 'YYYY-MM') AS month,
    COUNT(*) AS total_runners,
    SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS with_bsp,
    ROUND(100.0 * SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) / COUNT(*), 1) AS bsp_coverage_pct,
    SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END) AS with_sp,
    ROUND(100.0 * SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END) / COUNT(*), 1) AS sp_coverage_pct
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
WHERE ra.race_date >= '2024-01-01'
  AND r.pos_num IS NOT NULL
GROUP BY 1
ORDER BY 1 DESC;

\echo ''

-- ============================================================
-- 7. Critical Issues Summary
-- ============================================================

\echo '🚨 CRITICAL ISSUES SUMMARY'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH issues AS (
    SELECT 'Missing Runner Counts' AS issue_type,
           COUNT(*) AS count
    FROM races
    WHERE ran IS NULL AND race_date >= '2024-01-01'
    
    UNION ALL
    
    SELECT 'Runner Count Mismatches',
           COUNT(*)
    FROM (
        SELECT ra.race_id
        FROM races ra
        LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
        WHERE ra.race_date >= '2024-01-01' AND ra.ran IS NOT NULL
        GROUP BY ra.race_id, ra.ran
        HAVING ra.ran != COUNT(r.runner_id)
    ) x
    
    UNION ALL
    
    SELECT 'Unresolved Horses (Last 30d)',
           COUNT(DISTINCT r.horse_name)
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    WHERE ra.race_date >= CURRENT_DATE - INTERVAL '30 days'
      AND r.horse_id IS NULL
      AND r.horse_name IS NOT NULL
    
    UNION ALL
    
    SELECT 'Missing Betfair BSP (2024)',
           COUNT(*)
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    WHERE ra.race_date >= '2024-01-01'
      AND r.pos_num IS NOT NULL
      AND r.win_bsp IS NULL
)
SELECT 
    issue_type,
    count,
    CASE 
        WHEN count = 0 THEN '✅ OK'
        WHEN count < 100 THEN '⚠️  Warning'
        ELSE '❌ Critical'
    END AS severity
FROM issues
ORDER BY count DESC;

\echo ''
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  VERIFICATION COMPLETE'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

