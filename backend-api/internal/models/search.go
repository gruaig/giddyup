package models

// SearchResults represents global search results
type SearchResults struct {
	Horses   []SearchEntity `json:"horses"`
	Trainers []SearchEntity `json:"trainers"`
	Jockeys  []SearchEntity `json:"jockeys"`
	Owners   []SearchEntity `json:"owners"`
	Courses  []SearchEntity `json:"courses"`
	Total    int            `json:"total_results"`
}

// SearchEntity represents a search result entity
type SearchEntity struct {
	ID    int64   `json:"id" db:"id"`
	Name  string  `json:"name" db:"name"`
	Score float64 `json:"score" db:"score"`
	Type  string  `json:"type,omitempty"`
}

// CommentSearchResult represents a comment search result
type CommentSearchResult struct {
	RunnerID   int64   `json:"runner_id" db:"runner_id"`
	RaceID     int64   `json:"race_id" db:"race_id"`
	RaceDate   string  `json:"race_date" db:"race_date"`
	CourseName string  `json:"course_name" db:"course_name"`
	RaceName   string  `json:"race_name" db:"race_name"`
	HorseName  string  `json:"horse_name" db:"horse_name"`
	Comment    string  `json:"comment" db:"comment"`
	Rank       float64 `json:"rank" db:"rank"`
}

// CommentSearchParams represents comment search parameters
type CommentSearchParams struct {
	Query    string  `form:"q" binding:"required"`
	DateFrom *string `form:"date_from"`
	DateTo   *string `form:"date_to"`
	Region   *string `form:"region"`
	CourseID *int64  `form:"course_id"`
	Limit    int     `form:"limit"`
}
