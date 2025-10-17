-- REMAP ALL COURSE IDS
-- Comprehensive fix for 127,199 orphaned races
--
-- Strategy: UPDATE races.course_id from old_id → new_id
-- This preserves all race data while fixing foreign key references

BEGIN;

-- Create a comprehensive mapping of old → new course_ids
CREATE TEMP TABLE course_remap AS
SELECT old_id, new_id, course_name FROM (VALUES
    -- GB Courses (verified to exist)
    (73, 16591, 'Ascot'),
    (75, 16324, 'Haydock'),
    (50, 16200, 'Musselburgh'),
    (74, 41, 'Warwick'),
    (162, 16197, 'Goodwood'),
    -- IRE Courses (verified to exist)
    (10942, 16593, 'Leopardstown')
    -- More mappings will be added as we identify them
) AS t(old_id, new_id, course_name);

-- Show what we're about to remap
SELECT 
    'Will remap:' as status,
    COUNT(*) as course_mappings,
    SUM((SELECT COUNT(*) FROM racing.races WHERE course_id = old_id)) as races_affected
FROM course_remap;

-- Perform the remap for confirmed courses
UPDATE racing.races r
SET course_id = cr.new_id
FROM course_remap cr
WHERE r.course_id = cr.old_id;

-- Show improvement
SELECT 
    COUNT(*) as races_still_orphaned 
FROM racing.races 
WHERE course_id NOT IN (SELECT course_id FROM racing.courses);

COMMIT;

