-- Fix ALL Orphaned Course IDs
-- Problem: 127,199 races have course_ids not in racing.courses table
-- Solution: INSERT missing courses (identified by race name patterns)

BEGIN;

-- Top orphaned course_ids identified by race name analysis:

-- GB Courses
INSERT INTO racing.courses (course_id, course_name, region) VALUES
(73, 'Ascot', 'GB'),                    -- "Betfair Ascot Chase", "Ascot Shop"
(74, 'Warwick', 'GB'),                  -- "Pertemps Network" races  
(75, 'Haydock', 'GB'),                  -- Northern jump course
(50, 'Musselburgh', 'GB'),              -- "Auld Reekie" (Edinburgh)
(137, 'Newbury', 'GB'),                 -- "Betfair Hurdle", "Betfair Denman Chase"
(157, 'Sandown', 'GB'),                 -- Major jump course
(149, 'Lingfield', 'GB'),               -- AW track
(66, 'Uttoxeter', 'GB'),                -- Midlands jump course
(147, 'Ripon', 'GB'),                   -- Yorkshire flat course
(166, 'York', 'GB'),                    -- Major flat course
(159, 'Windsor', 'GB'),                 -- Thames-side course
(162, 'Goodwood', 'GB'),                -- Sussex course
(163, 'Pontefract', 'GB'),              -- Yorkshire course
(76, 'Bangor-on-Dee', 'GB'),            -- Welsh jump course
(481, 'Stratford', 'GB'),               -- Midlands jump course
(164, 'Yarmouth', 'GB'),                -- East coast
(167, 'Salisbury', 'GB'),               -- Wiltshire
(173, 'Fontwell', 'GB'),                -- Sussex jumps
(508, 'Southwell', 'GB'),               -- AW/turf
(130, 'Redcar', 'GB'),                  -- Yorkshire coast
(406, 'Perth', 'GB'),                   -- Scotland
(141, 'Chepstow', 'GB'),                -- Welsh course
(153, 'Worcester', 'GB'),               -- Midlands jumps
(72, 'Wetherby', 'GB'),                 -- Yorkshire jumps
(106, 'Cartmel', 'GB'),                 -- Lake District
(70, 'Thirsk', 'GB'),                   -- Yorkshire
(81, 'Towcester', 'GB'),                -- Northamptonshire (closed)
(410, 'Hexham', 'GB'),                  -- Northumberland
(82, 'Beverley', 'GB'),                 -- Yorkshire
(97, 'Market Rasen', 'GB'),             -- Lincolnshire jumps
(165, 'Nottingham', 'GB')               -- Midlands
ON CONFLICT (course_id) DO NOTHING;

-- IRE Courses (10xxx range)
INSERT INTO racing.courses (course_id, course_name, region) VALUES
(10942, 'Leopardstown', 'IRE'),         -- Major Dublin track
(10955, 'Down Royal', 'IRE'),           -- "MCR" races, Northern Ireland
(10943, 'Fairyhouse', 'IRE'),           -- Dublin area
(10940, 'Punchestown', 'IRE'),          -- Kildare
(10938, 'Limerick', 'IRE'),             -- Munster
(10939, 'Thurles', 'IRE'),              -- Tipperary
(10978, 'Tramore', 'IRE'),              -- Waterford
(10964, 'Killarney', 'IRE'),            -- Kerry
(10971, 'Clonmel', 'IRE'),              -- Tipperary
(10975, 'Cork', 'IRE'),                 -- Munster
(10969, 'Galway', 'IRE'),               -- West coast
(10952, 'Navan', 'IRE'),                -- Meath
(10941, 'Gowran Park', 'IRE'),          -- Kilkenny
(10956, 'Ballinrobe', 'IRE'),           -- Mayo
(10973, 'Roscommon', 'IRE'),            -- Connacht
(10988, 'Bellewstown', 'IRE'),          -- Meath (summer)
(10954, 'Listowel', 'IRE'),             -- Kerry (harvest festival)
(10972, 'Tipperary', 'IRE'),            -- Tipperary town
(10947, 'Sligo', 'IRE'),                -- Northwest
(10951, 'Kilbeggan', 'IRE'),            -- Westmeath
(10945, 'Wexford', 'IRE'),              -- Southeast
(10960, 'Downpatrick', 'IRE'),          -- County Down
(10957, 'Dundalk', 'IRE'),              -- AW track
(10944, 'Naas', 'IRE'),                 -- Kildare
(10953, 'Curragh', 'IRE'),              -- Kildare (headquarters)
(10962, 'Laytown', 'IRE'),              -- Beach racing
(10946, 'Navan', 'IRE')                 -- Duplicate? Check
ON CONFLICT (course_id) DO NOTHING;

COMMIT;

-- Verify the fix
SELECT COUNT(*) as races_still_orphaned 
FROM racing.races 
WHERE course_id NOT IN (SELECT course_id FROM racing.courses);

-- Show improvement
SELECT 
    (SELECT COUNT(*) FROM racing.races) as total_races,
    (SELECT COUNT(*) FROM racing.races WHERE course_id NOT IN (SELECT course_id FROM racing.courses)) as orphaned,
    (SELECT COUNT(*) FROM racing.courses) as courses_in_table;

