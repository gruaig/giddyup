package stitcher

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"giddyup/api/internal/scraper"
)

// Stitcher handles matching Racing Post races with Betfair prices
type Stitcher struct {
	rpRaces  []scraper.Race
	bfPrices []scraper.BetfairPrice
}

// MasterRace represents a race with matched Betfair data
type MasterRace struct {
	RaceKey    string
	Date       string
	Region     string
	CourseID   int
	Course     string
	OffTime    string
	RaceName   string
	Type       string
	Class      string
	Pattern    string
	AgeBand    string
	RatingBand string
	SexRest    string
	Distance   string
	DistanceF  float64
	Going      string
	Surface    string
	Ran        int
}

// MasterRunner represents a runner with matched Betfair prices
type MasterRunner struct {
	RunnerKey string
	RaceKey   string
	RaceDate  string // Added for partition constraint
	Num       int
	Pos       string
	Draw      int
	Horse     string
	HorseID   int
	Age       int
	Jockey    string
	JockeyID  int
	Trainer   string
	TrainerID int
	Weight    string
	OR        int
	RPR       int
	TS        int
	SP        string
	Prize     string
	Comment   string

	// Betfair prices
	WinBSP        float64
	WinPPWAP      float64
	WinMorningWAP float64
	WinPPMax      float64
	WinPPMin      float64
	PlaceBSP      float64
	PlacePPWAP    float64

	// Match metadata
	MatchJaccard  float64
	MatchTimeDiff int
	MatchReason   string
}

// BetfairMatch represents a matched Betfair race
type BetfairMatch struct {
	Prices       []scraper.BetfairPrice
	Jaccard      float64
	Score        float64
	TimeDiffMins int
}

// New creates a new stitcher
func New(rpRaces []scraper.Race, bfPrices []scraper.BetfairPrice) *Stitcher {
	return &Stitcher{
		rpRaces:  rpRaces,
		bfPrices: bfPrices,
	}
}

// StitchData matches Racing Post races with Betfair prices
func (s *Stitcher) StitchData() ([]MasterRace, []MasterRunner, error) {
	log.Printf("[Stitcher] Starting to stitch %d RP races with %d BF prices", len(s.rpRaces), len(s.bfPrices))

	// Group Betfair prices by (date, normalized course)
	bfByDateCourse := s.groupBetfairPrices()
	log.Printf("[Stitcher DEBUG] Grouped BF into %d date-course combinations", len(bfByDateCourse))

	// Debug: Show first few groupings
	count := 0
	for key, prices := range bfByDateCourse {
		if count < 3 {
			log.Printf("[Stitcher DEBUG] BF Group '%s': %d prices", key, len(prices))
			count++
		}
	}

	masterRaces := []MasterRace{}
	masterRunners := []MasterRunner{}
	matchedCount := 0

	for i, rpRace := range s.rpRaces {
		// Debug first few races
		if i < 3 {
			log.Printf("[Stitcher DEBUG] RP Race #%d: date=%s, course='%s', off=%s, runners=%d",
				i+1, rpRace.Date, rpRace.Course, rpRace.OffTime, len(rpRace.Runners))
		}
		// Generate race key (include race_name and type)
		raceKey := generateRaceKey(rpRace.Date, rpRace.Region, rpRace.Course, rpRace.OffTime, rpRace.RaceName, rpRace.Type)

		// Find Betfair candidates for this race (by date only)
		key := rpRace.Date
		candidates := bfByDateCourse[key]

		// Debug first few lookups
		if i < 3 {
			log.Printf("[Stitcher DEBUG] Lookup key: '%s', candidates: %d", key, len(candidates))
		}

		// Try to match with Betfair data
		match := s.matchRace(rpRace, candidates)

		if i < 3 && match != nil {
			log.Printf("[Stitcher DEBUG] Match found: jaccard=%.2f, score=%.2f", match.Jaccard, match.Score)
		} else if i < 3 {
			log.Printf("[Stitcher DEBUG] No match found")
		}

		// Create master race
		masterRace := MasterRace{
			RaceKey:    raceKey,
			Date:       rpRace.Date,
			Region:     rpRace.Region,
			CourseID:   rpRace.CourseID,
			Course:     rpRace.Course,
			OffTime:    rpRace.OffTime,
			RaceName:   rpRace.RaceName,
			Type:       rpRace.Type,
			Class:      rpRace.Class,
			Pattern:    rpRace.Pattern,
			AgeBand:    rpRace.AgeBand,
			RatingBand: rpRace.RatingBand,
			SexRest:    rpRace.SexRest,
			Distance:   rpRace.Distance,
			DistanceF:  rpRace.DistanceF,
			Going:      rpRace.Going,
			Surface:    rpRace.Surface,
			Ran:        rpRace.Ran,
		}
		masterRaces = append(masterRaces, masterRace)

		// Stitch runners
		runners := s.stitchRunners(rpRace, raceKey, match)
		masterRunners = append(masterRunners, runners...)

		if match != nil {
			matchedCount++
			if (i+1)%10 == 0 {
				log.Printf("[Stitcher] Progress: %d/%d races processed (%d matched)", i+1, len(s.rpRaces), matchedCount)
			}
		}
	}

	log.Printf("[Stitcher] Complete: %d/%d races matched (%.1f%%)",
		matchedCount, len(s.rpRaces), float64(matchedCount)/float64(len(s.rpRaces))*100)

	return masterRaces, masterRunners, nil
}

