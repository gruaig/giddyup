package repository

import (
	"fmt"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type ProfileRepository struct {
	db *database.DB
}

func NewProfileRepository(db *database.DB) *ProfileRepository {
	return &ProfileRepository{db: db}
}

// GetHorseProfile returns complete horse profile with all statistics
func (r *ProfileRepository) GetHorseProfile(horseID int64) (*models.HorseProfile, error) {
	profile := &models.HorseProfile{}

	// Get horse details
	horseQuery := `SELECT horse_id, horse_name FROM racing.horses WHERE horse_id = $1`
	if err := r.db.Get(&profile.Horse, horseQuery, horseID); err != nil {
		return nil, fmt.Errorf("failed to get horse: %w", err)
	}

	// Get career summary - using mv_runner_base for faster query
	summaryQuery := `
		SELECT 
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE win_flag) AS wins,
			COUNT(*) FILTER (WHERE pos_num <= 3) AS places,
			0 AS total_prize,
			AVG(rpr) FILTER (WHERE rpr IS NOT NULL) AS avg_rpr,
			MAX(rpr) AS peak_rpr,
			AVG("or") FILTER (WHERE "or" IS NOT NULL) AS avg_or,
			MAX("or") AS peak_or
		FROM mv_runner_base
		WHERE horse_id = $1
	`
	if err := r.db.Get(&profile.CareerSummary, summaryQuery, horseID); err != nil {
		return nil, fmt.Errorf("failed to get career summary: %w", err)
	}

	// Get recent form - using mv_runner_base for faster query
	formQuery := `
		SELECT 
			rb.race_date,
			c.course_name,
			''::text AS race_name,
			rb.race_type,
			rb.going,
			rb.dist_f,
			rb.pos_num,
			''::text AS pos_raw,
			rb.btn,
			rb."or",
			rb.rpr,
			rb.win_bsp,
			rb.dec,
			t.trainer_name,
			j.jockey_name,
			rb.race_date - LAG(rb.race_date) OVER (ORDER BY rb.race_date) AS dsr
		FROM mv_runner_base rb
		LEFT JOIN racing.courses c ON c.course_id = rb.course_id
		LEFT JOIN racing.trainers t ON t.trainer_id = rb.trainer_id
		LEFT JOIN racing.jockeys j ON j.jockey_id = rb.jockey_id
		WHERE rb.horse_id = $1
		ORDER BY rb.race_date DESC
		LIMIT 20
	`
	profile.RecentForm = []models.FormEntry{}
	if err := r.db.Select(&profile.RecentForm, formQuery, horseID); err != nil {
		return nil, fmt.Errorf("failed to get recent form: %w", err)
	}

	// Get going splits
	goingSplits, err := r.GetHorseGoingSplits(horseID)
	if err != nil {
		return nil, err
	}
	profile.GoingSplits = goingSplits

	// Get distance splits
	distSplits, err := r.GetHorseDistanceSplits(horseID)
	if err != nil {
		return nil, err
	}
	profile.DistanceSplits = distSplits

	// Get course splits
	courseSplits, err := r.GetHorseCourseSplits(horseID)
	if err != nil {
		return nil, err
	}
	profile.CourseSplits = courseSplits

	// Get RPR trend - using mv_runner_base for faster query
	trendQuery := `
		SELECT 
			rb.race_date AS date,
			rb.rpr,
			rb."or",
			rb.class,
			rb.win_flag
		FROM mv_runner_base rb
		WHERE rb.horse_id = $1
			AND (rb.rpr IS NOT NULL OR rb."or" IS NOT NULL)
		ORDER BY rb.race_date DESC
		LIMIT 20
	`
	profile.RPRTrend = []models.TrendPoint{}
	if err := r.db.Select(&profile.RPRTrend, trendQuery, horseID); err != nil {
		return nil, fmt.Errorf("failed to get RPR trend: %w", err)
	}

	return profile, nil
}

// GetHorseGoingSplits returns performance splits by going
func (r *ProfileRepository) GetHorseGoingSplits(horseID int64) ([]models.StatsSplit, error) {
	query := `
		SELECT 
			rb.going AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE rb.win_flag) AS wins,
			COUNT(*) FILTER (WHERE rb.pos_num <= 3) AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE rb.win_flag) / COUNT(*), 2) AS sr,
			AVG(rb.rpr) FILTER (WHERE rb.rpr IS NOT NULL) AS avg_rpr
		FROM mv_runner_base rb
		WHERE rb.horse_id = $1 AND rb.going IS NOT NULL
		GROUP BY rb.going
		HAVING COUNT(*) >= 2
		ORDER BY runs DESC
	`
	var splits []models.StatsSplit
	if err := r.db.Select(&splits, query, horseID); err != nil {
		return nil, fmt.Errorf("failed to get going splits: %w", err)
	}
	return splits, nil
}

// GetHorseDistanceSplits returns performance splits by distance
func (r *ProfileRepository) GetHorseDistanceSplits(horseID int64) ([]models.StatsSplit, error) {
	query := `
		SELECT 
			CASE 
				WHEN rb.dist_f < 7 THEN '5-6f'
				WHEN rb.dist_f < 9 THEN '7-8f'
				WHEN rb.dist_f < 13 THEN '9-12f'
				ELSE '13f+'
			END AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE rb.win_flag) AS wins,
			COUNT(*) FILTER (WHERE rb.pos_num <= 3) AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE rb.win_flag) / COUNT(*), 2) AS sr,
			AVG(rb.rpr) FILTER (WHERE rb.rpr IS NOT NULL) AS avg_rpr
		FROM mv_runner_base rb
		WHERE rb.horse_id = $1 AND rb.dist_f IS NOT NULL
		GROUP BY category
		ORDER BY MIN(rb.dist_f)
	`
	var splits []models.StatsSplit
	if err := r.db.Select(&splits, query, horseID); err != nil {
		return nil, fmt.Errorf("failed to get distance splits: %w", err)
	}
	return splits, nil
}

// GetHorseCourseSplits returns performance splits by course
func (r *ProfileRepository) GetHorseCourseSplits(horseID int64) ([]models.StatsSplit, error) {
	query := `
		SELECT 
			c.course_name AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE rb.win_flag) AS wins,
			COUNT(*) FILTER (WHERE rb.pos_num <= 3) AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE rb.win_flag) / COUNT(*), 2) AS sr,
			AVG(rb.rpr) FILTER (WHERE rb.rpr IS NOT NULL) AS avg_rpr
		FROM mv_runner_base rb
		JOIN racing.courses c ON c.course_id = rb.course_id
		WHERE rb.horse_id = $1
		GROUP BY c.course_id, c.course_name
		HAVING COUNT(*) >= 2
		ORDER BY sr DESC
	`
	var splits []models.StatsSplit
	if err := r.db.Select(&splits, query, horseID); err != nil {
		return nil, fmt.Errorf("failed to get course splits: %w", err)
	}
	return splits, nil
}

// GetTrainerProfile returns complete trainer profile
func (r *ProfileRepository) GetTrainerProfile(trainerID int64) (*models.TrainerProfile, error) {
	profile := &models.TrainerProfile{}

	// Get trainer details
	trainerQuery := `SELECT trainer_id, trainer_name FROM racing.trainers WHERE trainer_id = $1`
	if err := r.db.Get(&profile.Trainer, trainerQuery, trainerID); err != nil {
		return nil, fmt.Errorf("failed to get trainer: %w", err)
	}

	// Get rolling form (14, 30, 90 days)
	formQuery := `
		WITH recent_runs AS (
			SELECT 
				ru.runner_id,
				r.race_date,
				ru.win_flag,
				CASE WHEN ru.win_bsp > 0 THEN (ru.win_bsp - 1) * ru.win_lose ELSE 0 END AS pl
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE ru.trainer_id = $1
				AND r.race_date >= CURRENT_DATE - INTERVAL '90 days'
		)
		SELECT '14d'::text AS period,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days') AS runs,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days' AND win_flag) AS wins,
			ROUND(100.0 * COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days' AND win_flag) / 
				NULLIF(COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days'), 0), 2) AS sr,
			SUM(pl) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days') AS pl
		FROM recent_runs
		UNION ALL
		SELECT '30d'::text,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days'),
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days' AND win_flag),
			ROUND(100.0 * COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days' AND win_flag) / 
				NULLIF(COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days'), 0), 2),
			SUM(pl) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days')
		FROM recent_runs
		UNION ALL
		SELECT '90d'::text,
			COUNT(*),
			COUNT(*) FILTER (WHERE win_flag),
			ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / NULLIF(COUNT(*), 0), 2),
			SUM(pl)
		FROM recent_runs
	`
	profile.RollingForm = []models.FormPeriod{}
	if err := r.db.Select(&profile.RollingForm, formQuery, trainerID); err != nil {
		return nil, fmt.Errorf("failed to get rolling form: %w", err)
	}

	// Get course splits
	courseSplitsQuery := `
		SELECT 
			c.course_name AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
			0 AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) AS sr
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		JOIN racing.courses c ON c.course_id = r.course_id
		WHERE ru.trainer_id = $1
		GROUP BY c.course_id, c.course_name
		HAVING COUNT(*) >= 10
		ORDER BY sr DESC
		LIMIT 20
	`
	profile.CourseSplits = []models.StatsSplit{}
	if err := r.db.Select(&profile.CourseSplits, courseSplitsQuery, trainerID); err != nil {
		return nil, fmt.Errorf("failed to get course splits: %w", err)
	}

	// Get type splits
	typeSplitsQuery := `
		SELECT 
			r.race_type AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
			0 AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) AS sr
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE ru.trainer_id = $1
		GROUP BY r.race_type
		ORDER BY runs DESC
	`
	profile.TypeSplits = []models.StatsSplit{}
	if err := r.db.Select(&profile.TypeSplits, typeSplitsQuery, trainerID); err != nil {
		return nil, fmt.Errorf("failed to get type splits: %w", err)
	}

	// Get distance splits
	distSplitsQuery := `
		SELECT 
			CASE 
				WHEN r.dist_f < 7 THEN '5-6f'
				WHEN r.dist_f < 9 THEN '7-8f'
				WHEN r.dist_f < 13 THEN '9-12f'
				ELSE '13f+'
			END AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
			0 AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) AS sr
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE ru.trainer_id = $1 AND r.dist_f IS NOT NULL
		GROUP BY category
		ORDER BY MIN(r.dist_f)
	`
	profile.DistSplits = []models.StatsSplit{}
	if err := r.db.Select(&profile.DistSplits, distSplitsQuery, trainerID); err != nil {
		return nil, fmt.Errorf("failed to get distance splits: %w", err)
	}

	return profile, nil
}

// GetJockeyProfile returns complete jockey profile
func (r *ProfileRepository) GetJockeyProfile(jockeyID int64) (*models.JockeyProfile, error) {
	profile := &models.JockeyProfile{}

	// Get jockey details
	jockeyQuery := `SELECT jockey_id, jockey_name FROM racing.jockeys WHERE jockey_id = $1`
	if err := r.db.Get(&profile.Jockey, jockeyQuery, jockeyID); err != nil {
		return nil, fmt.Errorf("failed to get jockey: %w", err)
	}

	// Get career stats (reusing CareerSummary struct)
	statsQuery := `
		SELECT 
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE win_flag) AS wins,
			COUNT(*) FILTER (WHERE pos_num <= 3) AS places,
			SUM(prize) AS total_prize,
			AVG(rpr) FILTER (WHERE rpr IS NOT NULL) AS avg_rpr,
			MAX(rpr) AS peak_rpr,
			AVG("or") FILTER (WHERE "or" IS NOT NULL) AS avg_or,
			MAX("or") AS peak_or
		FROM runners
		WHERE jockey_id = $1
	`
	if err := r.db.Get(&profile.CareerStats, statsQuery, jockeyID); err != nil {
		return nil, fmt.Errorf("failed to get career stats: %w", err)
	}

	// Get rolling form
	formQuery := `
		WITH recent_runs AS (
			SELECT 
				ru.runner_id,
				r.race_date,
				ru.win_flag
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE ru.jockey_id = $1
				AND r.race_date >= CURRENT_DATE - INTERVAL '90 days'
		)
		SELECT '14d'::text AS period,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days') AS runs,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days' AND win_flag) AS wins,
			ROUND(100.0 * COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days' AND win_flag) / 
				NULLIF(COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '14 days'), 0), 2) AS sr,
			NULL::double precision AS pl
		FROM recent_runs
		UNION ALL
		SELECT '30d'::text,
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days'),
			COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days' AND win_flag),
			ROUND(100.0 * COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days' AND win_flag) / 
				NULLIF(COUNT(*) FILTER (WHERE recent_runs.race_date >= CURRENT_DATE - INTERVAL '30 days'), 0), 2),
			NULL::double precision
		FROM recent_runs
		UNION ALL
		SELECT '90d'::text,
			COUNT(*),
			COUNT(*) FILTER (WHERE win_flag),
			ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / NULLIF(COUNT(*), 0), 2),
			NULL::double precision
		FROM recent_runs
	`
	profile.RollingForm = []models.FormPeriod{}
	if err := r.db.Select(&profile.RollingForm, formQuery, jockeyID); err != nil {
		return nil, fmt.Errorf("failed to get rolling form: %w", err)
	}

	// Get trainer combos
	combosQuery := `
		SELECT 
			t.trainer_name,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) AS sr
		FROM racing.runners ru
		JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
		WHERE ru.jockey_id = $1
		GROUP BY t.trainer_id, t.trainer_name
		HAVING COUNT(*) >= 5
		ORDER BY sr DESC
		LIMIT 20
	`
	profile.TrainerCombos = []models.TrainerCombo{}
	if err := r.db.Select(&profile.TrainerCombos, combosQuery, jockeyID); err != nil {
		return nil, fmt.Errorf("failed to get trainer combos: %w", err)
	}

	// Get course splits
	courseSplitsQuery := `
		SELECT 
			c.course_name AS category,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
			0 AS places,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_flag) / COUNT(*), 2) AS sr
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		JOIN racing.courses c ON c.course_id = r.course_id
		WHERE ru.jockey_id = $1
		GROUP BY c.course_id, c.course_name
		HAVING COUNT(*) >= 10
		ORDER BY sr DESC
		LIMIT 20
	`
	profile.CourseSplits = []models.StatsSplit{}
	if err := r.db.Select(&profile.CourseSplits, courseSplitsQuery, jockeyID); err != nil {
		return nil, fmt.Errorf("failed to get course splits: %w", err)
	}

	return profile, nil
}
