package scraper

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type SportingLifeScraper struct {
	client *http.Client
}

func NewSportingLifeScraper() *SportingLifeScraper {
	return &SportingLifeScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetRacesForDate fetches races from Sporting Life
// date can be: "2025-10-15", "today", "tomorrow", or any valid date
func (s *SportingLifeScraper) GetRacesForDate(date string) ([]Race, error) {
	url := fmt.Sprintf("https://www.sportinglife.com/racing/racecards/%s", date)
	
	log.Printf("[SportingLife] Fetching races for %s...", date)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers to look like a browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	
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
	
	// Filter to UK/IRE only and convert to our Race format
	var races []Race
	for _, meeting := range data.Props.PageProps.Meetings {
		country := meeting.MeetingSummary.Course.Country.ShortName
		if country != "ENG" && country != "Eire" {
			continue
		}
		
		for _, slRace := range meeting.Races {
			race := s.convertToRace(slRace, meeting)
			races = append(races, race)
		}
	}
	
	log.Printf("[SportingLife] Found %d UK/IRE races for %s", len(races), date)
	return races, nil
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