// groupBetfairPrices groups prices by date only
func (s *Stitcher) groupBetfairPrices() map[string][]scraper.BetfairPrice {
	grouped := make(map[string][]scraper.BetfairPrice)

	for _, price := range s.bfPrices {
		// Group by date only - we'll match by time window and horses
		key := price.Date
		grouped[key] = append(grouped[key], price)
	}

	return grouped
}

// matchRace finds the best Betfair match for a Racing Post race
func (s *Stitcher) matchRace(rpRace scraper.Race, bfPrices []scraper.BetfairPrice) *BetfairMatch {
	if len(bfPrices) == 0 {
		return nil
	}

	// Group BF prices by off time
	bfByTime := make(map[string][]scraper.BetfairPrice)
	for _, bf := range bfPrices {
		bfByTime[bf.OffTime] = append(bfByTime[bf.OffTime], bf)
	}

	// Debug: Show time groupings
	if len(bfByTime) > 0 {
		log.Printf("[Stitcher DEBUG] RP time: %s, BF times: %d groups", rpRace.OffTime, len(bfByTime))
		for bfTime := range bfByTime {
			log.Printf("[Stitcher DEBUG]   BF time: %s", bfTime)
			break // Just show first one
		}
	}

	var bestMatch *BetfairMatch
	bestScore := 0.0

	for bfTime, bfGroup := range bfByTime {
		// Check if times are within 10 minute window
		timeDiff := s.timeWindowMinutes(rpRace.OffTime, bfTime)
		if timeDiff > 10 {
			continue
		}

		// Get normalized horse sets
		rpHorses := s.normalizeHorseSet(rpRace.Runners)
		bfHorses := s.extractHorseSet(bfGroup)

		// Calculate Jaccard similarity
		jaccard := jaccardSimilarity(rpHorses, bfHorses)

		// Calculate total score with bonuses
		score := jaccard

		// Bonus: Runner count matches
		if len(rpRace.Runners) == len(bfHorses) {
			score += 0.5
		}

		// Bonus: Both handicaps
		if s.bothHandicap(rpRace.RaceName, bfGroup) {
			score += 0.5
		}

		// Accept if score >= 0.5 (matching Python threshold)
		if score >= 0.5 && score > bestScore {
			bestScore = score
			bestMatch = &BetfairMatch{
				Prices:       bfGroup,
				Jaccard:      jaccard,
				Score:        score,
				TimeDiffMins: timeDiff,
			}
		}
	}

	return bestMatch
}

// stitchRunners creates master runners with Betfair prices matched
func (s *Stitcher) stitchRunners(rpRace scraper.Race, raceKey string, match *BetfairMatch) []MasterRunner {
	runners := []MasterRunner{}

	// Create map of Betfair prices by normalized horse name
	bfMap := make(map[string]*scraper.BetfairPrice)
	if match != nil {
		for i := range match.Prices {
			normName := scraper.NormalizeName(match.Prices[i].Horse)
			bfMap[normName] = &match.Prices[i]
		}
	}

	for _, rpRunner := range rpRace.Runners {
		runnerKey := generateRunnerKey(raceKey, rpRunner.Horse, rpRunner.Num, rpRunner.Draw)

		runner := MasterRunner{
			RunnerKey: runnerKey,
			RaceKey:   raceKey,
			RaceDate:  rpRace.Date, // Pass race_date through
			Num:       rpRunner.Num,
			Pos:       rpRunner.Pos,
			Draw:      rpRunner.Draw,
			Horse:     rpRunner.Horse,
			HorseID:   rpRunner.HorseID,
			Age:       rpRunner.Age,
			Jockey:    rpRunner.Jockey,
			JockeyID:  rpRunner.JockeyID,
			Trainer:   rpRunner.Trainer,
			TrainerID: rpRunner.TrainerID,
			Weight:    rpRunner.Weight,
			OR:        rpRunner.OR,
			RPR:       rpRunner.RPR,
			TS:        rpRunner.TS,
			SP:        rpRunner.SP,
			Prize:     rpRunner.Prize,
			Comment:   rpRunner.Comment,
		}

		// Match with Betfair data if available
		normName := scraper.NormalizeName(rpRunner.Horse)
		if bfPrice, found := bfMap[normName]; found {
			runner.WinBSP = bfPrice.WinBSP
			runner.WinPPWAP = bfPrice.WinPPWAP
			runner.WinMorningWAP = bfPrice.WinMorningWAP
			runner.WinPPMax = bfPrice.WinPPMax
			runner.WinPPMin = bfPrice.WinPPMin
			runner.PlaceBSP = bfPrice.PlaceBSP
			runner.PlacePPWAP = bfPrice.PlacePPWAP

			if match != nil {
				runner.MatchJaccard = match.Jaccard
				runner.MatchTimeDiff = match.TimeDiffMins
				runner.MatchReason = "jaccard"
			}
		}

		runners = append(runners, runner)
	}

	return runners
}

