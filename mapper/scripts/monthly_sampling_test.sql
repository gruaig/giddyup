-- Monthly Sampling Verification Test
-- Tests random samples from each month across all years
-- Run: docker exec -i horse_racing psql -U postgres -d horse_db -f /path/to/this

SET search_path TO racing, public;

\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  MONTHLY SAMPLING VERIFICATION TEST'
\echo '  Testing random horses across all months'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo ''

-- ============================================================
-- 1. Data Coverage by Year
-- ============================================================

\echo '📊 YEARLY DATA COVERAGE'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

SELECT 
    EXTRACT(YEAR FROM race_date)::int AS year,
    COUNT(*) AS races,
    COUNT(DISTINCT course_id) AS courses,
    SUM(COALESCE(ran, 0)) AS declared_runners,
    COUNT(DISTINCT r.runner_id) AS actual_runners_in_db,
    MIN(race_date)::text AS first_race,
    MAX(race_date)::text AS last_race,
    ROUND(100.0 * COUNT(DISTINCT r.runner_id) / NULLIF(SUM(COALESCE(ran, 0)), 0), 1) AS data_completeness_pct
FROM races ra
LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
WHERE race_date >= '2015-01-01'
GROUP BY 1
ORDER BY 1 DESC;

\echo ''

-- ============================================================
-- 2. Sample 10 Random Horses Per Year and Verify All Data Points
-- ============================================================

\echo '🐎 RANDOM HORSE DEEP DIVE (10 per year, 2020-2024)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH yearly_horses AS (
    SELECT 
        EXTRACT(YEAR FROM ra.race_date)::int AS year,
        r.horse_id,
        h.horse_name,
        COUNT(*) AS runs,
        SUM(CASE WHEN r.win_flag THEN 1 ELSE 0 END) AS wins,
        SUM(CASE WHEN r."or" IS NOT NULL THEN 1 ELSE 0 END) AS with_or,
        SUM(CASE WHEN r.rpr IS NOT NULL THEN 1 ELSE 0 END) AS with_rpr,
        SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS with_bsp,
        SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END) AS with_sp,
        SUM(CASE WHEN r.trainer_id IS NOT NULL THEN 1 ELSE 0 END) AS with_trainer,
        SUM(CASE WHEN r.jockey_id IS NOT NULL THEN 1 ELSE 0 END) AS with_jockey
    FROM runners r
    JOIN races ra ON ra.race_id = r.race_id
    JOIN horses h ON h.horse_id = r.horse_id
    WHERE ra.race_date >= '2020-01-01'
      AND r.pos_num IS NOT NULL
    GROUP BY 1, 2, 3
    HAVING COUNT(*) >= 5
),
sampled AS (
    SELECT 
        *,
        ROW_NUMBER() OVER (PARTITION BY year ORDER BY RANDOM()) AS rn
    FROM yearly_horses
)
SELECT 
    year,
    horse_name,
    runs,
    wins,
    ROUND(100.0 * with_or / runs, 0) AS or_pct,
    ROUND(100.0 * with_rpr / runs, 0) AS rpr_pct,
    ROUND(100.0 * with_bsp / runs, 0) AS bsp_pct,
    ROUND(100.0 * with_sp / runs, 0) AS sp_pct,
    ROUND(100.0 * with_trainer / runs, 0) AS trainer_pct,
    ROUND(100.0 * with_jockey / runs, 0) AS jockey_pct,
    horse_id
FROM sampled
WHERE rn <= 10
ORDER BY year DESC, horse_name;

\echo ''

-- ============================================================
-- 3. Check ONE Horse in Detail (Pick Random Active Horse)
-- ============================================================

