package models

// Runner represents a runner entity with complete details
type Runner struct {
	RunnerID  int64  `json:"runner_id" db:"runner_id"`
	RunnerKey string `json:"runner_key" db:"runner_key"`
	RaceID    int64  `json:"race_id" db:"race_id"`
	RaceDate  string `json:"race_date" db:"race_date"`

	// Horse/Trainer/Jockey details
	HorseID     *int64  `json:"horse_id,omitempty" db:"horse_id"`
	HorseName   *string `json:"horse_name,omitempty" db:"horse_name"`
	TrainerID   *int64  `json:"trainer_id,omitempty" db:"trainer_id"`
	TrainerName *string `json:"trainer_name,omitempty" db:"trainer_name"`
	JockeyID    *int64  `json:"jockey_id,omitempty" db:"jockey_id"`
	JockeyName  *string `json:"jockey_name,omitempty" db:"jockey_name"`
	OwnerID     *int64  `json:"owner_id,omitempty" db:"owner_id"`
	OwnerName   *string `json:"owner_name,omitempty" db:"owner_name"`

	// Race details
	Num     *int     `json:"num,omitempty" db:"num"`
	PosRaw  *string  `json:"pos_raw,omitempty" db:"pos_raw"`
	PosNum  *int     `json:"pos_num,omitempty" db:"pos_num"`
	Draw    *int     `json:"draw,omitempty" db:"draw"`
	OvrBTN  *float64 `json:"ovr_btn,omitempty" db:"ovr_btn"`
	BTN     *float64 `json:"btn,omitempty" db:"btn"`
	Age     *int     `json:"age,omitempty" db:"age"`
	Sex     *string  `json:"sex,omitempty" db:"sex"`
	Lbs     *int     `json:"lbs,omitempty" db:"lbs"`
	HG      *string  `json:"hg,omitempty" db:"hg"`
	TimeRaw *string  `json:"time_raw,omitempty" db:"time_raw"`
	Secs    *float64 `json:"secs,omitempty" db:"secs"`
	Dec     *float64 `json:"dec,omitempty" db:"dec"`
	Prize   *float64 `json:"prize,omitempty" db:"prize"`
	OR      *int     `json:"or,omitempty" db:"or"`
	RPR     *int     `json:"rpr,omitempty" db:"rpr"`
	Comment *string  `json:"comment,omitempty" db:"comment"`

	// Betfair WIN market
	WinBSP        *float64 `json:"win_bsp,omitempty" db:"win_bsp"`
	WinPPWAP      *float64 `json:"win_ppwap,omitempty" db:"win_ppwap"`
	WinMorningWAP *float64 `json:"win_morningwap,omitempty" db:"win_morningwap"`
	WinPPMax      *float64 `json:"win_ppmax,omitempty" db:"win_ppmax"`
	WinPPMin      *float64 `json:"win_ppmin,omitempty" db:"win_ppmin"`
	WinIPMax      *float64 `json:"win_ipmax,omitempty" db:"win_ipmax"`
	WinIPMin      *float64 `json:"win_ipmin,omitempty" db:"win_ipmin"`
	WinMorningVol *float64 `json:"win_morning_vol,omitempty" db:"win_morning_vol"`
	WinPreVol     *float64 `json:"win_pre_vol,omitempty" db:"win_pre_vol"`
	WinIPVol      *float64 `json:"win_ip_vol,omitempty" db:"win_ip_vol"`
	WinLose       *int     `json:"win_lose,omitempty" db:"win_lose"`

	// Betfair PLACE market
	PlaceBSP        *float64 `json:"place_bsp,omitempty" db:"place_bsp"`
	PlacePPWAP      *float64 `json:"place_ppwap,omitempty" db:"place_ppwap"`
	PlaceMorningWAP *float64 `json:"place_morningwap,omitempty" db:"place_morningwap"`
	PlacePPMax      *float64 `json:"place_ppmax,omitempty" db:"place_ppmax"`
	PlacePPMin      *float64 `json:"place_ppmin,omitempty" db:"place_ppmin"`
	PlaceIPMax      *float64 `json:"place_ipmax,omitempty" db:"place_ipmax"`
	PlaceIPMin      *float64 `json:"place_ipmin,omitempty" db:"place_ipmin"`
	PlaceMorningVol *float64 `json:"place_morning_vol,omitempty" db:"place_morning_vol"`
	PlacePreVol     *float64 `json:"place_pre_vol,omitempty" db:"place_pre_vol"`
	PlaceIPVol      *float64 `json:"place_ip_vol,omitempty" db:"place_ip_vol"`
	PlaceWinLose    *int     `json:"place_win_lose,omitempty" db:"place_win_lose"`

	// Bloodlines
	Sire    *string `json:"sire,omitempty" db:"sire"`
	Dam     *string `json:"dam,omitempty" db:"dam"`
	Damsire *string `json:"damsire,omitempty" db:"damsire"`

	// Generated fields
	WinFlag bool `json:"win_flag" db:"win_flag"`
}
