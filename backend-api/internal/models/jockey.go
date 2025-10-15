package models

// Jockey represents a jockey entity
type Jockey struct {
	JockeyID   int64  `json:"jockey_id" db:"jockey_id"`
	JockeyName string `json:"jockey_name" db:"jockey_name"`
}

// JockeyProfile represents complete jockey profile data
type JockeyProfile struct {
	Jockey        Jockey         `json:"jockey"`
	CareerStats   CareerSummary  `json:"career_stats"`
	RollingForm   []FormPeriod   `json:"rolling_form"`
	TrainerCombos []TrainerCombo `json:"trainer_combos"`
	CourseSplits  []StatsSplit   `json:"course_splits"`
}

// TrainerCombo represents jockey-trainer combination statistics
type TrainerCombo struct {
	TrainerName string   `json:"trainer_name" db:"trainer_name"`
	Runs        int      `json:"runs" db:"runs"`
	Wins        int      `json:"wins" db:"wins"`
	SR          float64  `json:"sr" db:"sr"`
	ROI         *float64 `json:"roi,omitempty" db:"roi"`
}
