-- Create Materialized View for Last→Next Run Pairs
-- Used by angles/near-miss-no-hike/past endpoint

SET search_path TO racing, public;

\echo 'Creating materialized view mv_last_next...'

CREATE MATERIALIZED VIEW IF NOT EXISTS mv_last_next AS
WITH ordered AS (
	SELECT
		r.horse_id,
		r.runner_id,
		r.race_id,
		r.race_date,
		r.pos_num,
		r.btn,
		r."or"           AS or_now,
		ra.race_type,
		ra.class,
		ra.dist_f,
		ra.surface,
		ra.going,
		ra.course_id,
		r.win_bsp,
		r.dec,
		r.win_ppwap,
		r.win_flag,
		LEAD(r.runner_id) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_runner_id,
		LEAD(r.race_id)   OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_race_id,
		LEAD(r.race_date) OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_date,
		LEAD(r.pos_num)   OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_pos,
		LEAD(r.win_flag)  OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_win,
		LEAD(r."or")      OVER (PARTITION BY r.horse_id ORDER BY r.race_date) AS next_or
	FROM runners r
	JOIN races ra ON ra.race_id = r.race_id
	WHERE r.pos_num IS NOT NULL
)
SELECT
	o.horse_id,
	o.runner_id      AS last_runner_id,
	o.race_id        AS last_race_id,
	o.race_date      AS last_date,
	o.pos_num        AS last_pos,
	o.btn            AS last_btn,
	o.or_now         AS last_or,
	o.race_type      AS last_race_type,
	o.class          AS last_class,
	o.dist_f         AS last_dist_f,
	o.surface        AS last_surface,
	o.going          AS last_going,
	o.course_id      AS last_course_id,
	o.next_runner_id,
	o.next_race_id,
	o.next_date,
	o.next_pos,
	o.next_win,
	o.next_or,
	(o.next_date - o.race_date) AS dsr_next
FROM ordered o
WHERE o.next_race_id IS NOT NULL;

\echo 'Creating indexes on mv_last_next...'

CREATE INDEX IF NOT EXISTS mvln_dates_idx ON mv_last_next (last_date, next_date);
CREATE INDEX IF NOT EXISTS mvln_filter_idx ON mv_last_next (last_pos, last_btn, dsr_next);
CREATE INDEX IF NOT EXISTS mvln_horse_idx ON mv_last_next (horse_id);

\echo 'Running ANALYZE...'
ANALYZE mv_last_next;

\echo ''
\echo '✅ Materialized view created successfully!'
\echo ''
\echo 'Row count:'
SELECT COUNT(*) FROM mv_last_next;

\echo ''
\echo 'To refresh after loading new data:'
\echo '  REFRESH MATERIALIZED VIEW CONCURRENTLY mv_last_next;'

