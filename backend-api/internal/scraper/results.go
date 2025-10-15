package scraper

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// ResultsScraper handles scraping of race results from Racing Post
type ResultsScraper struct {
	client           *http.Client
	userAgents       []string
	delay            time.Duration
	consecutiveFails int
	lastRequestTime  time.Time
}

// NewResultsScraper creates a new results scraper with default configuration
func NewResultsScraper() *ResultsScraper {
	return &ResultsScraper{
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
		delay:            5 * time.Second, // Increased from 2s to 5s
		consecutiveFails: 0,
		lastRequestTime:  time.Now(),
	}
}

// randomUserAgent returns a random user agent string
func (s *ResultsScraper) randomUserAgent() string {
	return s.userAgents[rand.Intn(len(s.userAgents))]
}

// ScrapeDate scrapes all race results for a specific date (with caching)
// Date format: YYYY-MM-DD
func (s *ResultsScraper) ScrapeDate(date string) ([]Race, error) {
	// Check cache first
	cache := NewRaceCacheManager("/home/smonaghan/GiddyUp/data")
	cachedRaces, found, err := cache.LoadRaces(date)
	if err != nil {
		log.Printf("[Scraper] Warning: Cache load error: %v", err)
	}

	if found && len(cachedRaces) > 0 {
		log.Printf("[Scraper] ✅ Loaded %d races from cache for %s (no web scraping needed)", len(cachedRaces), date)
		return cachedRaces, nil
	}

	// Circuit breaker: if too many consecutive failures, pause
	if s.consecutiveFails >= 3 {
		pauseDuration := 5 * time.Minute
		log.Printf("[Scraper] ⚠️  Circuit breaker: %d consecutive failures, pausing for %v", s.consecutiveFails, pauseDuration)
		time.Sleep(pauseDuration)
		s.consecutiveFails = 0 // Reset after pause
	}

	log.Printf("[Scraper] Fetching race URLs for %s...", date)

	// Get race URLs for this date
	urls, err2 := s.getRaceURLsForDate(date)
	if err2 != nil {
		s.consecutiveFails++
		return nil, fmt.Errorf("failed to get race URLs: %w", err2)
	}

	log.Printf("[Scraper] Found %d race URLs for %s", len(urls), date)

	// Scrape each race
	var races []Race
	for i, url := range urls {
		log.Printf("[Scraper] Scraping race %d/%d: %s", i+1, len(urls), url)

		race, err3 := s.scrapeRaceWithRetry(url, 3)
		if err3 != nil {
			log.Printf("[Scraper] Warning: Failed to scrape %s: %v", url, err3)
			s.consecutiveFails++
			continue
		}

		s.consecutiveFails = 0 // Reset on success
		races = append(races, race)

		// Rate limiting - delay between requests with jitter
		if i < len(urls)-1 {
			jitter := time.Duration(rand.Intn(3000)) * time.Millisecond // 0-3s jitter
			sleepDuration := s.delay + jitter
			log.Printf("[Scraper] Rate limit: sleeping %v before next race", sleepDuration)
			time.Sleep(sleepDuration)
		}
	}

	log.Printf("[Scraper] Successfully scraped %d/%d races for %s", len(races), len(urls), date)

	// Save to cache
	err = cache.SaveRaces(date, races)
	if err != nil {
		log.Printf("[Scraper] Warning: Failed to save cache: %v", err)
	}

	return races, nil
}

// scrapeRaceWithRetry attempts to scrape a race with exponential backoff
func (s *ResultsScraper) scrapeRaceWithRetry(url string, maxRetries int) (Race, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		race, err := s.scrapeRace(url)
		if err == nil {
			return race, nil
		}

		lastErr = err

		// Check for rate limiting errors
		if strings.Contains(err.Error(), "429") || strings.Contains(err.Error(), "Too Many Requests") {
			waitTime := 5 * time.Minute
			log.Printf("[Scraper] ⚠️  Rate limited (429), waiting %v before retry", waitTime)
			time.Sleep(waitTime)
			continue
		}

		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			log.Printf("[Scraper] ❌ Forbidden (403) - may be blocked")
			return Race{}, fmt.Errorf("blocked by Racing Post: %w", err)
		}

		if attempt < maxRetries {
			// Exponential backoff: 30s, 120s, 270s
			backoff := time.Duration(attempt*attempt) * 30 * time.Second
			log.Printf("[Scraper] Retry %d/%d after %v (error: %v)", attempt, maxRetries, backoff, err)
			time.Sleep(backoff)
		}
	}

	return Race{}, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// getRaceURLsForDate fetches all race result URLs for a specific date
func (s *ResultsScraper) getRaceURLsForDate(date string) ([]string, error) {
	url := fmt.Sprintf("https://www.racingpost.com/results/%s", date)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", s.randomUserAgent())

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

	// Find all race links
	urls := []string{}
	doc.Find("a[data-test-selector='link-listCourseNameLink']").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if exists {
			// href is like /results/32/newcastle/2025-10-14/123456
			fullURL := "https://www.racingpost.com" + href
			urls = append(urls, fullURL)
		}
	})

	return urls, nil
}

