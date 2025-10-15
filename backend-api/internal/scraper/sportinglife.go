package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SportingLifeScraper struct {
	client           *http.Client
	userAgents       []string
	lastRequestTime  time.Time
	consecutiveFails int
}

func NewSportingLifeScraper() *SportingLifeScraper {
	return &SportingLifeScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgents: []string{
			// Chrome - various versions and platforms
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/118.0.0.0 Safari/537.36",
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
			// Firefox - various versions
			"Mozilla/5.0 (X11; Linux x86_64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:121.0) Gecko/20100101 Firefox/121.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:120.0) Gecko/20100101 Firefox/120.0",
			// Safari
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.1 Safari/605.1.15",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.6 Safari/605.1.15",
			// Edge
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/119.0.0.0 Safari/537.36 Edg/119.0.0.0",
			// Opera
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 OPR/106.0.0.0",
		},
		lastRequestTime:  time.Now(),
		consecutiveFails: 0,
	}
}

// randomUserAgent returns a random user agent string
func (s *SportingLifeScraper) randomUserAgent() string {
	return s.userAgents[rand.Intn(len(s.userAgents))]
}

// rateLimit ensures we don't spam Sporting Life
func (s *SportingLifeScraper) rateLimit() {
	elapsed := time.Since(s.lastRequestTime)
	minDelay := 400 * time.Millisecond // Minimum 400ms between requests
	
	if elapsed < minDelay {
		sleep := minDelay - elapsed
		time.Sleep(sleep)
	}
	
	s.lastRequestTime = time.Now()
}

