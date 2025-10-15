package betfair

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"giddyup/api/internal/scraper"
)

// RaceMapping maps a Betfair market to a database race
type RaceMapping struct {
	MarketID    string
	RaceID      int64
	Venue       string
	OffTime     string
	Runners     map[int64]int64 // selectionID → runner_id
	RunnerNames map[int64]string // selectionID → normalized horse name (for debugging)
}

// Matcher handles Racing Post ↔ Betfair matching
type Matcher struct {
	client *Client
}

// NewMatcher creates a new matcher
func NewMatcher(client *Client) *Matcher {
	return &Matcher{client: client}
}

// FindTodaysMarkets discovers today's UK/IRE horse racing markets
func (m *Matcher) FindTodaysMarkets(ctx context.Context, date string) ([]MarketCatalogue, error) {
	// Parse date to get time range
	startDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return nil, err
	}
	endDate := startDate.AddDate(0, 0, 1) // Next day

	filter := MarketFilter{
		EventTypeIds:    []string{"7"}, // 7 = Horse Racing
		MarketCountries: []string{"GB", "IE"}, // UK and Ireland
		MarketTypeCodes: []string{"WIN"}, // WIN markets only
		MarketStartTime: &TimeRange{
			From: &startDate,
			To:   &endDate,
		},
	}

	projection := []MarketProjection{
		ProjectionEvent,
		ProjectionRunnerDescription,
		ProjectionMarketStartTime,
	}

	markets, err := m.client.ListMarketCatalogue(ctx, filter, projection, SortFirstToStart, 500)
	if err != nil {
		return nil, fmt.Errorf("list markets: %w", err)
	}

	log.Printf("[Betfair] Found %d markets for %s (GB/IE WIN markets)", len(markets), date)
	return markets, nil
}

// MatchRacesToMarkets maps Racing Post races to Betfair markets
func (m *Matcher) MatchRacesToMarkets(rpRaces []scraper.Race, bfMarkets []MarketCatalogue, raceIDMap map[string]int64) map[string]*RaceMapping {
	mappings := make(map[string]*RaceMapping)

	// Build Betfair market lookup by (normalized venue, off time)
	bfMap := make(map[string]MarketCatalogue)
	for _, market := range bfMarkets {
		if market.Event == nil || market.MarketStartTime == nil {
			continue
		}

		venue := scraper.NormalizeName(market.Event.Venue)
		offTime := market.MarketStartTime.Format("15:04")
		key := fmt.Sprintf("%s|%s", venue, offTime)
		bfMap[key] = market
	}

	// Match Racing Post races
	matched := 0
	for _, race := range rpRaces {
		normCourse := scraper.NormalizeName(race.Course)
		
		// Try exact time match first
		key := fmt.Sprintf("%s|%s", normCourse, race.OffTime)
		bfMarket, found := bfMap[key]
		
		// Try ±1 minute tolerance
		if !found {
			bfMarket, found = m.findMarketWithTimeTolerance(bfMap, normCourse, race.OffTime)
		}

		if !found {
			log.Printf("[Matcher] No Betfair market for: %s @ %s", race.Course, race.OffTime)
			continue
		}

		// Get race_id from our map
		raceKey := generateRaceKeyHelper(race)
		raceID, ok := raceIDMap[raceKey]
		if !ok {
			log.Printf("[Matcher] No race_id for race_key: %s", raceKey)
			continue
		}

		// Match runners by normalized horse name
		runnerMap := make(map[int64]int64)    // selectionID → runner_id
		runnerNames := make(map[int64]string) // for debugging
		
		for _, bfRunner := range bfMarket.Runners {
			normBFHorse := scraper.NormalizeName(bfRunner.RunnerName)
			
			// Find matching RP runner
			for _, rpRunner := range race.Runners {
				normRPHorse := scraper.NormalizeName(rpRunner.Horse)
				if normBFHorse == normRPHorse && rpRunner.RunnerID > 0 {
					runnerMap[bfRunner.SelectionID] = int64(rpRunner.RunnerID)
					runnerNames[bfRunner.SelectionID] = normBFHorse
					break
				}
			}
		}

		if len(runnerMap) > 0 {
			mappings[bfMarket.MarketID] = &RaceMapping{
				MarketID:    bfMarket.MarketID,
				RaceID:      raceID,
				Venue:       race.Course,
				OffTime:     race.OffTime,
				Runners:     runnerMap,
				RunnerNames: runnerNames,
			}
			matched++
			log.Printf("[Matcher] ✓ Matched: %s @ %s → market %s (%d/%d runners)",
				race.Course, race.OffTime, bfMarket.MarketID, len(runnerMap), len(race.Runners))
		}
	}

	log.Printf("[Matcher] Matched %d/%d races with Betfair markets", matched, len(rpRaces))
	return mappings
}

// findMarketWithTimeTolerance tries to find a market within ±1 minute
func (m *Matcher) findMarketWithTimeTolerance(bfMap map[string]MarketCatalogue, venue string, offTime string) (MarketCatalogue, bool) {
	// Parse HH:MM
	t, err := time.Parse("15:04", offTime)
	if err != nil {
		return MarketCatalogue{}, false
	}

	// Try ±1 minute
	for delta := -1; delta <= 1; delta++ {
		adjusted := t.Add(time.Duration(delta) * time.Minute)
		key := fmt.Sprintf("%s|%s", venue, adjusted.Format("15:04"))
		if market, found := bfMap[key]; found {
			return market, true
		}
	}

	return MarketCatalogue{}, false
}

// Helper to generate race key (duplicate from backfill_dates - consider moving to shared location)
func generateRaceKeyHelper(race scraper.Race) string {
	// For now, use simplified key - should match what's in database
	normCourse := strings.ToLower(strings.TrimSpace(race.Course))
	normTime := race.OffTime
	normName := strings.ToLower(strings.TrimSpace(race.RaceName))
	normType := strings.ToLower(strings.TrimSpace(race.Type))
	normRegion := strings.ToUpper(strings.TrimSpace(race.Region))

	// Generate simple concatenated key (actual implementation uses MD5 hash)
	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
}


