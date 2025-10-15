package repository

import (
	"fmt"
	"time"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type AngleRepository struct {
	db *database.DB
}

func NewAngleRepository(db *database.DB) *AngleRepository {
	return &AngleRepository{db: db}
}

// GetNearMissTodayQualifiers returns horses qualifying for near-miss-no-hike angle today
func (r *AngleRepository) GetNearMissTodayQualifiers(params models.NearMissTodayParams) ([]models.NearMissQualifier, error) {
	// Set defaults
	if params.LastPos == 0 {
		params.LastPos = 2
	}
	if params.BTNMax == 0 {
		params.BTNMax = 3.0
	}
	if params.DSRMax == 0 {
		params.DSRMax = 14
	}
	if params.DistFTolerance == 0 {
		params.DistFTolerance = 1.0
	}
	if params.Limit == 0 {
		params.Limit = 200
	}
	// SameSurface defaults to true via param binding

	targetDate := time.Now().Format("2006-01-02")
	if params.On != nil {
		targetDate = *params.On
	}

	query := `
		WITH entries AS (
			SELECT
				r.runner_id,
				r.horse_id,
				r."or"        AS entry_or,
				ra.race_id,
				ra.race_date,
				ra.race_type,
				ra.dist_f,
				ra.surface,
				ra.going,
				ra.course_id
			FROM racing.runners r
			JOIN racing.races ra ON ra.race_id = r.race_id
			WHERE ra.race_date = $1
				AND r.pos_raw IS NULL
	`

	args := []interface{}{targetDate}
	argCount := 1

	if params.RaceType != nil {
		argCount++
		query += fmt.Sprintf(" AND ra.race_type = $%d", argCount)
		args = append(args, *params.RaceType)
	}

	query += `
		),
		last_run AS (
			SELECT x.*
			FROM (
				SELECT
					r.horse_id,
					r.runner_id   AS last_runner_id,
					r.race_id     AS last_race_id,
					r.race_date   AS last_date,
					r.pos_num     AS last_pos,
					r.btn         AS last_btn,
					r."or"        AS last_or,
					ra.race_type  AS last_race_type,
					ra.class      AS last_class,
					ra.dist_f     AS last_dist_f,
					ra.surface    AS last_surface,
					ra.going      AS last_going,
					ra.course_id  AS last_course_id,
					row_number() OVER (PARTITION BY r.horse_id ORDER BY r.race_date DESC) AS rn
				FROM racing.runners r
				JOIN racing.races ra ON ra.race_id = r.race_id
				WHERE r.pos_num IS NOT NULL
					AND r.race_date < $1
			) x
			WHERE x.rn = 1
		)
		SELECT
			h.horse_id,
			h.horse_name,
			e.race_id   AS next_race_id,
			e.race_date AS next_date,
			e.race_type AS next_race_type,
			e.course_id AS next_course_id,
			e.dist_f    AS next_dist_f,
			e.surface   AS next_surface,
			e.going     AS next_going,
			e.entry_or  AS next_or,
			l.last_race_id,
			l.last_date,
			l.last_pos,
			l.last_btn,
			l.last_or,
			l.last_class,
			l.last_dist_f,
			l.last_surface,
			l.last_going,
			(e.race_date - l.last_date)::int                 AS dsr,
			(COALESCE(e.entry_or, 0) - COALESCE(l.last_or, 0))::int AS rating_change,
			ABS(COALESCE(e.dist_f, 0) - COALESCE(l.last_dist_f, 0))  AS dist_f_diff,
			(e.surface = l.last_surface)                     AS same_surface
		FROM entries e
		JOIN last_run l ON l.horse_id = e.horse_id
		JOIN racing.horses h ON h.horse_id = e.horse_id
		WHERE l.last_pos = $2
	`

	argCount++
	args = append(args, params.LastPos)

	argCount++
	query += fmt.Sprintf(" AND COALESCE(l.last_btn, 99.0) <= $%d", argCount)
	args = append(args, params.BTNMax)

	argCount++
	query += fmt.Sprintf(" AND (e.race_date - l.last_date) <= $%d", argCount)
	args = append(args, params.DSRMax)

	// OR delta filter
	if !params.IncludeNullOR {
		query += " AND e.entry_or IS NOT NULL AND l.last_or IS NOT NULL"
	}

	argCount++
	argCount2 := argCount + 1
	query += fmt.Sprintf(" AND (COALESCE(e.entry_or, l.last_or) - l.last_or) <= $%d", argCount)
	args = append(args, params.ORDeltaMax)

	argCount = argCount2
	query += fmt.Sprintf(" AND ABS(COALESCE(e.dist_f, 0) - COALESCE(l.last_dist_f, 0)) <= $%d", argCount)
	args = append(args, params.DistFTolerance)

	if params.SameSurface {
		query += " AND e.surface = l.last_surface"
	}

	query += " ORDER BY e.race_date, e.race_id, h.horse_name"

	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, params.Limit)

	if params.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, params.Offset)
	}

	var qualifiers []models.NearMissQualifier
	if err := r.db.Select(&qualifiers, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get today qualifiers: %w", err)
	}

	return qualifiers, nil
}

