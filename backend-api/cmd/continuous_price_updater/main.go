package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"giddyup/api/internal/betfair"
	"giddyup/api/internal/config"
	"giddyup/api/internal/database"
	"giddyup/api/internal/scraper"

	_ "github.com/lib/pq"
)

func main() {
	// Parse flags
	targetDate := flag.String("date", time.Now().AddDate(0, 0, 1).Format("2006-01-02"), "Target date (default: tomorrow)")
	interval := flag.Int("interval", 30, "Update interval in minutes")
	flag.Parse()

	log.Println("🏇 GiddyUp Continuous Betfair Price Updater")
	log.Printf("📅 Target Date: %s", *targetDate)
	log.Printf("⏱️  Update Interval: %d minutes", *interval)
	log.Println()

	// Load config
	cfg := config.MustLoad()

	// Connect to database
	log.Println("🔌 Connecting to database...")
	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Database connected")
	log.Println()

	// Create Betfair client
	appKey := os.Getenv("BETFAIR_APP_KEY")
	sessionToken := os.Getenv("BETFAIR_SESSION_TOKEN")

	if appKey == "" {
		log.Fatal("❌ BETFAIR_APP_KEY not set in environment")
	}

	// Authenticate if no session token
	if sessionToken == "" {
		username := os.Getenv("BETFAIR_USERNAME")
		password := os.Getenv("BETFAIR_PASSWORD")

		if username == "" || password == "" {
			log.Fatal("❌ Need BETFAIR_SESSION_TOKEN or BETFAIR_USERNAME/PASSWORD")
		}

		log.Println("🔐 Authenticating with Betfair...")
		auth := betfair.NewAuthenticator(appKey, username, password)
		sessionToken, err = auth.Login()
		if err != nil {
			log.Fatalf("❌ Betfair authentication failed: %v", err)
		}
		log.Println("✅ Authenticated with Betfair")
	}

	bfClient := betfair.NewClient(appKey, sessionToken)
	updater := NewPriceUpdater(db.DB, bfClient, *targetDate)

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n⚠️  Shutdown signal received...")
		cancel()
	}()

	// Run continuous updates
	log.Println("🔄 Starting continuous price updates...")
	log.Printf("   Press Ctrl+C to stop\n")
	log.Println()

	if err := updater.RunContinuous(ctx, time.Duration(*interval)*time.Minute); err != nil {
		if err == context.Canceled {
			log.Println("✅ Stopped gracefully")
		} else {
			log.Fatalf("❌ Error: %v", err)
		}
	}
}

type PriceUpdater struct {
	db         *database.DB
	bfClient   *betfair.Client
	targetDate string
	matcher    *betfair.Matcher
}

func NewPriceUpdater(db *database.DB, bfClient *betfair.Client, targetDate string) *PriceUpdater {
	return &PriceUpdater{
		db:         db,
		bfClient:   bfClient,
		targetDate: targetDate,
		matcher:    betfair.NewMatcher(bfClient),
	}
}

func (u *PriceUpdater) RunContinuous(ctx context.Context, interval time.Duration) error {
	// Run immediately
	if err := u.updatePrices(ctx); err != nil {
		log.Printf("⚠️  Initial update failed: %v", err)
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := u.updatePrices(ctx); err != nil {
				log.Printf("⚠️  Update failed: %v", err)
			}
		}
	}
}

