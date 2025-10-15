-- Migration 001: ETL Run Tracking and Ingested Days
-- Purpose: Track data ingestion runs and prevent duplicate imports
-- Run: psql -U postgres -d giddyup -f 001_ingest_tracking.sql

SET search_path TO racing, public;

-- ETL Runs: Track each ingestion run with scope, stats, and status
CREATE TABLE IF NOT EXISTS etl_runs (
  run_id       bigserial PRIMARY KEY,
  started_at   timestamptz NOT NULL DEFAULT now(),
  finished_at  timestamptz,
  status       text NOT NULL CHECK (status IN ('running','success','failed','canceled')),
  scope        jsonb NOT NULL,       -- {"regions":["gb"],"codes":["flat"],"dates":["2025-10-13"]}
  stats        jsonb,                -- {"races_inserted":125,"runners_inserted":1350,"errors":0}
  error_msg    text,
  duration_ms  bigint,
  CONSTRAINT status_final_check CHECK (
    (status IN ('success','failed','canceled') AND finished_at IS NOT NULL) OR
    (status = 'running' AND finished_at IS NULL)
  )
);

CREATE INDEX IF NOT EXISTS idx_etl_runs_status ON etl_runs(status);
CREATE INDEX IF NOT EXISTS idx_etl_runs_started ON etl_runs(started_at DESC);

-- Ingested Days: Track what data has been loaded (for idempotency)
CREATE TABLE IF NOT EXISTS ingested_days (
  region text NOT NULL,
  code   text NOT NULL,
  d      date NOT NULL,
  run_id bigint REFERENCES etl_runs(run_id),
  inserted_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(region, code, d)
);

CREATE INDEX IF NOT EXISTS idx_ingested_days_date ON ingested_days(d DESC);

-- Helper indexes for gap detection (if not already present)
CREATE INDEX IF NOT EXISTS idx_runners_race    ON runners(race_id) WHERE pos_num IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_races_date      ON races(race_date);
CREATE INDEX IF NOT EXISTS idx_runners_horse_d ON runners(horse_id, race_date DESC);
CREATE INDEX IF NOT EXISTS idx_runners_unraced ON runners(race_id) WHERE pos_raw IS NULL;

-- Comment FTS (if not already present)
CREATE INDEX IF NOT EXISTS idx_runners_comment_fts
  ON runners USING GIN (to_tsvector('english', coalesce(comment,'')));

-- Advisory lock function for exclusive ingestion
COMMENT ON TABLE etl_runs IS 'Tracks data ingestion runs with scope, status, and performance metrics';
COMMENT ON TABLE ingested_days IS 'Prevents duplicate data imports by tracking which region/code/dates have been loaded';

-- Example advisory lock usage:
-- SELECT pg_try_advisory_lock(123456789);  -- returns true if acquired
-- SELECT pg_advisory_unlock(123456789);     -- release lock

-- Grant permissions (adjust as needed)
GRANT SELECT, INSERT, UPDATE ON etl_runs TO postgres;
GRANT SELECT, INSERT ON ingested_days TO postgres;
GRANT USAGE, SELECT ON SEQUENCE etl_runs_run_id_seq TO postgres;

\echo '✅ Migration 001 complete: ETL tracking tables created'

