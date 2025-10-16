-- Add Betfair selection ID and bookmaker odds to runners table
-- This enables easy matching with Betfair Exchange API without name normalization

ALTER TABLE racing.runners 
ADD COLUMN IF NOT EXISTS betfair_selection_id bigint,
ADD COLUMN IF NOT EXISTS best_odds double precision,
ADD COLUMN IF NOT EXISTS best_bookmaker varchar(100);

CREATE INDEX IF NOT EXISTS idx_runners_betfair_selection 
ON racing.runners(betfair_selection_id) 
WHERE betfair_selection_id IS NOT NULL;

COMMENT ON COLUMN racing.runners.betfair_selection_id IS 'Betfair Exchange selection ID for direct matching (from Sporting Life bookmaker odds)';
COMMENT ON COLUMN racing.runners.best_odds IS 'Best available decimal odds across all bookmakers at time of scrape';
COMMENT ON COLUMN racing.runners.best_bookmaker IS 'Which bookmaker offered the best odds';