func (u *PriceUpdater) updatePrices(ctx context.Context) error {
	startTime := time.Now()
	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🔄 Fetching prices at %s", startTime.Format("15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Step 1: Get races from database
	races, err := u.getRacesFromDB()
	if err != nil {
		return fmt.Errorf("get races: %w", err)
	}

	log.Printf("📊 Found %d races for %s", len(races), u.targetDate)

	if len(races) == 0 {
		return fmt.Errorf("no races found for %s", u.targetDate)
	}

	// Step 2: Discover Betfair markets
	log.Println("🔍 Discovering Betfair markets...")
	markets, err := u.matcher.FindTodaysMarkets(ctx, u.targetDate)
	if err != nil {
		return fmt.Errorf("find markets: %w", err)
	}

	log.Printf("✅ Found %d Betfair markets", len(markets))

	if len(markets) == 0 {
		return fmt.Errorf("no Betfair markets found for %s", u.targetDate)
	}

	// Step 3: Match races to markets
	log.Println("🔀 Matching races to markets...")
	mappings := u.matchRacesToMarkets(races, markets)
	log.Printf("✅ Matched %d/%d races", len(mappings), len(races))

	if len(mappings) == 0 {
		return fmt.Errorf("no races matched to Betfair markets")
	}

	// Step 4: Fetch market books (prices)
	marketIDs := make([]string, 0, len(mappings))
	for marketID := range mappings {
		marketIDs = append(marketIDs, marketID)
	}

	log.Printf("💰 Fetching prices for %d markets...", len(marketIDs))
	marketBooks, err := u.bfClient.ListMarketBook(ctx, marketIDs)
	if err != nil {
		return fmt.Errorf("fetch market books: %w", err)
	}

	log.Printf("✅ Received %d market books", len(marketBooks))

	// Step 5: Update database
	totalUpdates := 0
	for _, book := range marketBooks {
		mapping, exists := mappings[book.MarketID]
		if !exists {
			continue
		}

		for _, runner := range book.Runners {
			runnerID, exists := mapping.Runners[runner.SelectionID]
			if !exists {
				continue
			}

			// Calculate PPWAP (weighted average of back prices)
			ppwap := calculatePPWAP(runner)
			if ppwap == 0 {
				continue
			}

			// Update win_ppwap
			_, err := u.db.Exec(`
				UPDATE racing.runners
				SET win_ppwap = $1
				WHERE runner_id = $2
			`, ppwap, runnerID)

			if err != nil {
				log.Printf("⚠️  Failed to update runner %d: %v", runnerID, err)
				continue
			}

			totalUpdates++
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ Updated %d runners with prices (took %v)", totalUpdates, elapsed.Round(time.Millisecond))
	log.Println()

	return nil
}

func (u *PriceUpdater) getRacesFromDB() ([]*scraper.Race, error) {
	query := `
		SELECT 
			r.race_id,
			r.race_name,
			r.off_time,
			c.course_name,
			r.region
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE r.race_date = $1
		ORDER BY r.off_time
	`

	rows, err := u.db.Query(query, u.targetDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var races []*scraper.Race
	for rows.Next() {
		race := &scraper.Race{}
		var offTime time.Time
		err := rows.Scan(&race.RaceID, &race.RaceName, &offTime, &race.Course, &race.Region)
		if err != nil {
			return nil, err
		}
		race.OffTime = offTime.Format("15:04")
		races = append(races, race)
	}

	return races, rows.Err()
}

func (u *PriceUpdater) matchRacesToMarkets(races []*scraper.Race, markets []betfair.Market) map[string]*betfair.RaceMapping {
	mappings := make(map[string]*betfair.RaceMapping)

	for _, market := range markets {
		for _, race := range races {
			// Match by course and time
			if matchesCourseAndTime(race, market) {
				// Get runners for this race with Betfair selection IDs
				runners, err := u.getRunnersForRace(race.RaceID)
				if err != nil {
					continue
				}

				runnerMap := make(map[int64]int64) // selectionID → runnerID
				for _, runner := range runners {
					if runner.BetfairSelectionID > 0 {
						runnerMap[runner.BetfairSelectionID] = runner.RunnerID
					}
				}

				if len(runnerMap) > 0 {
					mappings[market.MarketID] = &betfair.RaceMapping{
						RaceID:   race.RaceID,
						MarketID: market.MarketID,
						Runners:  runnerMap,
					}
				}
				break
			}
		}
	}

	return mappings
}

type DBRunner struct {
	RunnerID             int64
	BetfairSelectionID   int64
}

func (u *PriceUpdater) getRunnersForRace(raceID int64) ([]DBRunner, error) {
	query := `
		SELECT runner_id, betfair_selection_id
		FROM racing.runners
		WHERE race_id = $1
		AND betfair_selection_id IS NOT NULL
	`

	rows, err := u.db.Query(query, raceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runners []DBRunner
	for rows.Next() {
		var r DBRunner
		if err := rows.Scan(&r.RunnerID, &r.BetfairSelectionID); err != nil {
			return nil, err
		}
		runners = append(runners, r)
	}

	return runners, rows.Err()
}

func matchesCourseAndTime(race *scraper.Race, market betfair.Market) bool {
	// Simple matching by course name (normalize both)
	raceCourseLower := strings.ToLower(strings.TrimSpace(race.Course))
	marketCourseLower := strings.ToLower(strings.TrimSpace(market.Event.Venue))

	// Handle common variations
	raceCourseLower = strings.ReplaceAll(raceCourseLower, " (aw)", "")
	marketCourseLower = strings.ReplaceAll(marketCourseLower, " (aw)", "")

	return strings.Contains(raceCourseLower, marketCourseLower) || 
		   strings.Contains(marketCourseLower, raceCourseLower)
}

func calculatePPWAP(runner betfair.Runner) float64 {
	// Get best back price (most likely to be matched)
	if len(runner.Ex.AvailableToBack) > 0 {
		return runner.Ex.AvailableToBack[0].Price
	}

	// Fallback to last price traded
	if runner.LastPriceTraded > 0 {
		return runner.LastPriceTraded
	}

	return 0
}

