-- Data Updates Tracking Table
-- Tracks all data update operations (daily scrapes, backfills, etc.)

CREATE TABLE IF NOT EXISTS racing.data_updates (
    update_id SERIAL PRIMARY KEY,
    update_type VARCHAR(50) NOT NULL,  -- 'daily', 'racecard', 'backfill'
    update_date DATE NOT NULL,
    started_at TIMESTAMP NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP,
    status VARCHAR(20) NOT NULL DEFAULT 'running',  -- 'running', 'completed', 'failed'
    
    -- Progress flags
    racing_post_scraped BOOLEAN DEFAULT FALSE,
    betfair_fetched BOOLEAN DEFAULT FALSE,
    data_stitched BOOLEAN DEFAULT FALSE,
    data_loaded BOOLEAN DEFAULT FALSE,
    
    -- Statistics
    races_scraped INT DEFAULT 0,
    runners_scraped INT DEFAULT 0,
    races_matched INT DEFAULT 0,
    races_loaded INT DEFAULT 0,
    runners_loaded INT DEFAULT 0,
    
    -- Error tracking
    error_message TEXT,
    
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_data_updates_date ON racing.data_updates(update_date);
CREATE INDEX IF NOT EXISTS idx_data_updates_status ON racing.data_updates(update_type, status);
CREATE INDEX IF NOT EXISTS idx_data_updates_type_date ON racing.data_updates(update_type, update_date);

-- View to quickly check latest update status
CREATE OR REPLACE VIEW racing.update_status AS
SELECT 
    update_date,
    MAX(CASE WHEN status = 'completed' THEN update_id END) as latest_successful_id,
    MAX(CASE WHEN status = 'completed' THEN completed_at END) as last_updated_at,
    COUNT(CASE WHEN status = 'failed' THEN 1 END) as failure_count,
    MAX(CASE WHEN status = 'completed' THEN races_loaded END) as races_loaded,
    MAX(CASE WHEN status = 'completed' THEN runners_loaded END) as runners_loaded
FROM racing.data_updates
WHERE update_type = 'daily'
GROUP BY update_date
ORDER BY update_date DESC;

COMMENT ON TABLE racing.data_updates IS 'Tracks all data update operations including daily scrapes, racecards, and backfills';
COMMENT ON VIEW racing.update_status IS 'Quick summary of latest update status by date';