// GetNearMissPastCases returns historical backtest cases for near-miss-no-hike angle
func (r *AngleRepository) GetNearMissPastCases(params models.NearMissPastParams) (*models.NearMissPastResponse, error) {
	// Set defaults
	if params.LastPos == 0 {
		params.LastPos = 2
	}
	if params.BTNMax == 0 {
		params.BTNMax = 3.0
	}
	if params.DSRMax == 0 {
		params.DSRMax = 14
	}
	if params.DistFTolerance == 0 {
		params.DistFTolerance = 1.0
	}
	if params.Limit == 0 {
		params.Limit = 200
	}
	if params.PriceSource == "" {
		params.PriceSource = "bsp"
	}

	// Build the query
	query := `
		WITH base AS (
			SELECT ln.*, r_next.dist_f AS next_dist_f, r_next.surface AS next_surface
			FROM mv_last_next ln
			JOIN racing.races r_next ON r_next.race_id = ln.next_race_id
			WHERE 1=1
	`

	args := []interface{}{}
	argCount := 0

	if params.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND ln.last_date >= $%d::date", argCount)
		args = append(args, *params.DateFrom)
	}

	if params.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND ln.last_date <= $%d::date", argCount)
		args = append(args, *params.DateTo)
	}

	if params.RaceType != nil {
		argCount++
		query += fmt.Sprintf(" AND ln.last_race_type = $%d", argCount)
		args = append(args, *params.RaceType)
	}

	query += `
		),
		filtered AS (
			SELECT
				b.*,
				(b.next_or - b.last_or) AS rating_change,
				ABS(b.next_dist_f - b.last_dist_f) AS dist_f_diff,
				(b.next_surface = b.last_surface) AS same_surface
			FROM base b
			WHERE b.last_pos = $` + fmt.Sprintf("%d", argCount+1)

	argCount++
	args = append(args, params.LastPos)

	argCount++
	query += fmt.Sprintf(" AND COALESCE(b.last_btn, 99.0) <= $%d", argCount)
	args = append(args, params.BTNMax)

	argCount++
	query += fmt.Sprintf(" AND b.dsr_next <= $%d", argCount)
	args = append(args, params.DSRMax)

	if !params.IncludeNullOR {
		query += " AND b.last_or IS NOT NULL AND b.next_or IS NOT NULL"
	}

	argCount++
	query += fmt.Sprintf(" AND (COALESCE(b.next_or, b.last_or) - b.last_or) <= $%d", argCount)
	args = append(args, params.ORDeltaMax)

	argCount++
	query += fmt.Sprintf(" AND ABS(b.next_dist_f - b.last_dist_f) <= $%d", argCount)
	args = append(args, params.DistFTolerance)

	if params.SameSurface {
		query += " AND b.next_surface = b.last_surface"
	}

	if params.RequireNextWin {
		query += " AND b.next_win = TRUE"
	}

	query += `
		),
		priced AS (
			SELECT
				f.*,
				CASE $` + fmt.Sprintf("%d", argCount+1) + `
					WHEN 'bsp'   THEN (SELECT r2.win_bsp FROM racing.runners r2 WHERE r2.runner_id = f.next_runner_id)
					WHEN 'dec'   THEN (SELECT r2.dec FROM racing.runners r2 WHERE r2.runner_id = f.next_runner_id)
					WHEN 'ppwap' THEN (SELECT r2.win_ppwap FROM racing.runners r2 WHERE r2.runner_id = f.next_runner_id)
				END AS price
			FROM filtered f
		)
		SELECT
			h.horse_name,
			p.horse_id,
			p.last_race_id,
			p.last_date,
			p.last_pos,
			p.last_btn,
			p.last_or,
			p.last_class,
			p.last_dist_f,
			p.last_surface,
			p.next_race_id,
			p.next_date,
			p.next_pos,
			p.next_win,
			p.next_or,
			p.next_dist_f,
			p.next_surface,
			p.dsr_next AS dsr,
			p.rating_change,
			p.dist_f_diff,
			p.same_surface,
			p.price
		FROM priced p
		JOIN racing.horses h ON h.horse_id = p.horse_id
		ORDER BY p.last_date DESC
	`

	argCount++
	args = append(args, params.PriceSource)

	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, params.Limit)

	if params.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, params.Offset)
	}

	var cases []models.NearMissPastCase
	if err := r.db.Select(&cases, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get past cases: %w", err)
	}

	response := &models.NearMissPastResponse{
		Cases: cases,
	}

	// Calculate summary if requested
	if params.Summary && len(cases) > 0 {
		summary, err := r.calculateAngleSummary(cases)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate summary: %w", err)
		}
		response.Summary = summary
	}

	return response, nil
}

