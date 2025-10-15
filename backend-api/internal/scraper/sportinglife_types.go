package scraper

// Sporting Life JSON structure (embedded in __NEXT_DATA__ script tag)

type SportingLifeData struct {
	Props SportingLifeProps `json:"props"`
}

type SportingLifeProps struct {
	PageProps SportingLifePageProps `json:"pageProps"`
}

type SportingLifePageProps struct {
	Meetings []SportingLifeMeeting `json:"meetings,omitempty"` // For main racecards page
	Race     *SportingLifeRace     `json:"race,omitempty"`     // For individual race page
}

type SportingLifeMeeting struct {
	MeetingSummary SportingLifeMeetingSummary `json:"meeting_summary"`
	Races          []SportingLifeRace         `json:"races"`
}

type SportingLifeMeetingSummary struct {
	Date           string                     `json:"date"`
	Course         SportingLifeCourse         `json:"course"`
	Going          string                     `json:"going"`
	SurfaceSummary string                     `json:"surface_summary"`
}

type SportingLifeCourse struct {
	CourseReference SportingLifeReference `json:"course_reference"`
	Name            string                `json:"name"`
	Country         SportingLifeCountry   `json:"country"`
}

type SportingLifeCountry struct {
	ShortName string `json:"short_name"` // "ENG", "Eire", etc.
	LongName  string `json:"long_name"`
}

type SportingLifeReference struct {
	ID int `json:"id"`
}

type SportingLifeRace struct {
	MeetingSummaryReference SportingLifeReference     `json:"meeting_summary_reference"`
	RaceSummaryReference    SportingLifeReference     `json:"race_summary_reference"`
	Name                    string                    `json:"name"`
	CourseName              string                    `json:"course_name"`
	CourseShortcode         string                    `json:"course_shortcode"`
	CourseSurface           SportingLifeCourseSurface `json:"course_surface"`
	Age                     string                    `json:"age"`
	RaceClass               string                    `json:"race_class"`
	Distance                string                    `json:"distance"`
	Date                    string                    `json:"date"`
	Time                    string                    `json:"time"`     // "12:35" in 24-hour format
	OffTime                 string                    `json:"off_time"` // "12:36:04" actual off time
	WinningTime             string                    `json:"winning_time,omitempty"`
	RideCount               int                       `json:"ride_count"`
	RaceStage               string                    `json:"race_stage"` // "DORMANT", "WEIGHEDIN", etc.
	Going                   string                    `json:"going"`
	HasHandicap             bool                      `json:"has_handicap"`
	Rides                   []SportingLifeRide        `json:"rides,omitempty"`
}

type SportingLifeCourseSurface struct {
	Surface string `json:"surface"` // "TURF", "POLYTRACK", "DIRT", etc.
}

type SportingLifeRide struct {
	RideReference    SportingLifeReference       `json:"ride_reference"`
	ClothNumber      int                         `json:"cloth_number"`
	DrawNumber       int                         `json:"draw_number"`
	FinishPosition   int                         `json:"finish_position"`
	RideStatus       string                      `json:"ride_status"` // "RUNNER", "NONRUNNER"
	Horse            SportingLifeHorse           `json:"horse"`
	Handicap         string                      `json:"handicap"` // "8-7" (weight)
	OfficialRating   int                         `json:"official_rating"`
	Jockey           *SportingLifePerson         `json:"jockey,omitempty"`
	Trainer          *SportingLifeBusiness       `json:"trainer,omitempty"`
	Owner            *SportingLifeOwner          `json:"owner,omitempty"`
	Betting          *SportingLifeBetting        `json:"betting,omitempty"`
	Commentary       string                      `json:"commentary,omitempty"` // Runner comments
	Headgear         []SportingLifeHeadgear      `json:"headgear,omitempty"`
}

type SportingLifeHeadgear struct {
	Symbol string `json:"symbol"` // "b", "t", "p", "v", etc.
	Name   string `json:"name"`   // "Blinkers", "Tongue strap", etc.
	Count  int    `json:"count"`  // How many times worn
}

type SportingLifeHorse struct {
	HorseReference SportingLifeReference `json:"horse_reference"`
	Name           string                `json:"name"`
	Age            int                   `json:"age"`
	Sex            *SportingLifeSex      `json:"sex,omitempty"`
	FormSummary    *SportingLifeForm     `json:"formsummary,omitempty"`
	LastRanDays    int                   `json:"last_ran_days,omitempty"`
}

type SportingLifeSex struct {
	Type string `json:"type"` // "c", "f", "g", "h", "m"
}

type SportingLifeForm struct {
	DisplayText string `json:"display_text"`
}

type SportingLifePerson struct {
	PersonReference SportingLifeReference `json:"person_reference"`
	Name            string                `json:"name"`
}

type SportingLifeBusiness struct {
	BusinessReference SportingLifeReference `json:"business_reference"`
	Name              string                `json:"name"`
}

type SportingLifeOwner struct {
	Name string `json:"name"`
}

type SportingLifeBetting struct {
	CurrentOdds   string `json:"current_odds"`
	StartingPrice string `json:"starting_price,omitempty"`
}
