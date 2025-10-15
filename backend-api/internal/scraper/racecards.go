package scraper

import (
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// RacecardScraper scrapes today's race cards (pre-race data)
type RacecardScraper struct {
	client     *http.Client
	userAgents []string
}

// NewRacecardScraper creates a new racecard scraper
func NewRacecardScraper() *RacecardScraper {
	return &RacecardScraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgents: []string{
			"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
			"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		},
	}
}

// GetTodaysRaces fetches today's race cards
func (s *RacecardScraper) GetTodaysRaces(date string) ([]Race, error) {
	url := fmt.Sprintf("https://www.racingpost.com/racecards/%s", date)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.userAgents[0])

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	// Find all meetings (accordion sections)
	var raceURLs []string
	var ukIreRaces int

	doc.Find("section[data-accordion-row]").Each(func(i int, meeting *goquery.Selection) {
		// Find all race links in this meeting
		meeting.Find("a.RC-meetingItem__link").Each(func(j int, raceLink *goquery.Selection) {
			href, exists := raceLink.Attr("href")
			if !exists {
				return
			}

			// href is like /racecards/32/newcastle/2025-10-15/123456
			parts := strings.Split(href, "/")
			if len(parts) >= 3 {
				courseIDStr := parts[2]
				courseID, err := strconv.Atoi(courseIDStr)
				if err == nil {
					region := getRegionFromCourseIDStatic(courseID)
					if region != "" { // Only UK or IRE
						fullURL := "https://www.racingpost.com" + href
						raceURLs = append(raceURLs, fullURL)
						ukIreRaces++
					}
				}
			}
		})
	})

	log.Printf("[RacecardScraper] Found %d UK/IRE race cards for %s", ukIreRaces, date)
	
	// Scrape each racecard
	var races []Race
	for i, url := range raceURLs {
		log.Printf("[RacecardScraper] Scraping racecard %d/%d: %s", i+1, len(raceURLs), url)
		
		race, err := s.scrapeRacecard(url, date)
		if err != nil {
			log.Printf("[RacecardScraper] Warning: Failed to scrape %s: %v", url, err)
			continue
		}
		
		races = append(races, race)
		
		// Polite delay between requests
		if i < len(raceURLs)-1 {
			time.Sleep(500 * time.Millisecond)
		}
	}
	
	log.Printf("[RacecardScraper] Successfully scraped %d/%d racecards", len(races), len(raceURLs))
	return races, nil
}

// scrapeRacecard parses a single racecard page
func (s *RacecardScraper) scrapeRacecard(url string, date string) (Race, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Race{}, err
	}
	req.Header.Set("User-Agent", s.userAgents[0])
	
	resp, err := s.client.Do(req)
	if err != nil {
		return Race{}, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return Race{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Race{}, err
	}
	
	race := Race{
		Date: date,
		Ran:  0, // Preliminary data
	}
	
	// Parse URL for course ID and race ID
	parts := strings.Split(url, "/")
	if len(parts) >= 5 {
		race.CourseID, _ = strconv.Atoi(parts[2])
		if len(parts) > 5 {
			race.RaceID, _ = strconv.Atoi(parts[5])
		}
	}
	
	// Extract course name from data attribute on any element
	doc.Find("[data-card-coursename]").Each(func(i int, s *goquery.Selection) {
		if attr, exists := s.Attr("data-card-coursename"); exists && race.Course == "" {
			race.Course = strings.TrimSpace(attr)
		}
	})
	if race.Course == "" && len(parts) >= 4 {
		// Fallback: from URL
		race.Course = strings.ReplaceAll(parts[3], "-", " ")
		race.Course = strings.Title(race.Course)
	}
	
	// Extract off time from data attribute
	doc.Find("[data-card-race-time]").Each(func(i int, s *goquery.Selection) {
		if attr, exists := s.Attr("data-card-race-time"); exists && race.OffTime == "" {
			race.OffTime = strings.TrimSpace(attr)
		}
	})
	
	// Extract race name from meta description
	if metaDesc, exists := doc.Find("meta[name='description']").Attr("content"); exists {
		// "British Stallion Studs EBF Maiden Stakes (GBB Race), 15th October 2025..."
		parts := strings.Split(metaDesc, ",")
		if len(parts) > 0 {
			race.RaceName = strings.TrimSpace(parts[0])
		}
	}
	
	// Extract distance
	race.Distance = strings.TrimSpace(doc.Find("span.RC-courseHeader__distance").First().Text())
	
	// Extract going
	race.Going = strings.TrimSpace(doc.Find("span.RC-courseHeader__going").First().Text())
	
	// Extract class
	classText := doc.Find("span.RC-courseHeader__class").First().Text()
	race.Class = strings.TrimSpace(strings.Trim(classText, "()"))
	
	// Extract race type (flat/jumps)
	race.Type = s.extractRaceType(race.RaceName, race.Distance)
	
	// Surface from going
	race.Surface = s.extractSurface(race.Going)
	
	// Determine region from course ID
	race.Region = strings.ToUpper(getRegionFromCourseIDStatic(race.CourseID))
	
	// Extract runners
	race.Runners = s.extractRunners(doc)
	race.Ran = len(race.Runners)
	
	return race, nil
}

