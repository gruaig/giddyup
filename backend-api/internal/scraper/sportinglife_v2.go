package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// SportingLifeAPIV2 uses the proper 3-endpoint flow
type SportingLifeAPIV2 struct {
	client           *http.Client
	userAgents       []string
	lastRequestTime  time.Time
	consecutiveFails int
}

func NewSportingLifeAPIV2() *SportingLifeAPIV2 {
	return &SportingLifeAPIV2{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgents: []string{
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
		},
		lastRequestTime:  time.Now(),
		consecutiveFails: 0,
	}
}

func (s *SportingLifeAPIV2) randomUserAgent() string {
	return s.userAgents[rand.Intn(len(s.userAgents))]
}

func (s *SportingLifeAPIV2) rateLimit() {
	elapsed := time.Since(s.lastRequestTime)
	minDelay := 400 * time.Millisecond

	if elapsed < minDelay {
		time.Sleep(minDelay - elapsed)
	}

	s.lastRequestTime = time.Now()
}

// raceInfo holds metadata from step 1 (racecards endpoint)
type raceInfo struct {
	ID          int
	CourseName  string
	RaceName    string
	Time        string
	Date        string
	Distance    string
	Going       string
	RaceClass   string
	Age         string
	HasHandicap bool
	Surface     string
	Country     string
}

// GetRacesForDate uses the 3-endpoint API flow
func (s *SportingLifeAPIV2) GetRacesForDate(date string) ([]Race, error) {
	// Check cache first
	cache := NewSportingLifeCache("/home/smonaghan/GiddyUp/data")
	cachedRaces, found, err := cache.LoadRaces(date)
	if err != nil {
		log.Printf("[SportingLife] Warning: Cache load error: %v", err)
	}

	if found && len(cachedRaces) > 0 {
		log.Printf("[SportingLife] ✅ Loaded %d races from cache for %s (no API calls needed)", len(cachedRaces), date)
		return cachedRaces, nil
	}

	log.Printf("[SportingLife] Fetching races for %s via API (3-endpoint flow)...", date)

	// STEP 1: Get race IDs from /racing/racecards/{date}
	racecardsURL := fmt.Sprintf("https://www.sportinglife.com/api/horse-racing/racing/racecards/%s", date)
	s.rateLimit()

	req, err := http.NewRequest("GET", racecardsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("User-Agent", s.randomUserAgent())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", "https://www.sportinglife.com/racing/racecards")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from racecards endpoint", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body failed: %w", err)
	}

	var racecardsData SLRacecardsResponse
	if err := json.Unmarshal(body, &racecardsData); err != nil {
		return nil, fmt.Errorf("parse racecards JSON failed: %w", err)
	}

	// Collect UK/IRE race IDs and metadata
	var raceIDs []raceInfo

	for _, meeting := range racecardsData {
		country := meeting.MeetingSummary.Course.Country.ShortName
		// Filter UK/IRE only
		if country != "ENG" && country != "SCO" && country != "Wale" && country != "Eire" {
			continue
		}

		for _, race := range meeting.Races {
			raceIDs = append(raceIDs, raceInfo{
				ID:          race.RaceSummaryReference.ID,
				CourseName:  race.CourseName,
				RaceName:    race.Name,
				Time:        race.Time,
				Date:        race.Date,
				Distance:    race.Distance,
				Going:       race.Going,
				RaceClass:   race.RaceClass,
				Age:         race.Age,
				HasHandicap: race.HasHandicap,
				Surface:     race.CourseSurface.Surface,
				Country:     country,
			})
		}
	}

	log.Printf("[SportingLife] Found %d UK/IRE races for %s", len(raceIDs), date)

	// STEP 2 & 3: For each race, fetch betting data (includes runners + odds + selectionId!)
	var races []Race
	for i, info := range raceIDs {
		if i > 0 {
			s.rateLimit()
		}

		race, err := s.fetchRaceWithBetting(info)
		if err != nil {
			log.Printf("[SportingLife] Warning: failed to fetch race %d (%s): %v", info.ID, info.CourseName, err)
			s.consecutiveFails++
			if s.consecutiveFails >= 3 {
				return nil, fmt.Errorf("too many consecutive failures (%d), aborting", s.consecutiveFails)
			}
			continue
		}

		s.consecutiveFails = 0
		races = append(races, race)
	}

	log.Printf("[SportingLife] Successfully fetched %d races with runners and odds", len(races))

	// Save to cache
	if err := cache.SaveRaces(date, races); err != nil {
		log.Printf("[SportingLife] Warning: Failed to save cache: %v", err)
	}

	return races, nil
}

