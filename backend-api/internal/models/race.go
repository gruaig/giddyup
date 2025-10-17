package models

// Race represents a race entity
type Race struct {
	RaceID     int64    `json:"race_id" db:"race_id"`
	RaceKey    string   `json:"race_key" db:"race_key"`
	RaceDate   string   `json:"race_date" db:"race_date"`
	Region     string   `json:"region" db:"region"`
	CourseID   *int64   `json:"course_id,omitempty" db:"course_id"`
	CourseName *string  `json:"course_name,omitempty" db:"course_name"`
	OffTime    *string  `json:"off_time,omitempty" db:"off_time"`
	RaceName   string   `json:"race_name" db:"race_name"`
	RaceType   string   `json:"race_type" db:"race_type"`
	Class      *string  `json:"class,omitempty" db:"class"`
	Pattern    *string  `json:"pattern,omitempty" db:"pattern"`
	RatingBand *string  `json:"rating_band,omitempty" db:"rating_band"`
	AgeBand    *string  `json:"age_band,omitempty" db:"age_band"`
	SexRest    *string  `json:"sex_rest,omitempty" db:"sex_rest"`
	DistRaw    *string  `json:"dist_raw,omitempty" db:"dist_raw"`
	DistF      *float64 `json:"dist_f,omitempty" db:"dist_f"`
	DistM      *int     `json:"dist_m,omitempty" db:"dist_m"`
	Going      *string  `json:"going,omitempty" db:"going"`
	Surface    *string  `json:"surface,omitempty" db:"surface"`
	Ran        int      `json:"ran" db:"ran"`
	Runners    []Runner `json:"runners,omitempty"` // Populated by GetRacesByMeetings
}

// RaceWithRunners represents a race with its runners
type RaceWithRunners struct {
	Race    Race     `json:"race"`
	Runners []Runner `json:"runners"`
}

// MeetingWithRaces represents a meeting (course + date) with its races
type MeetingWithRaces struct {
	RaceDate       string  `json:"race_date"`
	Region         string  `json:"region"`
	CourseID       *int64  `json:"course_id,omitempty"`
	CourseName     *string `json:"course_name,omitempty"`
	RaceCount      int     `json:"race_count"`
	FirstRaceTime  *string `json:"first_race_time,omitempty"`
	LastRaceTime   *string `json:"last_race_time,omitempty"`
	RaceTypes      *string `json:"race_types,omitempty"` // e.g., "Flat, Chase"
	Races          []Race  `json:"races"`
}

// RaceFilters represents search filters for races
type RaceFilters struct {
	DateFrom *string  `form:"date_from"`
	DateTo   *string  `form:"date_to"`
	Region   *string  `form:"region"`
	CourseID *int64   `form:"course_id"`
	Type     *string  `form:"type"`
	Class    *string  `form:"class"`
	Pattern  *string  `form:"pattern"`
	Handicap *bool    `form:"handicap"`
	DistMin  *float64 `form:"dist_min"`
	DistMax  *float64 `form:"dist_max"`
	Going    *string  `form:"going"`
	Surface  *string  `form:"surface"`
	FieldMin *int     `form:"field_min"`
	FieldMax *int     `form:"field_max"`
	Limit    int      `form:"limit"`
	Offset   int      `form:"offset"`
}