// extractRunners parses runner information from racecard
func (s *RacecardScraper) extractRunners(doc *goquery.Document) []Runner {
	var runners []Runner
	
	doc.Find("div.RC-runnerRow").Each(func(i int, row *goquery.Selection) {
		runner := Runner{}
		
		// Runner number - from data-test-selector
		numText := row.Find("[data-test-selector='RC-cardPage-runnerNumber-no']").First().Text()
		runner.Num, _ = strconv.Atoi(strings.TrimSpace(numText))
		if runner.Num == 0 {
			runner.Num = i + 1 // Fallback to position
		}
		
		// Draw
		drawText := row.Find("[data-test-selector='RC-cardPage-runnerNumber-draw']").First().Text()
		if drawText != "" {
			runner.Draw, _ = strconv.Atoi(strings.TrimSpace(drawText))
		}
		
		// Horse name and ID
		horseLink := row.Find("a[data-test-selector='RC-cardPage-runnerName']").First()
		runner.Horse = strings.TrimSpace(horseLink.Text())
		if href, exists := horseLink.Attr("href"); exists {
			// Extract horse ID from href like /horse/123456/horse-name
			horseParts := strings.Split(href, "/")
			if len(horseParts) >= 3 {
				runner.HorseID, _ = strconv.Atoi(horseParts[2])
			}
		}
		
		// Jockey - try multiple possible selectors
		jockeyLink := row.Find("a[data-test-selector*='Jockey']").First()
		runner.Jockey = strings.TrimSpace(jockeyLink.Text())
		if href, exists := jockeyLink.Attr("href"); exists {
			jockeyParts := strings.Split(href, "/")
			if len(jockeyParts) >= 3 {
				runner.JockeyID, _ = strconv.Atoi(jockeyParts[2])
			}
		}
		
		// Trainer - try multiple possible selectors  
		trainerLink := row.Find("a[data-test-selector*='Trainer']").First()
		runner.Trainer = strings.TrimSpace(trainerLink.Text())
		if href, exists := trainerLink.Attr("href"); exists {
			trainerParts := strings.Split(href, "/")
			if len(trainerParts) >= 3 {
				runner.TrainerID, _ = strconv.Atoi(trainerParts[2])
			}
		}
		
		// Age - look for age in stats
		ageText := row.Find("[data-test-selector*='Age']").First().Text()
		runner.Age, _ = strconv.Atoi(strings.TrimSpace(ageText))
		
		// Weight - look for weight in stats
		weightText := row.Find("[data-test-selector*='Weight']").First().Text()
		if weightText != "" {
			runner.Lbs = s.parseWeight(weightText)
		}
		
		// OR (Official Rating)
		orText := row.Find("[data-test-selector*='OR']").First().Text()
		runner.OR, _ = strconv.Atoi(strings.TrimSpace(orText))
		
		// RPR (Racing Post Rating)
		rprText := row.Find("[data-test-selector*='RPR']").First().Text()
		runner.RPR, _ = strconv.Atoi(strings.TrimSpace(rprText))
		
		// Skip if no horse name
		if runner.Horse != "" {
			runners = append(runners, runner)
		}
	})
	
	return runners
}

