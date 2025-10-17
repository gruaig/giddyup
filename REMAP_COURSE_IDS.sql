-- Remap Old Course IDs to New Course IDs
-- Problem: Old Racing Post data has course_id=73 for Ascot, but new data has course_id=16591
-- Solution: UPDATE all races to use the NEW course_ids

BEGIN;

-- Create mapping table
CREATE TEMP TABLE course_id_remap (
    old_id BIGINT,
    new_id BIGINT,
    course_name TEXT,
    region TEXT
);

-- Insert known mappings (to be populated based on analysis)
-- Format: old_id, new_id, course_name, region

-- These need to be discovered by checking which courses already exist
-- For example:
-- INSERT INTO course_id_remap VALUES (73, 16591, 'Ascot', 'GB');  -- IF Ascot exists as 16591

-- Show what we're dealing with
SELECT 'Orphaned course_ids by region:' as info;
SELECT region, COUNT(DISTINCT course_id) as orphaned_courses
FROM racing.races
WHERE course_id NOT IN (SELECT course_id FROM racing.courses)
GROUP BY region;

SELECT '';
SELECT 'Need to map these course_ids to existing courses or create new entries' as next_step;

ROLLBACK;