// scrapeRace parses a single race result page
func (s *ResultsScraper) scrapeRace(url string) (Race, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return Race{}, err
	}
	req.Header.Set("User-Agent", s.randomUserAgent())

	resp, err := s.client.Do(req)
	if err != nil {
		return Race{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Race{}, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Race{}, err
	}

	race := Race{}

	// Parse URL for IDs and fallback course name
	parts := strings.Split(url, "/")
	courseFromURL := ""
	if len(parts) >= 7 {
		race.CourseID, _ = strconv.Atoi(parts[4])
		courseFromURL = parts[5]
		courseFromURL = strings.ReplaceAll(courseFromURL, "-", " ")
		courseFromURL = strings.Title(courseFromURL)
		race.Date = parts[6]
		if len(parts) > 7 {
			race.RaceID, _ = strconv.Atoi(parts[7])
		}
	}

	// Extract race metadata from HTML
	courseFromHTML := s.extractCourse(doc)

	// Use HTML course if available, otherwise fallback to URL
	if courseFromHTML != "" {
		race.Course = courseFromHTML
	} else {
		race.Course = courseFromURL
	}

	race.OffTime = s.extractOffTime(doc)

	// Debug: Log if off time is missing
	if race.OffTime == "" {
		log.Printf("[Scraper DEBUG] Missing off time for %s - trying alternative extraction", url)
		// Try to extract from page title or other locations
		race.OffTime = s.extractOffTimeAlternative(doc, parts)
	}

	race.RaceName = s.extractRaceName(doc)
	race.Going = s.extractGoing(doc)
	race.Distance = s.extractDistance(doc)
	race.Class = s.extractClass(doc)
	race.Type = s.extractRaceType(doc)
	race.Surface = s.extractSurface(race.Going)

	// Extract runners
	race.Runners = s.extractRunners(doc)
	race.Ran = len(race.Runners)

	// Determine region from course ID (simplified - would need full mapping)
	race.Region = s.getRegionFromCourseID(race.CourseID)

	// IMPORTANT: Region must be UPPERCASE for race_key generation to match Python
	race.Region = strings.ToUpper(race.Region)

	// Debug log
	if race.Course == "" {
		log.Printf("[Scraper DEBUG] Warning: Empty course for URL: %s", url)
	}

	return race, nil
}

// extractCourse extracts the course name
func (s *ResultsScraper) extractCourse(doc *goquery.Document) string {
	// Try multiple selectors
	course := doc.Find("span.rp-raceTimeCourseName_name").First().Text()
	if course == "" {
		course = doc.Find("h1.rp-courseHeader__name").First().Text()
	}
	return strings.TrimSpace(course)
}

// extractOffTime extracts the race off time
func (s *ResultsScraper) extractOffTime(doc *goquery.Document) string {
	// Try multiple selectors
	offTime := doc.Find("span.rp-raceTimeCourseName_time").First().Text()
	if offTime == "" {
		offTime = doc.Find("span.RC-courseHeader__time").First().Text()
	}
	if offTime == "" {
		// Try data attribute
		if val, exists := doc.Find("main[data-analytics-race-date-time]").Attr("data-analytics-race-date-time"); exists {
			// Format: "2025-10-10T14:30:00"
			if len(val) > 11 {
				timePart := val[11:16] // Extract HH:MM
				offTime = timePart
			}
		}
	}
	return strings.TrimSpace(offTime)
}

// extractOffTimeAlternative tries alternative methods to extract off time
func (s *ResultsScraper) extractOffTimeAlternative(doc *goquery.Document, urlParts []string) string {
	// Check data-analytics attributes
	doc.Find("[data-analytics-race-time]").Each(func(i int, sel *goquery.Selection) {
		if val, exists := sel.Attr("data-analytics-race-time"); exists {
			log.Printf("[Scraper DEBUG] Found time in analytics: %s", val)
		}
	})

	// Check page title for time
	title := doc.Find("title").First().Text()
	// Title often contains time like "14:30 Race Name"
	if len(title) > 5 && title[2] == ':' {
		return title[0:5]
	}

	return ""
}

// extractRaceName extracts the race name/title
func (s *ResultsScraper) extractRaceName(doc *goquery.Document) string {
	name := doc.Find("h2.rp-raceTimeCourseName__title").First().Text()
	if name == "" {
		name = doc.Find("span.RC-header__raceInstanceTitle").First().Text()
	}
	return CleanString(name)
}

// extractGoing extracts the going description
func (s *ResultsScraper) extractGoing(doc *goquery.Document) string {
	going := doc.Find("span.rp-raceTimeCourseName_condition").First().Text()
	if going == "" {
		// Try alternative selector
		going = doc.Find("div.RC-headerBox__going").First().Text()
		if strings.Contains(going, "Going:") {
			going = strings.TrimPrefix(going, "Going:")
		}
	}
	return strings.TrimSpace(going)
}

