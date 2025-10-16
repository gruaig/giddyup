package models

// Horse represents a horse entity
type Horse struct {
	HorseID   int64  `json:"horse_id" db:"horse_id"`
	HorseName string `json:"horse_name" db:"horse_name"`
}

// HorseProfile represents complete horse profile data
type HorseProfile struct {
	Horse          Horse         `json:"horse"`
	CareerSummary  CareerSummary `json:"career_summary"`
	RecentForm     []FormEntry   `json:"recent_form"`
	GoingSplits    []StatsSplit  `json:"going_splits"`
	DistanceSplits []StatsSplit  `json:"distance_splits"`
	CourseSplits   []StatsSplit  `json:"course_splits"`
	RPRTrend       []TrendPoint  `json:"rpr_trend"`
}

// CareerSummary represents career statistics for a horse
type CareerSummary struct {
	Runs       int      `json:"runs" db:"runs"`
	Wins       int      `json:"wins" db:"wins"`
	Places     int      `json:"places" db:"places"`
	TotalPrize *float64 `json:"total_prize,omitempty" db:"total_prize"`
	AvgRPR     *float64 `json:"avg_rpr,omitempty" db:"avg_rpr"`
	PeakRPR    *int     `json:"peak_rpr,omitempty" db:"peak_rpr"`
	AvgOR      *float64 `json:"avg_or,omitempty" db:"avg_or"`
	PeakOR     *int     `json:"peak_or,omitempty" db:"peak_or"`
}

// FormEntry represents a single race result in form history
type FormEntry struct {
	RaceDate    string   `json:"race_date" db:"race_date"`
	CourseName  *string  `json:"course_name" db:"course_name"`
	RaceName    string   `json:"race_name" db:"race_name"`
	RaceType    string   `json:"race_type" db:"race_type"`
	Going       *string  `json:"going,omitempty" db:"going"`
	DistF       *float64 `json:"dist_f,omitempty" db:"dist_f"`
	PosNum      *int     `json:"pos_num,omitempty" db:"pos_num"`
	PosRaw      *string  `json:"pos_raw,omitempty" db:"pos_raw"`
	BTN         *float64 `json:"btn,omitempty" db:"btn"`
	OR          *int     `json:"or,omitempty" db:"or"`
	RPR         *int     `json:"rpr,omitempty" db:"rpr"`
	WinBSP      *float64 `json:"win_bsp,omitempty" db:"win_bsp"`
	Dec         *float64 `json:"dec,omitempty" db:"dec"`
	TrainerName *string  `json:"trainer_name,omitempty" db:"trainer_name"`
	JockeyName  *string  `json:"jockey_name,omitempty" db:"jockey_name"`
	DSR         *int     `json:"days_since_run,omitempty" db:"dsr"`
}