// normalizeHorseSet extracts normalized horse names from runners
func (s *Stitcher) normalizeHorseSet(runners []scraper.Runner) []string {
	names := make([]string, 0, len(runners))
	for _, r := range runners {
		normalized := scraper.NormalizeName(r.Horse)
		if normalized != "" {
			names = append(names, normalized)
		}
	}
	return names
}

// extractHorseSet extracts normalized horse names from Betfair prices
func (s *Stitcher) extractHorseSet(prices []scraper.BetfairPrice) []string {
	names := make([]string, 0, len(prices))
	seen := make(map[string]bool)
	for _, p := range prices {
		normalized := scraper.NormalizeName(p.Horse)
		if normalized != "" && !seen[normalized] {
			names = append(names, normalized)
			seen[normalized] = true
		}
	}
	return names
}

// timeWindowMinutes calculates difference in minutes between two time strings
func (s *Stitcher) timeWindowMinutes(time1, time2 string) int {
	t1 := s.parseTime(time1)
	t2 := s.parseTime(time2)

	diff := int(math.Abs(float64(t1 - t2)))
	return diff
}

// parseTime converts time string (HH:MM) to minutes since midnight
func (s *Stitcher) parseTime(timeStr string) int {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return 0
	}
	hours, _ := strconv.Atoi(parts[0])
	mins, _ := strconv.Atoi(parts[1])
	return hours*60 + mins
}

// bothHandicap checks if race name suggests handicap
func (s *Stitcher) bothHandicap(raceName string, bfPrices []scraper.BetfairPrice) bool {
	nameL := strings.ToLower(raceName)
	return strings.Contains(nameL, "handicap") || strings.Contains(nameL, "hcap")
}

// jaccardSimilarity calculates Jaccard similarity coefficient
func jaccardSimilarity(set1, set2 []string) float64 {
	if len(set1) == 0 || len(set2) == 0 {
		return 0.0
	}

	// Convert to maps for faster lookup
	m1 := make(map[string]bool)
	for _, s := range set1 {
		m1[s] = true
	}

	m2 := make(map[string]bool)
	for _, s := range set2 {
		m2[s] = true
	}

	// Calculate intersection
	intersection := 0
	for s := range m1 {
		if m2[s] {
			intersection++
		}
	}

	// Calculate union
	union := len(m1) + len(m2) - intersection

	if union == 0 {
		return 0.0
	}

	return float64(intersection) / float64(union)
}

// generateRaceKey creates MD5 hash for race
// Matches Python: MD5(date|REGION|course|off|race_name|type)
func generateRaceKey(date, region, course, offTime, raceName, raceType string) string {
	region = strings.ToUpper(region) // MUST be uppercase: GB not gb
	keyStr := fmt.Sprintf("%s|%s|%s|%s|%s|%s", date, region, course, offTime, raceName, raceType)
	hash := md5.Sum([]byte(keyStr))
	return hex.EncodeToString(hash[:])
}

// generateRunnerKey creates MD5 hash for runner
// Matches Python: MD5(race_key|horse|num|draw)
func generateRunnerKey(raceKey, horse string, num, draw int) string {
	keyStr := fmt.Sprintf("%s|%s|%d|%d", raceKey, horse, num, draw)
	hash := md5.Sum([]byte(keyStr))
	return hex.EncodeToString(hash[:])
}

// ParseDateYYYYMMDD parses YYYY-MM-DD date format
func ParseDateYYYYMMDD(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

// parseWeight converts weight string (e.g. "9-7") to int (pounds)
func parseWeight(weight string) int {
	if weight == "" {
		return 0
	}
	// Weight format: "9-7" means 9 stone 7 pounds
	// Convert to total pounds: (stone * 14) + pounds
	parts := strings.Split(weight, "-")
	if len(parts) != 2 {
		return 0
	}
	stone, _ := strconv.Atoi(parts[0])
	lbs, _ := strconv.Atoi(parts[1])
	return (stone * 14) + lbs
}