// GetRacesForDate fetches races from Sporting Life with full runner details
// date can be: "2025-10-15", "today", "tomorrow", or any valid date
func (s *SportingLifeScraper) GetRacesForDate(date string) ([]Race, error) {
	// Step 1: Get race list from main page
	url := fmt.Sprintf("https://www.sportinglife.com/racing/racecards/%s", date)
	
	log.Printf("[SportingLife] Fetching race list for %s...", date)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers to look like a real browser (with rotation)
	req.Header.Set("User-Agent", s.randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9,en-US;q=0.8")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("DNT", "1")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	
	// Rate limit before request
	s.rateLimit()
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from Sporting Life", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Extract JSON from <script id="__NEXT_DATA__">...</script>
	re := regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	match := re.FindSubmatch(body)
	if len(match) < 2 {
		return nil, fmt.Errorf("no __NEXT_DATA__ found in Sporting Life response")
	}
	
	var data SportingLifeData
	if err := json.Unmarshal(match[1], &data); err != nil {
		return nil, fmt.Errorf("failed to parse Sporting Life JSON: %w", err)
	}
	
	// Filter to UK/IRE races and collect race URLs
	var raceURLs []struct {
		url     string
		meeting SportingLifeMeeting
		race    SportingLifeRace
	}
	
	for _, meeting := range data.Props.PageProps.Meetings {
		country := meeting.MeetingSummary.Course.Country.ShortName
		if country != "ENG" && country != "Eire" {
			continue
		}
		
		for _, slRace := range meeting.Races {
			// Build individual race URL
			// Pattern: /racing/racecards/2025-10-15/nottingham/racecard/885027/race-name
			courseName := strings.ToLower(strings.ReplaceAll(slRace.CourseName, " ", "-"))
			raceName := strings.ToLower(strings.ReplaceAll(slRace.Name, " ", "-"))
			raceName = strings.ReplaceAll(raceName, "'", "")
			raceName = strings.ReplaceAll(raceName, "(", "")
			raceName = strings.ReplaceAll(raceName, ")", "")
			
			raceURL := fmt.Sprintf("https://www.sportinglife.com/racing/racecards/%s/%s/racecard/%d/%s",
				slRace.Date, courseName, slRace.RaceSummaryReference.ID, raceName)
			
			raceURLs = append(raceURLs, struct {
				url     string
				meeting SportingLifeMeeting
				race    SportingLifeRace
			}{raceURL, meeting, slRace})
		}
	}
	
	log.Printf("[SportingLife] Found %d UK/IRE races, fetching runner details...", len(raceURLs))
	
	// Step 2: Fetch each individual race page for runner details
	var races []Race
	for i, item := range raceURLs {
		log.Printf("[SportingLife] Fetching race %d/%d: %s", i+1, len(raceURLs), item.race.CourseName)
		
		// Fetch individual race page
		raceWithRunners, err := s.fetchRaceDetails(item.url, item.meeting, item.race)
		if err != nil {
			log.Printf("[SportingLife] ⚠️  Failed to fetch runners for race %d: %v", item.race.RaceSummaryReference.ID, err)
			// Use race without runners as fallback
			raceWithRunners = s.convertToRace(item.race, item.meeting)
		}
		
		races = append(races, raceWithRunners)
		
		// No additional sleep needed - rateLimit() handles it automatically
	}
	
	log.Printf("[SportingLife] ✅ Fetched %d races with full runner data for %s", len(races), date)
	return races, nil
}

// fetchRaceDetails fetches an individual race page to get runner details
func (s *SportingLifeScraper) fetchRaceDetails(url string, meeting SportingLifeMeeting, baseRace SportingLifeRace) (Race, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Race{}, err
	}
	
	// Set headers with rotation
	req.Header.Set("User-Agent", s.randomUserAgent())
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-GB,en;q=0.9,en-US;q=0.8")
	req.Header.Set("DNT", "1")
	req.Header.Set("Referer", "https://www.sportinglife.com/racing/racecards")
	
	// Rate limit
	s.rateLimit()
	
	resp, err := s.client.Do(req)
	if err != nil {
		return Race{}, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return Race{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Race{}, err
	}
	
	// Extract JSON from individual race page
	re := regexp.MustCompile(`<script id="__NEXT_DATA__"[^>]*>(.*?)</script>`)
	match := re.FindSubmatch(body)
	if len(match) < 2 {
		return Race{}, fmt.Errorf("no JSON found in race page")
	}
	
	var data SportingLifeData
	if err := json.Unmarshal(match[1], &data); err != nil {
		return Race{}, fmt.Errorf("failed to parse race JSON: %w", err)
	}
	
	// Individual race pages have 'race' key at top level (not 'meetings')
	if data.Props.PageProps.Race != nil {
		raceData := *data.Props.PageProps.Race
		if len(raceData.Rides) > 0 {
			log.Printf("[SportingLife]    ✓ Got %d runners for race %d", len(raceData.Rides), raceData.RaceSummaryReference.ID)
			return s.convertToRace(raceData, meeting), nil
		}
	}
	
	// Try meetings structure as fallback
	if len(data.Props.PageProps.Meetings) > 0 {
		for _, m := range data.Props.PageProps.Meetings {
			for _, r := range m.Races {
				if r.RaceSummaryReference.ID == baseRace.RaceSummaryReference.ID && len(r.Rides) > 0 {
					return s.convertToRace(r, meeting), nil
				}
			}
		}
	}
	
	// Fallback: use base race without runners
	return s.convertToRace(baseRace, meeting), nil
}

// convertToRace converts Sporting Life format to our internal Race format
func (s *SportingLifeScraper) convertToRace(slRace SportingLifeRace, meeting SportingLifeMeeting) Race {
	race := Race{
		Date:     slRace.Date,
		Course:   slRace.CourseName,
		CourseID: meeting.MeetingSummary.Course.CourseReference.ID,
		RaceID:   slRace.RaceSummaryReference.ID,
		RaceName: slRace.Name,
		OffTime:  slRace.Time, // Already in HH:MM format (12:35)
		Distance: slRace.Distance,
		Going:    slRace.Going,
		Class:    slRace.RaceClass,
		Surface:  strings.Title(strings.ToLower(slRace.CourseSurface.Surface)), // TURF → Turf
		Ran:      slRace.RideCount,
		AgeBand:  slRace.Age, // "3YO plus", "2YO only", etc.
	}
	
	// Map region
	if meeting.MeetingSummary.Course.Country.ShortName == "ENG" {
		race.Region = "GB"
	} else if meeting.MeetingSummary.Course.Country.ShortName == "Eire" {
		race.Region = "IRE"
	}
	
	// Determine race type from name and handicap flag
	race.Type = s.extractRaceType(slRace.Name, slRace.HasHandicap)
	
	// Convert runners
	race.Runners = s.convertRunners(slRace.Rides)
	
	return race
}

// convertRunners converts Sporting Life rides to our Runner format
func (s *SportingLifeScraper) convertRunners(rides []SportingLifeRide) []Runner {
	var runners []Runner
	
	for _, ride := range rides {
		// Skip non-runners
		if ride.RideStatus == "NONRUNNER" {
			continue
		}
		
		runner := Runner{
			Num:     ride.ClothNumber,
			Draw:    ride.DrawNumber,
			Horse:   ride.Horse.Name,
			HorseID: ride.Horse.HorseReference.ID,
			Age:     ride.Horse.Age,
			Weight:  ride.Handicap, // Keep as string "8-7"
			OR:      ride.OfficialRating,
			Comment: ride.Commentary, // Runner commentary/tips
		}
		
		// Position (for results)
		if ride.FinishPosition > 0 {
			runner.Pos = strconv.Itoa(ride.FinishPosition)
		}
		
		// Jockey
		if ride.Jockey != nil {
			runner.Jockey = ride.Jockey.Name
			runner.JockeyID = ride.Jockey.PersonReference.ID
		}
		
		// Trainer
		if ride.Trainer != nil {
			runner.Trainer = ride.Trainer.Name
			runner.TrainerID = ride.Trainer.BusinessReference.ID
		}
		
		// Owner
		if ride.Owner != nil {
			runner.Owner = ride.Owner.Name
			runner.OwnerID = 0 // Will be populated by database lookup
		}
		
		// Sex
		if ride.Horse.Sex != nil {
			runner.Sex = ride.Horse.Sex.Type
		}
		
		// Headgear (combine into string like "b, t")
		if len(ride.Headgear) > 0 {
			var headgearSymbols []string
			for _, hg := range ride.Headgear {
				headgearSymbols = append(headgearSymbols, hg.Symbol)
			}
			runner.Headgear = strings.Join(headgearSymbols, ", ")
		}
		
		runners = append(runners, runner)
	}
	
	return runners
}

// extractRaceType determines race type from name and handicap flag
func (s *SportingLifeScraper) extractRaceType(name string, isHandicap bool) string {
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
	if strings.Contains(nameLower, "nh flat") || strings.Contains(nameLower, "bumper") || strings.Contains(nameLower, "flat race") {
		return "NH Flat"
	}
	
	// Default to Flat
	if isHandicap {
		return "Handicap"
	}
	return "Flat"
}

