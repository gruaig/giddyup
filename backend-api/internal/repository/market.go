package repository

import (
	"fmt"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type MarketRepository struct {
	db *database.DB
}

func NewMarketRepository(db *database.DB) *MarketRepository {
	return &MarketRepository{db: db}
}

// GetMarketMovers returns steamers and drifters
func (r *MarketRepository) GetMarketMovers(date string, minMove float64, moveType string) ([]models.MarketMover, error) {
	query := `
		SELECT 
			r.race_id,
			r.race_date,
			r.off_time,
			c.course_name,
			r.race_name,
			h.horse_name,
		ru.win_ppmax AS morning_price,
		ru.win_bsp AS bsp,
		ROUND((100.0 * (ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0))::numeric, 2) AS move_pct,
		ru.win_flag,
		ru.pos_num
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		JOIN racing.courses c ON c.course_id = r.course_id
		JOIN racing.horses h ON h.horse_id = ru.horse_id
		WHERE r.race_date = $1
			AND ru.win_ppmax > 0 
			AND ru.win_bsp > 0
			AND ru.win_ppmax != ru.win_bsp
			AND ABS((ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0)) >= $2 / 100.0
	`

	args := []interface{}{date, minMove}

	if moveType == "steamer" {
		query += " AND ru.win_bsp < ru.win_ppmax"
	} else if moveType == "drifter" {
		query += " AND ru.win_bsp > ru.win_ppmax"
	}

	query += " ORDER BY ABS((ru.win_ppmax - ru.win_bsp) / NULLIF(ru.win_ppmax, 0)) DESC LIMIT 100"

	var movers []models.MarketMover
	if err := r.db.Select(&movers, query, args...); err != nil {
		return nil, fmt.Errorf("failed to get market movers: %w", err)
	}

	return movers, nil
}

// GetWinCalibration returns BSP calibration by price bins
func (r *MarketRepository) GetWinCalibration(dateFrom, dateTo string) ([]models.CalibrationBin, error) {
	query := `
		WITH bsp_bins AS (
			SELECT 
				CASE 
					WHEN win_bsp < 2 THEN '1.0-2.0'
					WHEN win_bsp < 3 THEN '2.0-3.0'
					WHEN win_bsp < 5 THEN '3.0-5.0'
					WHEN win_bsp < 10 THEN '5.0-10.0'
					WHEN win_bsp < 20 THEN '10.0-20.0'
					ELSE '20.0+'
				END AS price_bin,
				win_bsp,
				win_flag
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE win_bsp > 0
				AND r.race_date BETWEEN $1 AND $2
		)
		SELECT 
			price_bin,
		COUNT(*) AS runners,
		COUNT(*) FILTER (WHERE win_flag) AS wins,
		ROUND((100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*))::numeric, 2) AS actual_sr,
		ROUND((100.0 / AVG(win_bsp))::numeric, 2) AS implied_sr,
		ROUND((100.0 * COUNT(*) FILTER (WHERE win_flag) / COUNT(*) - 100.0 / AVG(win_bsp))::numeric, 2) AS edge
		FROM bsp_bins
		GROUP BY price_bin
		ORDER BY MIN(win_bsp)
	`

	var bins []models.CalibrationBin
	if err := r.db.Select(&bins, query, dateFrom, dateTo); err != nil {
		return nil, fmt.Errorf("failed to get win calibration: %w", err)
	}

	return bins, nil
}

// GetPlaceCalibration returns place market calibration
func (r *MarketRepository) GetPlaceCalibration(dateFrom, dateTo string) ([]models.CalibrationBin, error) {
	query := `
		WITH place_bins AS (
			SELECT 
				CASE 
					WHEN place_bsp < 1.5 THEN '1.0-1.5'
					WHEN place_bsp < 2.0 THEN '1.5-2.0'
					WHEN place_bsp < 3.0 THEN '2.0-3.0'
					WHEN place_bsp < 5.0 THEN '3.0-5.0'
					ELSE '5.0+'
				END AS price_bin,
				place_bsp,
				CASE WHEN pos_num <= 3 THEN true ELSE false END AS placed
			FROM racing.runners ru
			JOIN racing.races r ON r.race_id = ru.race_id
			WHERE place_bsp > 0
				AND r.race_date BETWEEN $1 AND $2
		)
		SELECT 
			price_bin,
		COUNT(*) AS runners,
		SUM(CASE WHEN placed THEN 1 ELSE 0 END) AS wins,
		ROUND((100.0 * SUM(CASE WHEN placed THEN 1 ELSE 0 END) / COUNT(*))::numeric, 2) AS actual_sr,
		ROUND((100.0 / AVG(place_bsp))::numeric, 2) AS implied_sr,
		ROUND((100.0 * SUM(CASE WHEN placed THEN 1 ELSE 0 END) / COUNT(*) - 100.0 / AVG(place_bsp))::numeric, 2) AS edge
		FROM place_bins
		GROUP BY price_bin
		ORDER BY MIN(place_bsp)
	`

	var bins []models.CalibrationBin
	if err := r.db.Select(&bins, query, dateFrom, dateTo); err != nil {
		return nil, fmt.Errorf("failed to get place calibration: %w", err)
	}

	return bins, nil
}

// GetInPlayMoves returns in-play price movements
func (r *MarketRepository) GetInPlayMoves(dateFrom, dateTo string, minMovePct float64) ([]models.InPlayMove, error) {
	query := `
		SELECT 
			r.race_id,
			r.race_date,
			c.course_name,
			r.race_name,
			h.horse_name,
			ru.win_bsp,
		ru.win_ipmin AS ip_low,
		ru.win_ipmax AS ip_high,
		ROUND((100.0 * (ru.win_ipmin - ru.win_bsp) / NULLIF(ru.win_bsp, 0))::numeric, 2) AS surge_pct,
		ROUND((100.0 * (ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0))::numeric, 2) AS collapse_pct,
		ru.win_flag,
		ru.pos_num
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		JOIN racing.courses c ON c.course_id = r.course_id
		JOIN racing.horses h ON h.horse_id = ru.horse_id
		WHERE r.race_date BETWEEN $1 AND $2
			AND ru.win_bsp > 0
			AND (ru.win_ipmin > 0 OR ru.win_ipmax > 0)
			AND (ABS((ru.win_ipmin - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) > $3 / 100.0
				OR ABS((ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) > $3 / 100.0)
		ORDER BY ABS((ru.win_ipmax - ru.win_bsp) / NULLIF(ru.win_bsp, 0)) DESC
		LIMIT 100
	`

	var moves []models.InPlayMove
	if err := r.db.Select(&moves, query, dateFrom, dateTo, minMovePct); err != nil {
		return nil, fmt.Errorf("failed to get in-play moves: %w", err)
	}

	return moves, nil
}

// GetBookVsExchange compares SP vs BSP
func (r *MarketRepository) GetBookVsExchange(dateFrom, dateTo string) ([]models.BookVsExchange, error) {
	query := `
		SELECT 
			r.race_date,
			COUNT(DISTINCT r.race_id) AS races,
			AVG(ru.dec) FILTER (WHERE ru.dec IS NOT NULL AND ru.win_flag) AS avg_sp_winner,
			AVG(ru.win_bsp) FILTER (WHERE ru.win_bsp IS NOT NULL AND ru.win_flag) AS avg_bsp_winner,
			SUM((ru.dec - 1) * CASE WHEN ru.win_flag THEN 1 ELSE -1 END) 
				FILTER (WHERE ru.dec IS NOT NULL) AS sp_pl,
			SUM((ru.win_bsp - 1) * ru.win_lose) 
				FILTER (WHERE ru.win_bsp > 0) AS bsp_pl
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE r.race_date BETWEEN $1 AND $2
			AND ru.dec IS NOT NULL
		GROUP BY r.race_date
		ORDER BY r.race_date
	`

	var results []models.BookVsExchange
	if err := r.db.Select(&results, query, dateFrom, dateTo); err != nil {
		return nil, fmt.Errorf("failed to get book vs exchange: %w", err)
	}

	return results, nil
}
