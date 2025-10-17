-- Migration 012: Add price_updated_at timestamp
-- Purpose: Track when Betfair prices were last fetched for UI display

BEGIN;

-- Add column to runners table (will inherit to all partitions)
ALTER TABLE racing.runners 
ADD COLUMN IF NOT EXISTS price_updated_at TIMESTAMP DEFAULT NULL;

-- Add comment for documentation
COMMENT ON COLUMN racing.runners.price_updated_at IS 
'Timestamp when Betfair win_ppwap was last updated. NULL means never updated or using historical BSP.';

-- Create index for efficient queries
CREATE INDEX IF NOT EXISTS idx_runners_price_updated 
ON racing.runners(price_updated_at) 
WHERE price_updated_at IS NOT NULL;

COMMIT;

-- Verify
SELECT 
    column_name,
    data_type,
    is_nullable,
    column_default
FROM information_schema.columns
WHERE table_schema = 'racing'
AND table_name = 'runners'
AND column_name = 'price_updated_at';

