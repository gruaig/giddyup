-- Complete fix for Oct 18 data
-- 1. Ensure no duplicates
-- 2. Re-fetch with proper selection IDs
-- 3. Update with Betfair prices

BEGIN;

-- Delete old Oct 18 data
DELETE FROM racing.runners WHERE race_date = '2025-10-18';
DELETE FROM racing.races WHERE race_date = '2025-10-18';

COMMIT;

-- Now re-run: ./fetch_all 2025-10-18
-- Then run: ./bin/update_live_prices --date=2025-10-18
