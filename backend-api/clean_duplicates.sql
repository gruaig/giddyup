-- Clean up duplicate races from database
-- This removes duplicates while keeping the earliest race_id for each unique race

-- Step 1: Find duplicates
SELECT 
  race_date,
  COUNT(*) as total_races,
  COUNT(DISTINCT race_key) as unique_keys,
  COUNT(*) - COUNT(DISTINCT race_key) as duplicates
FROM racing.races
WHERE race_date >= '2025-10-16'
GROUP BY race_date
ORDER BY race_date;

-- Step 2: See which race_keys have duplicates
SELECT race_key, race_date, COUNT(*) as count, 
       string_agg(race_id::text, ', ') as race_ids,
       MAX(race_name) as race_name
FROM racing.races
WHERE race_date >= '2025-10-16'
GROUP BY race_key, race_date
HAVING COUNT(*) > 1
ORDER BY race_date, count DESC;

-- Step 3: Delete duplicates (keeps MIN race_id for each race_key)
-- This will cascade delete runners too due to foreign keys
DELETE FROM racing.races
WHERE race_id IN (
  SELECT race_id
  FROM (
    SELECT race_id,
           ROW_NUMBER() OVER (PARTITION BY race_key, race_date ORDER BY race_id) as rn
    FROM racing.races
    WHERE race_date >= '2025-10-16'
  ) t
  WHERE rn > 1
);

-- Step 4: Verify cleanup
SELECT race_date, COUNT(*) as total, COUNT(DISTINCT race_key) as unique
FROM racing.races
WHERE race_date >= '2025-10-16'
GROUP BY race_date
ORDER BY race_date;