\echo '🔬 DETAILED SINGLE HORSE VERIFICATION'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH random_horse AS (
    SELECT horse_id, horse_name
    FROM horses
    WHERE horse_id IN (
        SELECT DISTINCT r.horse_id
        FROM runners r
        JOIN races ra ON ra.race_id = r.race_id
        WHERE ra.race_date >= '2023-01-01'
          AND r.pos_num IS NOT NULL
        LIMIT 1000
    )
    ORDER BY RANDOM()
    LIMIT 1
)
SELECT 
    rh.horse_name AS "🐎 Sample Horse",
    rh.horse_id AS "Horse ID",
    (SELECT COUNT(*) 
     FROM runners r JOIN races ra ON ra.race_id = r.race_id
     WHERE r.horse_id = rh.horse_id AND r.pos_num IS NOT NULL) AS "Total Runs in DB",
    (SELECT COUNT(DISTINCT ra.race_date) 
     FROM runners r JOIN races ra ON ra.race_id = r.race_id
     WHERE r.horse_id = rh.horse_id AND r.pos_num IS NOT NULL) AS "Unique Race Days",
    (SELECT SUM(CASE WHEN win_flag THEN 1 ELSE 0 END)
     FROM runners WHERE horse_id = rh.horse_id) AS "Total Wins",
    (SELECT MIN(ra.race_date)::text
     FROM runners r JOIN races ra ON ra.race_id = r.race_id
     WHERE r.horse_id = rh.horse_id) AS "First Race",
    (SELECT MAX(ra.race_date)::text
     FROM runners r JOIN races ra ON ra.race_id = r.race_id
     WHERE r.horse_id = rh.horse_id) AS "Last Race",
    (SELECT COUNT(*) 
     FROM runners r 
     WHERE r.horse_id = rh.horse_id AND r.win_bsp IS NOT NULL) AS "Races with BSP",
    (SELECT COUNT(*) 
     FROM runners r 
     WHERE r.horse_id = rh.horse_id AND r."or" IS NOT NULL) AS "Races with OR",
    (SELECT COUNT(*) 
     FROM runners r 
     WHERE r.horse_id = rh.horse_id AND r.trainer_id IS NOT NULL) AS "Races with Trainer Resolved"
FROM random_horse rh;

\echo ''

-- Show last 5 races for this horse
\echo 'Last 5 Races for Sample Horse:'
\echo '─────────────────────────────────────────────'

WITH random_horse AS (
    SELECT horse_id
    FROM horses
    WHERE horse_id IN (
        SELECT DISTINCT r.horse_id
        FROM runners r
        JOIN races ra ON ra.race_id = r.race_id
        WHERE ra.race_date >= '2023-01-01'
        LIMIT 1000
    )
    ORDER BY RANDOM()
    LIMIT 1
)
SELECT 
    ra.race_date,
    c.course_name,
    r.pos_num AS pos,
    r.btn,
    COALESCE(r.win_bsp::text, 'NULL') AS bsp,
    COALESCE(r.dec::text, 'NULL') AS sp,
    COALESCE(r."or"::text, 'NULL') AS "or",
    COALESCE(r.rpr::text, 'NULL') AS rpr,
    CASE WHEN r.trainer_id IS NOT NULL THEN '✅' ELSE '❌' END AS trainer_ok,
    CASE WHEN r.jockey_id IS NOT NULL THEN '✅' ELSE '❌' END AS jockey_ok
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
JOIN courses c ON c.course_id = ra.course_id
WHERE r.horse_id = (SELECT horse_id FROM random_horse)
  AND r.pos_num IS NOT NULL
ORDER BY ra.race_date DESC
LIMIT 5;

\echo ''

-- ============================================================
-- 4. Sample 3 Races Per Month (Random)
-- ============================================================

\echo '🏇 RANDOM RACE SAMPLING (Last 12 Months)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH monthly_races AS (
    SELECT 
        TO_CHAR(race_date, 'YYYY-MM') AS month,
        race_id,
        race_date,
        course_id,
        race_key,
        ran,
        ROW_NUMBER() OVER (PARTITION BY TO_CHAR(race_date, 'YYYY-MM') ORDER BY RANDOM()) AS rn
    FROM races
    WHERE race_date >= CURRENT_DATE - INTERVAL '12 months'
      AND ran IS NOT NULL
)
SELECT 
    mr.month,
    mr.race_date::text,
    c.course_name,
    mr.ran AS declared,
    COUNT(r.runner_id) AS actual,
    mr.ran - COUNT(r.runner_id) AS difference,
    CASE 
        WHEN mr.ran = COUNT(r.runner_id) THEN '✅' 
        WHEN ABS(mr.ran - COUNT(r.runner_id)) <= 2 THEN '⚠️'
        ELSE '❌' 
    END AS status
