package models

// Trainer represents a trainer entity
type Trainer struct {
	TrainerID   int64  `json:"trainer_id" db:"trainer_id"`
	TrainerName string `json:"trainer_name" db:"trainer_name"`
}

// TrainerProfile represents complete trainer profile data
type TrainerProfile struct {
	Trainer      Trainer      `json:"trainer"`
	RollingForm  []FormPeriod `json:"rolling_form"`
	CourseSplits []StatsSplit `json:"course_splits"`
	TypeSplits   []StatsSplit `json:"type_splits"`
	DistSplits   []StatsSplit `json:"dist_splits"`
}

// FormPeriod represents statistics for a time period
type FormPeriod struct {
	Period string   `json:"period" db:"period"`
	Runs   int      `json:"runs" db:"runs"`
	Wins   int      `json:"wins" db:"wins"`
	SR     *float64 `json:"sr,omitempty" db:"sr"`
	PL     *float64 `json:"pl,omitempty" db:"pl"`
	ROI    *float64 `json:"roi,omitempty" db:"roi"`
}
