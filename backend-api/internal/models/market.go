package models

// MarketMover represents a market mover (steamer/drifter)
type MarketMover struct {
	RaceID       int64    `json:"race_id" db:"race_id"`
	RaceDate     string   `json:"race_date" db:"race_date"`
	OffTime      *string  `json:"off_time,omitempty" db:"off_time"`
	CourseName   string   `json:"course_name" db:"course_name"`
	RaceName     string   `json:"race_name" db:"race_name"`
	HorseName    string   `json:"horse_name" db:"horse_name"`
	MorningPrice *float64 `json:"morning_price,omitempty" db:"morning_price"`
	BSP          *float64 `json:"bsp,omitempty" db:"bsp"`
	MovePct      *float64 `json:"move_pct,omitempty" db:"move_pct"`
	WinFlag      bool     `json:"win_flag" db:"win_flag"`
	PosNum       *int     `json:"pos_num,omitempty" db:"pos_num"`
}

// CalibrationBin represents market calibration data for a price bin
type CalibrationBin struct {
	PriceBin  string  `json:"price_bin" db:"price_bin"`
	Runners   int     `json:"runners" db:"runners"`
	Wins      int     `json:"wins" db:"wins"`
	ActualSR  float64 `json:"actual_sr" db:"actual_sr"`
	ImpliedSR float64 `json:"implied_sr" db:"implied_sr"`
	Edge      float64 `json:"edge" db:"edge"`
}

// InPlayMove represents in-play price movement
type InPlayMove struct {
	RaceID      int64    `json:"race_id" db:"race_id"`
	RaceDate    string   `json:"race_date" db:"race_date"`
	CourseName  string   `json:"course_name" db:"course_name"`
	RaceName    string   `json:"race_name" db:"race_name"`
	HorseName   string   `json:"horse_name" db:"horse_name"`
	WinBSP      *float64 `json:"win_bsp,omitempty" db:"win_bsp"`
	IPLow       *float64 `json:"ip_low,omitempty" db:"ip_low"`
	IPHigh      *float64 `json:"ip_high,omitempty" db:"ip_high"`
	SurgePct    *float64 `json:"surge_pct,omitempty" db:"surge_pct"`
	CollapsePct *float64 `json:"collapse_pct,omitempty" db:"collapse_pct"`
	WinFlag     bool     `json:"win_flag" db:"win_flag"`
	PosNum      *int     `json:"pos_num,omitempty" db:"pos_num"`
}

// BookVsExchange represents comparison of bookmaker vs exchange prices
type BookVsExchange struct {
	RaceDate     string   `json:"race_date" db:"race_date"`
	Races        int      `json:"races" db:"races"`
	AvgSPWinner  *float64 `json:"avg_sp_winner,omitempty" db:"avg_sp_winner"`
	AvgBSPWinner *float64 `json:"avg_bsp_winner,omitempty" db:"avg_bsp_winner"`
	SPPL         *float64 `json:"sp_pl,omitempty" db:"sp_pl"`
	BSPPL        *float64 `json:"bsp_pl,omitempty" db:"bsp_pl"`
}