// extractDistance extracts the race distance
func (s *ResultsScraper) extractDistance(doc *goquery.Document) string {
	dist := doc.Find("span.rp-raceTimeCourseName_distance").First().Text()
	if dist == "" {
		dist = doc.Find("strong.RC-header__raceDistanceRound").First().Text()
	}
	return strings.TrimSpace(dist)
}

// extractClass extracts the race class
func (s *ResultsScraper) extractClass(doc *goquery.Document) string {
	class := doc.Find("span.rp-raceTimeCourseName_class").First().Text()
	class = strings.Trim(class, "()")
	return strings.TrimSpace(class)
}

// extractRaceType determines race type from name and distance
func (s *ResultsScraper) extractRaceType(doc *goquery.Document) string {
	name := strings.ToLower(s.extractRaceName(doc))

	if strings.Contains(name, "hurdle") {
		return "Hurdle"
	}
	if strings.Contains(name, "chase") || strings.Contains(name, "steeplechase") {
		return "Chase"
	}
	if strings.Contains(name, "nh flat") || strings.Contains(name, "national hunt flat") {
		return "NH Flat"
	}

	return "Flat"
}

// extractSurface determines surface from going
func (s *ResultsScraper) extractSurface(going string) string {
	goingLower := strings.ToLower(going)
	if strings.Contains(goingLower, "standard") || strings.Contains(goingLower, "slow") {
		return "AW"
	}
	if strings.Contains(goingLower, "tapeta") {
		return "Tapeta"
	}
	if strings.Contains(goingLower, "polytrack") {
		return "Polytrack"
	}
	if strings.Contains(goingLower, "fibresand") {
		return "Fibresand"
	}
	return "Turf"
}

// extractRunners extracts all runners from the results table
func (s *ResultsScraper) extractRunners(doc *goquery.Document) []Runner {
	runners := []Runner{}

	// Find runner rows in results table
	doc.Find("tr.rp-horseTable__mainRow, div.js-PC-runnerRow").Each(func(i int, sel *goquery.Selection) {
		runner := Runner{}

		// Position
		pos := sel.Find("span.rp-horseTable__pos__number").First().Text()
		runner.Pos = strings.TrimSpace(pos)

		// Draw
		draw := sel.Find("span.rp-horseTable__spanNarrow").First().Text()
		if draw != "" {
			runner.Draw, _ = strconv.Atoi(strings.TrimSpace(draw))
		}

		// Horse name and ID
		horseLink := sel.Find("a.rp-horseTable__horse__name").First()
		runner.Horse = CleanString(horseLink.Text())
		if href, exists := horseLink.Attr("href"); exists {
			// href like /horses/horse/123456/horse-name
			parts := strings.Split(href, "/")
			if len(parts) > 4 {
				runner.HorseID, _ = strconv.Atoi(parts[4])
			}
		}

		// Jockey and ID
		jockeyLink := sel.Find("a.rp-horseTable__jockey__name").First()
		runner.Jockey = CleanString(jockeyLink.Text())
		if href, exists := jockeyLink.Attr("href"); exists {
			parts := strings.Split(href, "/")
			if len(parts) > 3 {
				runner.JockeyID, _ = strconv.Atoi(parts[3])
			}
		}

		// Trainer and ID
		trainerLink := sel.Find("a.rp-horseTable__trainer__name").First()
		runner.Trainer = CleanString(trainerLink.Text())
		if href, exists := trainerLink.Attr("href"); exists {
			parts := strings.Split(href, "/")
			if len(parts) > 3 {
				runner.TrainerID, _ = strconv.Atoi(parts[3])
			}
		}

		// Age
		age := sel.Find("td.rp-horseTable__age").First().Text()
		runner.Age, _ = strconv.Atoi(strings.TrimSpace(age))

		// Weight
		weight := sel.Find("td.rp-horseTable__wgt").First().Text()
		runner.Weight = strings.TrimSpace(weight)

		// OR (Official Rating)
		or := sel.Find("td.rp-horseTable__or").First().Text()
		runner.OR, _ = strconv.Atoi(strings.TrimSpace(or))

		// RPR
		rpr := sel.Find("td.rp-horseTable__rpr").First().Text()
		runner.RPR, _ = strconv.Atoi(strings.TrimSpace(rpr))

		// SP (Starting Price)
		sp := sel.Find("span.rp-horseTable__horse__price").First().Text()
		runner.SP = strings.TrimSpace(sp)

		// Only add if we got a horse name
		if runner.Horse != "" {
			runners = append(runners, runner)
		}
	})

	return runners
}

// getRegionFromCourseID maps course ID to region (simplified version)
func (s *ResultsScraper) getRegionFromCourseID(courseID int) string {
	// This is a simplified version - would need full course mapping
	// Irish courses typically have higher IDs
	if courseID > 1000 {
		return "ire"
	}
	return "gb"
}
