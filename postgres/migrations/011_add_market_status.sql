-- Migration: Add market_status computed field to races

-- Add function to compute market status based on current time
CREATE OR REPLACE FUNCTION racing.compute_market_status(
    race_date_param DATE,
    off_time_param TIME
) RETURNS TEXT AS $$
DECLARE
    race_timestamp TIMESTAMP WITH TIME ZONE;
    current_timestamp TIMESTAMP WITH TIME ZONE;
BEGIN
    -- Combine date + time to get full timestamp (assume UK timezone)
    race_timestamp := (race_date_param || ' ' || off_time_param)::TIMESTAMP AT TIME ZONE 'Europe/London';
    current_timestamp := NOW();
    
    -- If race is in the past (more than 30 minutes ago), it's finished
    IF current_timestamp > (race_timestamp + INTERVAL '30 minutes') THEN
        RETURN 'Finished';
    -- If race is happening soon or now (within next 10 minutes to 30 minutes after), it's active
    ELSIF current_timestamp > (race_timestamp - INTERVAL '10 minutes') THEN
        RETURN 'Active';
    -- Otherwise, it's upcoming
    ELSE
        RETURN 'Upcoming';
    END IF;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Add computed column to races table (will recalculate on each query)
-- Note: We can't use GENERATED ALWAYS with a function that uses NOW()
-- So we'll add this as a regular column and compute it in the API

-- Instead, add a view that includes the status
CREATE OR REPLACE VIEW racing.races_with_status AS
SELECT 
    r.*,
    racing.compute_market_status(r.race_date, r.off_time) AS market_status
FROM racing.races r;

COMMENT ON VIEW racing.races_with_status IS 'Races with computed market_status (Finished/Active/Upcoming)';
