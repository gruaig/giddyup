package scraper

// Race represents a complete race with all metadata and runners
type Race struct {
	// Race metadata
	Date     string
	Region   string
	CourseID int
	Course   string
	RaceID   int
	OffTime  string
	RaceName string
	Type     string // Flat, Chase, Hurdle, NH Flat
	Class    string
	Pattern  string // Group 1, Listed, etc.

	// Race details
	AgeBand    string
	RatingBand string
	SexRest    string
	Distance   string
	DistanceF  float64
	DistanceM  int
	DistanceY  int
	Going      string
	Surface    string
	Ran        int

	// Runners
	Runners []Runner
}

// Runner represents a single runner in a race
type Runner struct {
	RunnerID  int // Database runner_id (populated after insert)
	Num       int
	Pos       string
	Draw      int
	OvrBtn    string
	Btn       string
	HorseID   int
	Horse     string
	Age       int
	Sex       string
	Weight    string
	Lbs       int
	Headgear  string
	Time      string
	Secs      float64
	SP        string
	Dec       float64
	JockeyID  int
	Jockey    string
	TrainerID int
	Trainer   string
	Prize     string
	OR        int
	RPR       int
	TS        int
	SireID    int
	Sire      string
	DamID     int
	Dam       string
	DamsireID int
	Damsire   string
	OwnerID   int
	Owner     string
	Comment   string

	// Betfair data (populated by stitcher)
	WinBSP          float64
	WinPPWAP        float64
	WinMorningWAP   float64
	WinPPMax        float64
	WinPPMin        float64
	WinIPMax        float64
	WinIPMin        float64
	WinMorningVol   float64
	WinPreVol       float64
	WinIPVol        float64
	WinLose         float64
	PlaceBSP        float64
	PlacePPWAP      float64
	PlaceMorningWAP float64
	PlacePPMax      float64
	PlacePPMin      float64
	PlaceIPMax      float64
	PlaceIPMin      float64
	PlaceMorningVol float64
	PlacePreVol     float64
	PlaceIPVol      float64
	PlaceWinLose    float64
}

// Racecard represents today's/tomorrow's upcoming race
type Racecard struct {
	Date      string
	CourseID  int
	Course    string
	OffTime   string
	RaceName  string
	Distance  string
	Going     string
	Surface   string
	Weather   string
	Stalls    string
	FieldSize int
	Runners   []RacecardRunner
}

// RacecardRunner represents a runner in an upcoming race (pre-results)
type RacecardRunner struct {
	Num       int
	Draw      int
	HorseID   int
	Horse     string
	Age       int
	Sex       string
	Weight    int
	OR        int
	RPR       int
	TS        int
	JockeyID  int
	Jockey    string
	TrainerID int
	Trainer   string
	Form      string
	LastRun   string
}

// BetfairPrice represents a single Betfair BSP record
type BetfairPrice struct {
	Region          string
	Date            string
	Course          string
	OffTime         string
	Horse           string
	WinBSP          float64
	WinPPWAP        float64
	WinMorningWAP   float64
	WinPPMax        float64
	WinPPMin        float64
	WinIPMax        float64
	WinIPMin        float64
	WinMorningVol   float64
	WinPreVol       float64
	WinIPVol        float64
	WinLose         float64
	PlaceBSP        float64
	PlacePPWAP      float64
	PlaceMorningWAP float64
	PlacePPMax      float64
	PlacePPMin      float64
	PlaceIPMax      float64
	PlaceIPMin      float64
	PlaceMorningVol float64
	PlacePreVol     float64
	PlaceIPVol      float64
	PlaceWinLose    float64
}
