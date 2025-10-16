-- Check for duplicate races in tomorrow's data
SELECT 
  race_date,
  COUNT(*) as total_races,
  COUNT(DISTINCT race_key) as unique_keys,
  COUNT(*) - COUNT(DISTINCT race_key) as duplicates
FROM racing.races
WHERE race_date = (CURRENT_DATE + INTERVAL '1 day')::date
GROUP BY race_date;

-- Find specific duplicates
SELECT race_key, COUNT(*) as count, 
       string_agg(race_id::text, ', ') as race_ids,
       MAX(race_name) as race_name,
       MAX(off_time::text) as off_time
FROM racing.races
WHERE race_date = (CURRENT_DATE + INTERVAL '1 day')::date
GROUP BY race_key
HAVING COUNT(*) > 1
ORDER BY count DESC
LIMIT 10;