// extractRaceType determines race type from name and distance
func (s *RacecardScraper) extractRaceType(raceName, distance string) string {
	nameLower := strings.ToLower(raceName)
	distLower := strings.ToLower(distance)
	
	if strings.Contains(nameLower, "hurdle") || strings.Contains(nameLower, "hrd") {
		return "Hurdle"
	}
	if strings.Contains(nameLower, "chase") || strings.Contains(nameLower, "chs") {
		return "Chase"
	}
	if strings.Contains(nameLower, "nh flat") || strings.Contains(nameLower, "nhf") {
		return "NH Flat"
	}
	if strings.Contains(distLower, "m") && (strings.Contains(nameLower, "national hunt") || strings.Contains(nameLower, "bumper")) {
		return "NH Flat"
	}
	
	return "Flat"
}

// extractSurface determines surface from going
func (s *RacecardScraper) extractSurface(going string) string {
	goingLower := strings.ToLower(going)
	if strings.Contains(goingLower, "polytrack") || strings.Contains(goingLower, "tapeta") ||
		strings.Contains(goingLower, "fibresand") || strings.Contains(goingLower, "standard") {
		return "AW"
	}
	return "Turf"
}

// parseWeight converts weight format to total lbs
func (s *RacecardScraper) parseWeight(weight string) int {
	weight = strings.TrimSpace(weight)
	
	// Format: "9-7" (9 stone 7 pounds)
	if strings.Contains(weight, "-") {
		parts := strings.Split(weight, "-")
		if len(parts) == 2 {
			stone, _ := strconv.Atoi(strings.TrimSpace(parts[0]))
			pounds, _ := strconv.Atoi(strings.TrimSpace(parts[1]))
			return stone*14 + pounds
		}
	}
	
	// Format: "133" (total pounds)
	lbs, _ := strconv.Atoi(weight)
	return lbs
}

// Helper function - duplicates logic from results.go
// TODO: Move to shared location
func getRegionFromCourseIDStatic(courseID int) string {
	ukCourses := map[int]bool{
		2: true, 3: true, 4: true, 7: true, 9: true, 10: true, 11: true, 12: true,
		13: true, 14: true, 15: true, 16: true, 17: true, 18: true, 19: true, 20: true,
		21: true, 22: true, 24: true, 25: true, 26: true, 27: true, 28: true, 29: true,
		30: true, 31: true, 32: true, 33: true, 34: true, 36: true, 37: true, 38: true,
		39: true, 40: true, 41: true, 42: true, 43: true, 44: true, 45: true, 46: true,
		47: true, 48: true, 49: true, 51: true, 52: true, 53: true, 54: true, 55: true,
		56: true, 57: true, 58: true, 59: true, 60: true, 61: true, 62: true, 63: true,
		107: true, 513: true,
	}

	irishCourses := map[int]bool{
		102: true, 103: true, 176: true, 177: true, 178: true, 179: true, 180: true,
		181: true, 182: true, 183: true, 184: true, 185: true, 186: true, 187: true,
		188: true, 189: true, 190: true, 191: true, 192: true, 193: true, 194: true,
		195: true, 196: true, 197: true, 198: true, 199: true,
	}

	if ukCourses[courseID] {
		return "gb"
	}
	if irishCourses[courseID] {
		return "ire"
	}
	return ""
}
