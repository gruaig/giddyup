package scraper

// Sporting Life API types for the 3-endpoint flow

// Step 1: /racing/racecards/YYYY-MM-DD response
type SLRacecardsResponse []struct {
	MeetingSummary struct {
		MeetingReference struct {
			ID int `json:"id"`
		} `json:"meeting_reference"`
		Date   string `json:"date"`
		Course struct {
			CourseReference struct {
				ID int `json:"id"`
			} `json:"course_reference"`
			Name    string `json:"name"`
			Country struct {
				ShortName string `json:"short_name"`
				LongName  string `json:"long_name"`
			} `json:"country"`
		} `json:"course"`
		Going          string `json:"going"`
		SurfaceSummary string `json:"surface_summary"`
	} `json:"meeting_summary"`
	Races []struct {
		MeetingSummaryReference struct {
			ID int `json:"id"`
		} `json:"meeting_summary_reference"`
		RaceSummaryReference struct {
			ID int `json:"id"`
		} `json:"race_summary_reference"`
		Name          string `json:"name"`
		CourseName    string `json:"course_name"`
		Age           string `json:"age"`
		RaceClass     string `json:"race_class"`
		Distance      string `json:"distance"`
		Date          string `json:"date"`
		Time          string `json:"time"`
		Going         string `json:"going"`
		HasHandicap   bool   `json:"has_handicap"`
		RideCount     int    `json:"ride_count"`
		CourseSurface struct {
			Surface string `json:"surface"`
		} `json:"course_surface"`
	} `json:"races"`
}

// Step 2: /race/{id} response (full runner details)
type SLRaceResponse struct {
	Rides []struct {
		RideReference struct {
			ID int `json:"id"`
		} `json:"ride_reference"`
		ClothNumber interface{} `json:"cloth_number"` // Can be string or int
		Stall       int         `json:"stall"`
		Horse       struct {
			HorseReference struct {
				ID int `json:"id"`
			} `json:"horse_reference"`
			Name string `json:"name"`
			Age  int    `json:"age"`
		} `json:"horse"`
		Jockey struct {
			JockeyReference struct {
				ID int `json:"id"`
			} `json:"jockey_reference"`
			Name string `json:"name"`
		} `json:"jockey"`
		Trainer struct {
			TrainerReference struct {
				ID int `json:"id"`
			} `json:"trainer_reference"`
			Name string `json:"name"`
		} `json:"trainer"`
		Owner struct {
			OwnerReference struct {
				ID int `json:"id"`
			} `json:"owner_reference"`
			Name string `json:"name"`
		} `json:"owner"`
		Weight      string      `json:"weight"`
		FormSummary string      `json:"form_summary"`
		Headgear    interface{} `json:"headgear"` // Can be []string or object
	} `json:"rides"`
}

// Step 3: /v2/racing/betting/{id} response
type SLBettingResponse struct {
	RaceSummary struct {
		RaceSummaryReference struct {
			ID string `json:"id"`
		} `json:"race_summary_reference"`
		CourseName               string `json:"course_name"`
		RaceNumber               int    `json:"race_number"`
		IsUnitedKingdomOrIreland bool   `json:"is_united_kingdom_or_ireland"`
		RaceDate                 string `json:"race_date"`
		Time                     string `json:"time"`
	} `json:"raceSummary"`
	Rides []struct {
		RideReference struct {
			ID int `json:"id"`
		} `json:"ride_reference"`
		RaceReference struct {
			Time string `json:"time"`
		} `json:"race_reference"`
		CourseReference struct {
			Name string `json:"name"`
		} `json:"course_reference"`
		ClothNumber   string `json:"cloth_number"`
		HorseName     string `json:"horse_name"`
		BookmakerOdds []struct {
			BookmakerID              int     `json:"bookmakerId"`
			BookmakerName            string  `json:"bookmakerName"`
			SelectionID              string  `json:"selectionId"` // KEY: Betfair selection ID!
			FractionalOdds           string  `json:"fractionalOdds"`
			DecimalOdds              float64 `json:"decimalOdds"`
			BookmakerRaceID          string  `json:"bookmakerRaceid"`
			BookmakerMarketID        string  `json:"bookmakerMarketId"`
			EachWayAvailable         bool    `json:"eachWayAvailable"`
			NumberOfPlaces           int     `json:"numberOfPlaces"`
			PlaceFractionDenominator int     `json:"placeFractionDenominator"`
			PlaceFractionNumerator   int     `json:"placeFractionNumerator"`
			PlaceFractionalOdds      string  `json:"placeFractionalOdds"`
			DecimalOddsString        string  `json:"decimalOddsString"`
			BestOdds                 bool    `json:"bestOdds"`
		} `json:"bookmakerOdds"`
	} `json:"rides"`
}
