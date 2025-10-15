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

	// For now, return URLs - full implementation would scrape each racecard for runner details
	return nil, fmt.Errorf("racecard scraping not yet implemented - found %d UK/IRE URLs", ukIreRaces)
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