// calculateAngleSummary computes aggregate metrics from cases
func (r *AngleRepository) calculateAngleSummary(cases []models.NearMissPastCase) (*models.AngleSummary, error) {
	n := len(cases)
	wins := 0
	totalROI := 0.0
	priceCount := 0

	for _, c := range cases {
		if c.NextWin {
			wins++
		}
		if c.Price != nil && *c.Price > 0 {
			if c.NextWin {
				totalROI += (*c.Price - 1)
			} else {
				totalROI += -1
			}
			priceCount++
		}
	}

	winRate := 0.0
	if n > 0 {
		winRate = float64(wins) / float64(n)
	}

	var roi *float64
	if priceCount > 0 {
		roiValue := totalROI / float64(priceCount)
		roi = &roiValue
	}

	return &models.AngleSummary{
		N:       n,
		Wins:    wins,
		WinRate: winRate,
		ROI:     roi,
	}, nil
}

// CreateLastNextMaterializedView creates the mv_last_next materialized view
func (r *AngleRepository) CreateLastNextMaterializedView() error {
	query := `
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
			FROM racing.runners r
			JOIN racing.races ra ON ra.race_id = r.race_id
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
	`

	if _, err := r.db.Exec(query); err != nil {
		return fmt.Errorf("failed to create materialized view: %w", err)
	}

	// Create indexes on the materialized view
	indexQueries := []string{
		"CREATE INDEX IF NOT EXISTS mvln_dates_idx ON mv_last_next (last_date, next_date)",
		"CREATE INDEX IF NOT EXISTS mvln_filter_idx ON mv_last_next (last_pos, last_btn, dsr_next)",
		"CREATE INDEX IF NOT EXISTS mvln_horse_idx ON mv_last_next (horse_id)",
	}

	for _, idxQuery := range indexQueries {
		if _, err := r.db.Exec(idxQuery); err != nil {
			return fmt.Errorf("failed to create index: %w", err)
		}
	}

	return nil
}

// RefreshLastNextMaterializedView refreshes the mv_last_next view
func (r *AngleRepository) RefreshLastNextMaterializedView() error {
	if _, err := r.db.Exec("REFRESH MATERIALIZED VIEW CONCURRENTLY mv_last_next"); err != nil {
		return fmt.Errorf("failed to refresh materialized view: %w", err)
	}
	return nil
}

