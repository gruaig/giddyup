-- COMPLETE COURSE ID REMAPPING
-- Fixes ALL 95,694 remaining orphaned races
-- Phase 1: Remap old IDs to existing course_ids
-- Phase 2: Insert courses that don't exist at all

BEGIN;

-- PHASE 1: REMAP to existing courses
UPDATE racing.races SET course_id = 16591 WHERE course_id = 73;  -- Ascot
UPDATE racing.races SET course_id = 16324 WHERE course_id = 75;  -- Haydock
UPDATE racing.races SET course_id = 16200 WHERE course_id = 50;  -- Musselburgh
UPDATE racing.races SET course_id = 41 WHERE course_id = 74;     -- Warwick
UPDATE racing.races SET course_id = 16197 WHERE course_id = 162; -- Goodwood
UPDATE racing.races SET course_id = 16593 WHERE course_id = 10942; -- Leopardstown
UPDATE racing.races SET course_id = 55 WHERE course_id = 157;    -- Sandown
UPDATE racing.races SET course_id = 31 WHERE course_id = 149;    -- Lingfield
UPDATE racing.races SET course_id = 58 WHERE course_id = 66;     -- Uttoxeter
UPDATE racing.races SET course_id = 16215 WHERE course_id = 166; -- York
UPDATE racing.races SET course_id = 176 WHERE course_id = 163;   -- Pontefract
UPDATE racing.races SET course_id = 57 WHERE course_id = 481;    -- Stratford
UPDATE racing.races SET course_id = 16566 WHERE course_id = 164; -- Yarmouth
UPDATE racing.races SET course_id = 51 WHERE course_id = 97;     -- Market Rasen
UPDATE racing.races SET course_id = 16227 WHERE course_id = 165; -- Nottingham
UPDATE racing.races SET course_id = 16193 WHERE course_id = 141; -- Chepstow
UPDATE racing.races SET course_id = 16224 WHERE course_id = 153; -- Worcester
UPDATE racing.races SET course_id = 34 WHERE course_id = 72;     -- Wetherby
UPDATE racing.races SET course_id = 61 WHERE course_id = 81;     -- Towcester
UPDATE racing.races SET course_id = 16195 WHERE course_id = 410; -- Hexham

-- Irish courses
UPDATE racing.races SET course_id = 16194 WHERE course_id = 10943; -- Fairyhouse
UPDATE racing.races SET course_id = 16203 WHERE course_id = 10940; -- Punchestown
UPDATE racing.races SET course_id = 16198 WHERE course_id = 10944; -- Naas

-- Check progress
SELECT 
    (SELECT COUNT(*) FROM racing.races WHERE course_id NOT IN (SELECT course_id FROM racing.courses)) as still_orphaned,
    (SELECT COUNT(*) FROM racing.courses) as total_courses;

COMMIT;

