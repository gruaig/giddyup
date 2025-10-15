package models

// NearMissQualifier represents a horse qualifying for the near-miss-no-hike angle
type NearMissQualifier struct {
	HorseID      int64          `json:"horse_id" db:"horse_id"`
	HorseName    string         `json:"horse_name" db:"horse_name"`
	Entry        EntryDetails   `json:"entry"`
	Last         LastRunDetails `json:"last"`
	DSR          int            `json:"dsr" db:"dsr"`
	RatingChange int            `json:"rating_change" db:"rating_change"`
	DistFDiff    float64        `json:"dist_f_diff" db:"dist_f_diff"`
	SameSurface  bool           `json:"same_surface" db:"same_surface"`
}

// EntryDetails represents the upcoming race entry
type EntryDetails struct {
	RaceID   int64    `json:"race_id" db:"next_race_id"`
	Date     string   `json:"date" db:"next_date"`
	RaceType string   `json:"race_type" db:"next_race_type"`
	CourseID int64    `json:"course_id" db:"next_course_id"`
	DistF    *float64 `json:"dist_f,omitempty" db:"next_dist_f"`
	Surface  *string  `json:"surface,omitempty" db:"next_surface"`
	Going    *string  `json:"going,omitempty" db:"next_going"`
	OR       *int     `json:"or,omitempty" db:"next_or"`
}

// LastRunDetails represents the last completed run
type LastRunDetails struct {
	RaceID  int64    `json:"race_id" db:"last_race_id"`
	Date    string   `json:"date" db:"last_date"`
	Pos     int      `json:"pos" db:"last_pos"`
	BTN     *float64 `json:"btn,omitempty" db:"last_btn"`
	OR      *int     `json:"or,omitempty" db:"last_or"`
	Class   *string  `json:"class,omitempty" db:"last_class"`
	DistF   *float64 `json:"dist_f,omitempty" db:"last_dist_f"`
	Surface *string  `json:"surface,omitempty" db:"last_surface"`
	Going   *string  `json:"going,omitempty" db:"last_going"`
}

// NearMissTodayParams represents query parameters for today's qualifiers
type NearMissTodayParams struct {
	On             *string `form:"on"`
	RaceType       *string `form:"race_type"`
	LastPos        int     `form:"last_pos"`
	BTNMax         float64 `form:"btn_max"`
	DSRMax         int     `form:"dsr_max"`
	ORDeltaMax     int     `form:"or_delta_max"`
	DistFTolerance float64 `form:"dist_f_tolerance"`
	SameSurface    bool    `form:"same_surface"`
	IncludeNullOR  bool    `form:"include_null_or"`
	Limit          int     `form:"limit"`
	Offset         int     `form:"offset"`
}

// NearMissPastCase represents a historical last→next pair
type NearMissPastCase struct {
	HorseID      int64    `json:"horse_id" db:"horse_id"`
	HorseName    string   `json:"horse_name" db:"horse_name"`
	LastRaceID   int64    `json:"last_race_id" db:"last_race_id"`
	LastDate     string   `json:"last_date" db:"last_date"`
	LastPos      int      `json:"last_pos" db:"last_pos"`
	LastBTN      *float64 `json:"last_btn,omitempty" db:"last_btn"`
	LastOR       *int     `json:"last_or,omitempty" db:"last_or"`
	LastClass    *string  `json:"last_class,omitempty" db:"last_class"`
	LastDistF    *float64 `json:"last_dist_f,omitempty" db:"last_dist_f"`
	LastSurface  *string  `json:"last_surface,omitempty" db:"last_surface"`
	NextRaceID   int64    `json:"next_race_id" db:"next_race_id"`
	NextDate     string   `json:"next_date" db:"next_date"`
	NextPos      *int     `json:"next_pos,omitempty" db:"next_pos"`
	NextWin      bool     `json:"next_win" db:"next_win"`
	NextOR       *int     `json:"next_or,omitempty" db:"next_or"`
	NextDistF    *float64 `json:"next_dist_f,omitempty" db:"next_dist_f"`
	NextSurface  *string  `json:"next_surface,omitempty" db:"next_surface"`
	DSR          int      `json:"dsr" db:"dsr"`
	RatingChange int      `json:"rating_change" db:"rating_change"`
	DistFDiff    float64  `json:"dist_f_diff" db:"dist_f_diff"`
	SameSurface  bool     `json:"same_surface" db:"same_surface"`
	Price        *float64 `json:"price,omitempty" db:"price"`
}

// NearMissPastParams represents query parameters for historical backtest
type NearMissPastParams struct {
	DateFrom       *string `form:"date_from"`
	DateTo         *string `form:"date_to"`
	RaceType       *string `form:"race_type"`
	LastPos        int     `form:"last_pos"`
	BTNMax         float64 `form:"btn_max"`
	DSRMax         int     `form:"dsr_max"`
	ORDeltaMax     int     `form:"or_delta_max"`
	DistFTolerance float64 `form:"dist_f_tolerance"`
	SameSurface    bool    `form:"same_surface"`
	IncludeNullOR  bool    `form:"include_null_or"`
	RequireNextWin bool    `form:"require_next_win"`
	PriceSource    string  `form:"price_source"`
	Summary        bool    `form:"summary"`
	Limit          int     `form:"limit"`
	Offset         int     `form:"offset"`
}

// NearMissPastResponse represents the backtest response with optional summary
type NearMissPastResponse struct {
	Summary *AngleSummary      `json:"summary,omitempty"`
	Cases   []NearMissPastCase `json:"cases"`
}

// AngleSummary represents aggregate performance metrics
type AngleSummary struct {
	N       int      `json:"n" db:"n"`
	Wins    int      `json:"wins" db:"wins"`
	WinRate float64  `json:"win_rate" db:"win_rate"`
	ROI     *float64 `json:"roi,omitempty" db:"roi"`
}
