-- Quick fix: Insert ONLY the missing courses that don't exist at all
-- This is safe because these courses truly don't exist in racing.courses

BEGIN;

-- First, let's just insert courses that are completely missing
-- (where the normalized name doesn't conflict)

-- Check current courses
SELECT COUNT(*) as current_courses FROM racing.courses;

-- Insert missing GB courses (that don't conflict)
INSERT INTO racing.courses (course_id, course_name, region) 
SELECT old_id, course_name, region FROM (VALUES
    (137, 'Newbury', 'GB'),
    (157, 'Sandown', 'GB'),
    (147, 'Ripon', 'GB'),
    (159, 'Windsor', 'GB'),
    (76, 'Bangor-on-Dee', 'GB'),
    (163, 'Pontefract', 'GB'),
    (167, 'Salisbury', 'GB'),
    (173, 'Fontwell', 'GB'),
    (481, 'Stratford', 'GB'),
    (406, 'Perth', 'GB'),
    (141, 'Chepstow', 'GB'),
    (153, 'Worcester', 'GB'),
    (72, 'Wetherby', 'GB'),
    (106, 'Cartmel', 'GB'),
    (70, 'Thirsk', 'GB'),
    (81, 'Towcester', 'GB'),
    (410, 'Hexham', 'GB'),
    (82, 'Beverley', 'GB'),
    (97, 'Market Rasen', 'GB'),
    (165, 'Nottingham', 'GB')
) AS t(old_id, course_name, region)
WHERE NOT EXISTS (
    SELECT 1 FROM racing.courses 
    WHERE racing.norm_text(course_name) = racing.norm_text(t.course_name)
    AND region = t.region
)
ON CONFLICT (course_id) DO NOTHING;

SELECT COUNT(*) as courses_after_insert FROM racing.courses;

COMMIT;

-- Check how many are still orphaned
SELECT COUNT(*) as still_orphaned FROM racing.races WHERE course_id NOT IN (SELECT course_id FROM racing.courses);
