package models

// Course represents a racing course/venue
type Course struct {
	CourseID   int64  `json:"course_id" db:"course_id"`
	CourseName string `json:"course_name" db:"course_name"`
	Region     string `json:"region" db:"region"`
}

// Meeting represents a race meeting at a course
type Meeting struct {
	RaceDate   string `json:"race_date" db:"race_date"`
	MeetingKey string `json:"meeting_key" db:"meeting_key"`
	RaceCount  int    `json:"race_count" db:"race_count"`
	FirstRace  string `json:"first_race" db:"first_race"`
	LastRace   string `json:"last_race" db:"last_race"`
	TotalRuns  int    `json:"total_runners" db:"total_runners"`
}
