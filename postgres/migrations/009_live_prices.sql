-- Migration: Add support for today's preliminary races and live prices
-- Date: 2025-10-15

-- Add prelim flag to track incomplete/preliminary race data
ALTER TABLE racing.races ADD COLUMN IF NOT EXISTS prelim boolean DEFAULT false;

-- Create live prices table for intraday price updates
CREATE TABLE IF NOT EXISTS racing.live_prices (
  race_id bigint NOT NULL,
  runner_id bigint NOT NULL,
  ts timestamptz NOT NULL,
  back_price double precision,
  lay_price double precision,
  vwap double precision,
  traded_vol double precision,
  PRIMARY KEY (runner_id, ts)
);

-- Indices for efficient live price queries
CREATE INDEX IF NOT EXISTS idx_live_prices_race_ts ON racing.live_prices(race_id, ts DESC);
CREATE INDEX IF NOT EXISTS idx_live_prices_runner_latest ON racing.live_prices(runner_id, ts DESC);

-- Add comment
COMMENT ON TABLE racing.live_prices IS 'Stores intraday Betfair exchange prices for today''s races (updated every 60s)';
COMMENT ON COLUMN racing.races.prelim IS 'True for preliminary data (racecards), false when results are complete';

