package models

// DrawBiasParams represents parameters for draw bias analysis
type DrawBiasParams struct {
	CourseID   int64    `form:"course_id" binding:"required"`
	DistMin    *float64 `form:"dist_min"`
	DistMax    *float64 `form:"dist_max"`
	Going      *string  `form:"going"`
	MinRunners int      `form:"min_runners"`
}

// DrawBiasResult represents draw bias statistics
type DrawBiasResult struct {
	Draw        int      `json:"draw" db:"draw"`
	TotalRuns   int      `json:"total_runs" db:"total_runs"`
	WinRate     float64  `json:"win_rate" db:"win_rate"`
	Top3Rate    float64  `json:"top3_rate" db:"top3_rate"`
	AvgPosition *float64 `json:"avg_position,omitempty" db:"avg_position"`
}

// RecencyEffect represents days-since-run statistics
type RecencyEffect struct {
	DSRBucket string   `json:"dsr_bucket" db:"dsr_bucket"`
	Runs      int      `json:"runs" db:"runs"`
	Wins      int      `json:"wins" db:"wins"`
	SR        float64  `json:"sr" db:"sr"`
	AvgPos    *float64 `json:"avg_pos,omitempty" db:"avg_pos"`
}

// TrainerChange represents trainer change impact
type TrainerChange struct {
	HorseName    string   `json:"horse_name" db:"horse_name"`
	OldTrainer   *string  `json:"old_trainer,omitempty" db:"old_trainer"`
	NewTrainer   *string  `json:"new_trainer,omitempty" db:"new_trainer"`
	RunsBefore   int      `json:"runs_before" db:"runs_before"`
	RunsAfter    int      `json:"runs_after" db:"runs_after"`
	AvgRPRBefore *float64 `json:"avg_rpr_before,omitempty" db:"avg_rpr_before"`
	AvgRPRAfter  *float64 `json:"avg_rpr_after,omitempty" db:"avg_rpr_after"`
}