// fetchRaceWithBetting fetches BOTH race details and betting data, then merges them
func (s *SportingLifeAPIV2) fetchRaceWithBetting(info raceInfo) (Race, error) {
	// STEP 1: Fetch race details (jockey, trainer, owner, form, etc.)
	raceURL := fmt.Sprintf("https://www.sportinglife.com/api/horse-racing/race/%d", info.ID)

	req1, err := http.NewRequest("GET", raceURL, nil)
	if err != nil {
		return Race{}, fmt.Errorf("create race request failed: %w", err)
	}
	req1.Header.Set("User-Agent", s.randomUserAgent())
	req1.Header.Set("Accept", "application/json")
	req1.Header.Set("Referer", "https://www.sportinglife.com/racing/racecards")

	resp1, err := s.client.Do(req1)
	if err != nil {
		return Race{}, fmt.Errorf("race request failed: %w", err)
	}
	defer resp1.Body.Close()

	if resp1.StatusCode != 200 {
		return Race{}, fmt.Errorf("race HTTP %d", resp1.StatusCode)
	}

	raceBody, err := io.ReadAll(resp1.Body)
	if err != nil {
		return Race{}, fmt.Errorf("read race body failed: %w", err)
	}

	var raceData SLRaceResponse
	if err := json.Unmarshal(raceBody, &raceData); err != nil {
		return Race{}, fmt.Errorf("parse race JSON failed: %w", err)
	}

	// STEP 2: Fetch betting data (odds + Betfair selection IDs)
	s.rateLimit() // Rate limit between the two requests

	bettingURL := fmt.Sprintf("https://www.sportinglife.com/api/horse-racing/v2/racing/betting/%d", info.ID)

	req2, err := http.NewRequest("GET", bettingURL, nil)
	if err != nil {
		return Race{}, fmt.Errorf("create betting request failed: %w", err)
	}
	req2.Header.Set("User-Agent", s.randomUserAgent())
	req2.Header.Set("Accept", "application/json")
	req2.Header.Set("Referer", "https://www.sportinglife.com/racing/racecards")

	resp2, err := s.client.Do(req2)
	if err != nil {
		return Race{}, fmt.Errorf("betting request failed: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != 200 {
		return Race{}, fmt.Errorf("betting HTTP %d", resp2.StatusCode)
	}

	bettingBody, err := io.ReadAll(resp2.Body)
	if err != nil {
		return Race{}, fmt.Errorf("read betting body failed: %w", err)
	}

	var bettingData SLBettingResponse
	if err := json.Unmarshal(bettingBody, &bettingData); err != nil {
		return Race{}, fmt.Errorf("parse betting JSON failed: %w", err)
	}

	// Build Race object using info from step 1 + betting data from step 3
	race := Race{
		Date:     info.Date,
		Course:   info.CourseName,
		CourseID: 0, // Will be populated by database lookup
		RaceID:   info.ID,
		RaceName: info.RaceName,
		OffTime:  s.formatOffTime(info.Time),
		Distance: info.Distance,
		Going:    info.Going,
		Class:    info.RaceClass,
		Surface:  strings.Title(strings.ToLower(info.Surface)),
		Ran:      len(bettingData.Rides),
		AgeBand:  info.Age,
	}

	// Determine region from country code
	if info.Country == "Eire" {
		race.Region = "IRE"
	} else if info.Country == "Wale" {
		race.Region = "GB" // Wales is part of GB region
	} else {
		race.Region = "GB"
	}

	// Determine race type
	race.Type = s.extractRaceType(info.RaceName, info.HasHandicap)

	// STEP 3: Merge race details + betting data
	race.Runners = s.mergeRunnerData(raceData.Rides, bettingData.Rides)

	return race, nil
}

// formatOffTime ensures time is in HH:MM:SS format for database
func (s *SportingLifeAPIV2) formatOffTime(timeStr string) string {
	// "12:35" → "12:35:00"
	if timeStr == "" {
		return ""
	}

	parts := strings.Split(timeStr, ":")
	if len(parts) == 2 {
		return timeStr + ":00"
	}
	return timeStr
}

// mergeRunnerData merges full race details with betting odds/selectionIds
func (s *SportingLifeAPIV2) mergeRunnerData(
	raceRides []struct {
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
	},
	bettingRides []struct {
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
			SelectionID              string  `json:"selectionId"`
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
	}) []Runner {
	var runners []Runner

	// Build a map of betting data by horse name for quick lookup
	bettingMap := make(map[string]struct {
		selectionID   int64
		bestOdds      float64
		bestBookmaker string
	})

	for _, bRide := range bettingRides {
		normHorse := strings.ToLower(strings.TrimSpace(bRide.HorseName))

		var betfairSelectionID int64
		var bestOdds float64
		var bestBookmaker string

		for _, odds := range bRide.BookmakerOdds {
			// Capture Betfair selection ID
			if odds.BookmakerName == "Betfair Sportsbook" && odds.SelectionID != "" {
				if selID, err := strconv.ParseInt(odds.SelectionID, 10, 64); err == nil {
					betfairSelectionID = selID
				}
			}

			// Track best odds
			if odds.BestOdds {
				bestOdds = odds.DecimalOdds
				bestBookmaker = odds.BookmakerName
			}
		}

		bettingMap[normHorse] = struct {
			selectionID   int64
			bestOdds      float64
			bestBookmaker string
		}{betfairSelectionID, bestOdds, bestBookmaker}
	}

	// Build runners from race details, then merge betting data
	for _, rRide := range raceRides {
		// Handle cloth_number being either string or int
		var clothNum int
		switch v := rRide.ClothNumber.(type) {
		case string:
			clothNum, _ = strconv.Atoi(v)
		case float64:
			clothNum = int(v)
		case int:
			clothNum = v
		}

		runner := Runner{
			Num:       clothNum,
			Draw:      rRide.Stall,
			Horse:     rRide.Horse.Name,
			HorseID:   0, // Will be looked up in DB
			Age:       rRide.Horse.Age,
			Jockey:    rRide.Jockey.Name,
			JockeyID:  0, // Will be looked up in DB
			Trainer:   rRide.Trainer.Name,
			TrainerID: 0, // Will be looked up in DB
			Owner:     rRide.Owner.Name,
			OwnerID:   0, // Will be looked up in DB
			Form:      rRide.FormSummary,
		}

		// Parse headgear (can be []string or object or null)
		if rRide.Headgear != nil {
			switch hg := rRide.Headgear.(type) {
			case []interface{}:
				var headgearStrs []string
				for _, item := range hg {
					if str, ok := item.(string); ok {
						headgearStrs = append(headgearStrs, str)
					}
				}
				runner.Headgear = strings.Join(headgearStrs, ", ")
			case map[string]interface{}:
				// If it's an object, just ignore for now
				runner.Headgear = ""
			}
		}
		runner.Headgear = strings.TrimSpace(runner.Headgear)

		// Continue with original code...
		if false { // Placeholder to avoid syntax error
		}

		// Parse weight (e.g. "11-7" → 11*14 + 7 = 161 lbs)
		if rRide.Weight != "" {
			parts := strings.Split(rRide.Weight, "-")
			if len(parts) == 2 {
				st, _ := strconv.Atoi(parts[0])
				lbs, _ := strconv.Atoi(parts[1])
				runner.Lbs = st*14 + lbs
			}
		}

		// Merge betting data
		normHorse := strings.ToLower(strings.TrimSpace(runner.Horse))
		if bettingInfo, found := bettingMap[normHorse]; found {
			runner.BetfairSelectionID = bettingInfo.selectionID
			runner.BestOdds = bettingInfo.bestOdds
			runner.BestBookmaker = bettingInfo.bestBookmaker
		}

		runners = append(runners, runner)
	}

	return runners
}

// extractRaceType determines race type from name and handicap flag
func (s *SportingLifeAPIV2) extractRaceType(name string, isHandicap bool) string {
	nameLower := strings.ToLower(name)

	// Chase
	if strings.Contains(nameLower, "chase") && !strings.Contains(nameLower, "hurdle") {
		if isHandicap {
			return "Handicap Chase"
		}
		return "Chase"
	}

	// Hurdle
	if strings.Contains(nameLower, "hurdle") {
		if isHandicap {
			return "Handicap Hurdle"
		}
		return "Hurdle"
	}

	// NH Flat / Bumper
	if strings.Contains(nameLower, "nh flat") || strings.Contains(nameLower, "bumper") {
		return "NH Flat"
	}

	// Default to Flat
	if isHandicap {
		return "Handicap"
	}
	return "Flat"
}
