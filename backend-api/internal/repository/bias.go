package repository

import (
	"fmt"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type BiasRepository struct {
	db *database.DB
}

func NewBiasRepository(db *database.DB) *BiasRepository {
	return &BiasRepository{db: db}
}

// GetDrawBias returns draw bias statistics
func (r *BiasRepository) GetDrawBias(params models.DrawBiasParams) ([]models.DrawBiasResult, error) {
	if params.MinRunners <= 0 {
		params.MinRunners = 10
	}

	query := `
		WITH draw_stats AS (
			SELECT 
				ru.draw,
				r.ran AS field_size,
				r.going,
				COUNT(*) AS runs,
				COUNT(*) FILTER (WHERE ru.pos_num <= 3) AS top3,
				COUNT(*) FILTER (WHERE ru.win_flag) AS wins,
				AVG(ru.pos_num) FILTER (WHERE ru.pos_num IS NOT NULL) AS avg_pos
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE r.course_id = $1
				AND r.race_type = 'Flat'
				AND r.ran >= $2
				AND ru.draw IS NOT NULL
	`

	args := []interface{}{params.CourseID, params.MinRunners}
	argCount := 2

	if params.DistMin != nil {
		argCount++
		query += fmt.Sprintf(" AND r.dist_f >= $%d", argCount)
		args = append(args, *params.DistMin)
	}

	if params.DistMax != nil {
		argCount++
		query += fmt.Sprintf(" AND r.dist_f <= $%d", argCount)
		args = append(args, *params.DistMax)
	}

	if params.Going != nil {
		argCount++
		query += fmt.Sprintf(" AND r.going ILIKE $%d", argCount)
		args = append(args, "%"+*params.Going+"%")
	}

	query += `
			GROUP BY ru.draw, r.ran, r.going
		)
		SELECT 
			draw,
			SUM(runs) AS total_runs,
			ROUND(100.0 * SUM(wins) / NULLIF(SUM(runs), 0), 2) AS win_rate,
			ROUND(100.0 * SUM(top3) / NULLIF(SUM(runs), 0), 2) AS top3_rate,
			ROUND(AVG(avg_pos), 2) AS avg_position
		FROM draw_stats
		GROUP BY draw
		ORDER BY draw
	`

	var results []models.DrawBiasResult
	if err := r.db.Select(&results, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get draw bias: %w", err)
	}

	return results, nil
}

// GetRecencyEffects returns days-since-run statistics
func (r *BiasRepository) GetRecencyEffects(dateFrom, dateTo string) ([]models.RecencyEffect, error) {
	query := `
		WITH runs_with_dsr AS (
			SELECT 
				ru.runner_id,
				r.race_date,
				ru.horse_id,
				ru.win_flag,
				ru.pos_num,
				LAG(r.race_date) OVER (PARTITION BY ru.horse_id ORDER BY r.race_date) AS prev_run_date,
				r.race_date - LAG(r.race_date) OVER (PARTITION BY ru.horse_id ORDER BY r.race_date) AS dsr
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE r.race_date BETWEEN $1 AND $2
		)
		SELECT 
			CASE 
				WHEN dsr < 14 THEN '0-13 days'
				WHEN dsr < 28 THEN '14-27 days'
				WHEN dsr < 56 THEN '28-55 days'
				WHEN dsr < 90 THEN '56-89 days'
				WHEN dsr < 180 THEN '90-179 days'
				ELSE '180+ days'
			END AS dsr_bucket,
			COUNT(*) AS runs,
			COUNT(*) FILTER (WHERE win_flag) AS wins,
			ROUND(100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*), 2) AS sr,
			ROUND(AVG(pos_num) FILTER (WHERE pos_num IS NOT NULL), 2) AS avg_pos
		FROM runs_with_dsr
		WHERE dsr IS NOT NULL
		GROUP BY dsr_bucket
		ORDER BY MIN(dsr)
	`

	var effects []models.RecencyEffect
	if err := r.db.Select(&effects, query, dateFrom, dateTo); err != nil {
		return nil, fmt.Errorf("failed to get recency effects: %w", err)
	}

	return effects, nil
}

// GetTrainerChanges returns trainer change impact analysis
// Note: This is a complex query that can take 20-30 seconds on large datasets
func (r *BiasRepository) GetTrainerChanges(minRuns int) ([]models.TrainerChange, error) {
	if minRuns <= 0 {
		minRuns = 5
	}

	// Simplified query that avoids expensive window functions
	// Uses a JOIN-based approach instead of LAG
	query := `
		WITH recent_runs AS (
			SELECT 
				ru.horse_id,
				ru.trainer_id,
				r.race_date,
				ru.rpr,
				ru.pos_num
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE ru.trainer_id IS NOT NULL
				AND r.race_date >= CURRENT_DATE - INTERVAL '6 months'
				AND ru.rpr IS NOT NULL
		),
		trainer_pairs AS (
			SELECT DISTINCT
				r1.horse_id,
				r1.trainer_id AS old_trainer_id,
				r2.trainer_id AS new_trainer_id
			FROM recent_runs r1
			JOIN recent_runs r2 ON r2.horse_id = r1.horse_id 
				AND r2.race_date > r1.race_date
				AND r2.trainer_id != r1.trainer_id
			WHERE NOT EXISTS (
				SELECT 1 FROM recent_runs r3
				WHERE r3.horse_id = r1.horse_id
					AND r3.race_date > r1.race_date
					AND r3.race_date < r2.race_date
					AND r3.trainer_id NOT IN (r1.trainer_id, r2.trainer_id)
			)
			LIMIT 100
		)
	SELECT 
		h.horse_name,
		t_old.trainer_name AS old_trainer,
		t_new.trainer_name AS new_trainer,
		COUNT(DISTINCT r_old.race_date) AS runs_before,
		COUNT(DISTINCT r_new.race_date) AS runs_after,
		ROUND(AVG(r_old.rpr)::numeric, 1) AS avg_rpr_before,
		ROUND(AVG(r_new.rpr)::numeric, 1) AS avg_rpr_after
	FROM trainer_pairs tp
	JOIN racing.horses h ON h.horse_id = tp.horse_id
	JOIN racing.trainers t_old ON t_old.trainer_id = tp.old_trainer_id
	JOIN racing.trainers t_new ON t_new.trainer_id = tp.new_trainer_id
	LEFT JOIN recent_runs r_old ON r_old.horse_id = tp.horse_id AND r_old.trainer_id = tp.old_trainer_id
	LEFT JOIN recent_runs r_new ON r_new.horse_id = tp.horse_id AND r_new.trainer_id = tp.new_trainer_id
	GROUP BY h.horse_name, t_old.trainer_name, t_new.trainer_name
	HAVING COUNT(DISTINCT r_old.race_date) + COUNT(DISTINCT r_new.race_date) >= $1
	ORDER BY (AVG(r_new.rpr) - AVG(r_old.rpr)) DESC NULLS LAST
	LIMIT 100
	`

	var changes []models.TrainerChange
	if err := r.db.Select(&changes, query, minRuns); err != nil {
		return nil, fmt.Errorf("failed to get trainer changes: %w", err)
	}

	return changes, nil
}