FROM monthly_races mr
JOIN courses c ON c.course_id = mr.course_id
LEFT JOIN runners r ON r.race_id = mr.race_id AND r.pos_num IS NOT NULL
WHERE mr.rn <= 3
GROUP BY mr.month, mr.race_date, c.course_name, mr.ran, mr.race_id
ORDER BY mr.month DESC, mr.race_date DESC;

\echo ''

-- ============================================================
-- 5. Critical Data Completeness Metrics
-- ============================================================

\echo '✅ DATA COMPLETENESS SUMMARY'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH stats AS (
    SELECT 
        COUNT(DISTINCT ra.race_id) AS total_races,
        COUNT(DISTINCT r.runner_id) AS total_runners,
        SUM(CASE WHEN ra.ran IS NOT NULL THEN 1 ELSE 0 END) AS races_with_ran,
        SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_bsp,
        SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_sp,
        SUM(CASE WHEN r."or" IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_or,
        SUM(CASE WHEN r.rpr IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_rpr,
        SUM(CASE WHEN r.horse_id IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_horse_resolved,
        SUM(CASE WHEN r.trainer_id IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_trainer_resolved,
        SUM(CASE WHEN r.jockey_id IS NOT NULL THEN 1 ELSE 0 END) AS runners_with_jockey_resolved
    FROM races ra
    LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
    WHERE ra.race_date >= '2024-01-01'
)
SELECT 
    'Total Races (2024)' AS metric,
    total_races AS value,
    '100%' AS completeness
FROM stats
UNION ALL
SELECT 
    'Total Runners (2024)',
    total_runners,
    '100%'
FROM stats
UNION ALL
SELECT 
    'Races with Runner Count',
    races_with_ran,
    ROUND(100.0 * races_with_ran / total_races, 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Runners with BSP',
    runners_with_bsp,
    ROUND(100.0 * runners_with_bsp / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Runners with SP',
    runners_with_sp,
    ROUND(100.0 * runners_with_sp / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Runners with OR',
    runners_with_or,
    ROUND(100.0 * runners_with_or / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Runners with RPR',
    runners_with_rpr,
    ROUND(100.0 * runners_with_rpr / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Horse Name Resolved',
    runners_with_horse_resolved,
    ROUND(100.0 * runners_with_horse_resolved / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Trainer Name Resolved',
    runners_with_trainer_resolved,
    ROUND(100.0 * runners_with_trainer_resolved / NULLIF(total_runners, 0), 1) || '%'
FROM stats
UNION ALL
SELECT 
    'Jockey Name Resolved',
    runners_with_jockey_resolved,
    ROUND(100.0 * runners_with_jockey_resolved / NULLIF(total_runners, 0), 1) || '%'
FROM stats;

\echo ''

-- ============================================================
-- 6. Test Specific Famous Horses (Known Good Data)
-- ============================================================

\echo '⭐ TESTING FAMOUS HORSES (Known Reference Data)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH famous AS (
    SELECT 
        h.horse_id,
        h.horse_name,
        COUNT(*) AS runs_in_db,
        SUM(CASE WHEN r.win_flag THEN 1 ELSE 0 END) AS wins,
        MIN(ra.race_date)::text AS first_race,
        MAX(ra.race_date)::text AS last_race,
        MAX(r.rpr) AS peak_rpr,
        SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) AS with_bsp
    FROM horses h
    LEFT JOIN runners r ON r.horse_id = h.horse_id AND r.pos_num IS NOT NULL
    LEFT JOIN races ra ON ra.race_id = r.race_id
    WHERE h.horse_name IN (
        'Frankel (GB)',
        'Enable (GB)',
        'Sea The Stars (IRE)',
        'Kauto Star (FR)',
        'Denman (IRE)',
        'Sprinter Sacre (FR)',
        'Altior (IRE)',
        'Stradivarius (IRE)',
        'Golden Horn (GB)',
        'Roaring Lion (USA)'
    )
    GROUP BY h.horse_id, h.horse_name
)
SELECT 
    horse_name,
    runs_in_db,
    wins,
    ROUND(100.0 * wins / NULLIF(runs_in_db, 0), 1) AS sr_pct,
    first_race,
    last_race,
    peak_rpr,
    with_bsp,
    CASE 
        WHEN runs_in_db = 0 THEN '❌ NO DATA'
        WHEN with_bsp = runs_in_db THEN '✅ COMPLETE'
        WHEN with_bsp::float / NULLIF(runs_in_db, 0) > 0.9 THEN '⚠️ MOSTLY COMPLETE'
        ELSE '❌ INCOMPLETE'
    END AS status
FROM famous
ORDER BY runs_in_db DESC;

\echo ''

-- ============================================================
-- 7. Monthly Data Quality Scores
-- ============================================================

\echo '📈 MONTHLY DATA QUALITY SCORES (2024)'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

WITH monthly_quality AS (
    SELECT 
        TO_CHAR(ra.race_date, 'YYYY-MM') AS month,
        COUNT(DISTINCT ra.race_id) AS races,
        COUNT(DISTINCT r.runner_id) AS runners,
        ROUND(100.0 * SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS bsp_pct,
        ROUND(100.0 * SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS sp_pct,
        ROUND(100.0 * SUM(CASE WHEN r."or" IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS or_pct,
        ROUND(100.0 * SUM(CASE WHEN r.rpr IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS rpr_pct,
        ROUND(100.0 * SUM(CASE WHEN r.horse_id IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS horse_resolved_pct,
        ROUND(100.0 * SUM(CASE WHEN r.trainer_id IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS trainer_resolved_pct,
        ROUND(100.0 * SUM(CASE WHEN r.jockey_id IS NOT NULL THEN 1 ELSE 0 END) / 
              NULLIF(COUNT(r.runner_id), 0), 1) AS jockey_resolved_pct,
        -- Quality score (average of all metrics)
        ROUND((
            SUM(CASE WHEN r.win_bsp IS NOT NULL THEN 1 ELSE 0 END)::float +
            SUM(CASE WHEN r.dec IS NOT NULL THEN 1 ELSE 0 END)::float +
            SUM(CASE WHEN r."or" IS NOT NULL THEN 1 ELSE 0 END)::float +
            SUM(CASE WHEN r.horse_id IS NOT NULL THEN 1 ELSE 0 END)::float
        ) / (COUNT(r.runner_id)::float * 4) * 100, 1) AS overall_quality_score
    FROM races ra
    LEFT JOIN runners r ON r.race_id = ra.race_id AND r.pos_num IS NOT NULL
    WHERE ra.race_date >= '2024-01-01'
    GROUP BY 1
)
SELECT 
    month,
    races,
    runners,
    bsp_pct || '%' AS bsp,
    sp_pct || '%' AS sp,
    or_pct || '%' AS "or",
    rpr_pct || '%' AS rpr,
    horse_resolved_pct || '%' AS horse_resolved,
    trainer_resolved_pct || '%' AS trainer_resolved,
    jockey_resolved_pct || '%' AS jockey_resolved,
    overall_quality_score || '%' AS quality_score,
    CASE 
        WHEN overall_quality_score >= 95 THEN '✅ EXCELLENT'
        WHEN overall_quality_score >= 85 THEN '⚠️ GOOD'
        WHEN overall_quality_score >= 70 THEN '⚠️ FAIR'
        ELSE '❌ POOR'
    END AS grade
FROM monthly_quality
ORDER BY month DESC;

\echo ''

-- ============================================================
-- 8. Cross-Check: Compare Total Races vs Total Runners
-- ============================================================

\echo '🔗 RELATIONSHIP INTEGRITY CHECK'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

SELECT 
    'Total Races in DB' AS metric,
    COUNT(*)::text AS value
FROM races
WHERE race_date >= '2015-01-01'

UNION ALL

SELECT 
    'Total Runners in DB',
    COUNT(*)::text
FROM runners r
JOIN races ra ON ra.race_id = r.race_id
WHERE ra.race_date >= '2015-01-01'
  AND r.pos_num IS NOT NULL

UNION ALL

SELECT 
    'Unique Horses',
    COUNT(DISTINCT horse_id)::text
FROM horses

UNION ALL

SELECT 
    'Unique Trainers',
    COUNT(*)::text
FROM trainers

UNION ALL

SELECT 
    'Unique Jockeys',
    COUNT(*)::text
FROM jockeys

UNION ALL

SELECT 
    'Unique Courses',
    COUNT(*)::text
FROM courses;

\echo ''
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'
\echo '  ✅ VERIFICATION COMPLETE'
\echo '━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━'

