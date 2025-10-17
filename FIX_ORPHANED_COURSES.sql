-- Fix Orphaned Course IDs
-- Problem: 127,199 races (56% of database) have course_ids that don't exist in racing.courses
-- Solution: Insert missing courses OR remap course_ids

-- Step 1: Identify orphaned course_ids and their likely course names
-- (Based on race name analysis)

BEGIN;

-- Insert missing courses based on race name patterns
-- Note: These are educated guesses based on race names and regions

-- course_id 10942: Leopardstown (IRE) - "Pierse Leopardstown" races
INSERT INTO racing.courses (course_id, course_name, region) 
VALUES (10942, 'Leopardstown', 'IRE')
ON CONFLICT (course_id) DO NOTHING;

-- course_id 75: Haydock (GB) - Northern jump course
INSERT INTO racing.courses (course_id, course_name, region) 
VALUES (75, 'Haydock', 'GB')
ON CONFLICT (course_id) DO NOTHING;

-- course_id 162: Goodwood (GB) - "Goodwood Park" races
INSERT INTO racing.courses (course_id, course_name, region) 
VALUES (162, 'Goodwood', 'GB')
ON CONFLICT (course_id) DO NOTHING;

-- course_id 137: Need to identify
-- course_id 74: Need to identify
-- course_id 50: Need to identify

-- TO BE CONTINUED: Need to analyze race names for other course_ids

ROLLBACK; -- Don't commit yet, just testing

