package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"giddyup/api/internal/betfair"
	"giddyup/api/internal/scraper"

	_ "github.com/lib/pq"
)

func main() {
	// Parse command line arguments
	dateFlag := flag.String("date", "", "Date to fetch Betfair prices for (YYYY-MM-DD)")
	flag.Parse()

	// Allow positional argument
	if *dateFlag == "" && len(flag.Args()) > 0 {
		*dateFlag = flag.Args()[0]
	}

	if *dateFlag == "" {
		fmt.Println("❌ Error: Date is required")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  ./fetch_all_betfair <date>              # e.g., ./fetch_all_betfair 2025-10-16")
		fmt.Println("  ./fetch_all_betfair --date 2025-10-16")
		fmt.Println("")
		fmt.Println("This fetches live Betfair prices for races on the specified date.")
		fmt.Println("Requires race data to exist (run fetch_all first if needed).")
		os.Exit(1)
	}

	dateStr := *dateFlag

	// Validate date format
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		log.Fatalf("❌ Invalid date format: %s (expected YYYY-MM-DD)", dateStr)
	}

	log.Printf("🏇 GiddyUp Betfair Live Prices Fetcher")
	log.Printf("📅 Date: %s", dateStr)
	log.Println("")

	// Get database connection
	dbHost := getEnv("DB_HOST", "localhost")
	dbPort := getEnv("DB_PORT", "5432")
	dbName := getEnv("DB_NAME", "horse_db")
	dbUser := getEnv("DB_USER", "postgres")
	dbPassword := getEnv("DB_PASSWORD", "password")

	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		dbHost, dbPort, dbName, dbUser, dbPassword)

	log.Println("🔌 Connecting to database...")
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Failed to connect to database: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database ping failed: %v", err)
	}
	log.Println("✅ Database connected")
	log.Println("")

	// Get Betfair credentials
	appKey := os.Getenv("BETFAIR_APP_KEY")
	sessionToken := os.Getenv("BETFAIR_SESSION_TOKEN")
	username := os.Getenv("BETFAIR_USERNAME")
	password := os.Getenv("BETFAIR_PASSWORD")

	if appKey == "" {
		log.Fatal("❌ Error: BETFAIR_APP_KEY not set in environment")
	}

	// Authenticate if no session token
	if sessionToken == "" {
		if username == "" || password == "" {
			log.Fatal("❌ Error: BETFAIR_SESSION_TOKEN or credentials (USERNAME/PASSWORD) required")
		}

		log.Println("🔐 Authenticating with Betfair...")
		auth := betfair.NewAuthenticator(appKey, username, password)
		sessionToken, err = auth.Login()
		if err != nil {
			log.Fatalf("❌ Betfair authentication failed: %v", err)
		}
		log.Println("✅ Authenticated with Betfair")
		log.Println("")
	}

	// Run the pipeline
	if err := fetchBetfairPrices(db, dateStr, appKey, sessionToken); err != nil {
		log.Fatalf("❌ Failed: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// fetchBetfairPrices runs the complete pipeline to fetch and store live Betfair prices
func fetchBetfairPrices(db *sql.DB, dateStr string, appKey, sessionToken string) error {
	ctx := context.Background()

	// Step 1: Check if we have race data in database
	log.Printf("📥 [1/4] Loading races from database for %s...", dateStr)
	races, raceIDMap, err := loadRacesFromDB(db, dateStr)
	if err != nil {
		return fmt.Errorf("load races from DB: %w", err)
	}
	if len(races) == 0 {
		return fmt.Errorf("no races found in database for %s - run fetch_all first", dateStr)
	}
	log.Printf("✅ Found %d races in database", len(races))
	log.Println("")

	// Step 2: Discover Betfair markets
	log.Printf("🔍 [2/4] Discovering Betfair WIN markets for %s...", dateStr)
	bfClient := betfair.NewClient(appKey, sessionToken)
	matcher := betfair.NewMatcher(bfClient)

	markets, err := matcher.FindTodaysMarkets(ctx, dateStr)
	if err != nil {
		return fmt.Errorf("discover Betfair markets: %w", err)
	}
	log.Printf("✅ Found %d Betfair WIN markets", len(markets))
	log.Println("")

	// Step 3: Match races to markets
	log.Printf("🔀 [3/4] Matching races to Betfair markets...")
	mappings := matcher.MatchRacesToMarkets(races, markets, raceIDMap)

	if len(mappings) == 0 {
		log.Println("⚠️  No races matched to Betfair markets")
		return nil
	}

	log.Printf("✅ Matched %d races to Betfair markets", len(mappings))
	log.Println("")

	// Step 4: Fetch live prices
	log.Printf("💰 [4/4] Fetching live Betfair prices...")
	updated, skipped, err := fetchAndUpdatePrices(ctx, db, bfClient, mappings, dateStr)
	if err != nil {
		return fmt.Errorf("fetch prices: %w", err)
	}

	log.Println("")
	log.Println("🎉 SUCCESS!")
	log.Printf("✅ Updated %d races with live prices", updated)
	if skipped > 0 {
		log.Printf("⏭  Skipped %d races (past off time)", skipped)
	}

	return nil
}

// loadRacesFromDB loads races with runner IDs from database
// Copied from internal/services/autoupdate.go
func loadRacesFromDB(db *sql.DB, dateStr string) ([]scraper.Race, map[string]int64, error) {
	rows, err := db.Query(`
		SELECT 
			r.race_id, r.race_date, r.region, c.course_name, r.off_time,
			r.race_name, r.race_type, r.class, r.dist_raw, r.going, r.surface, r.ran,
			run.runner_id, run.num, run.draw, h.horse_id, h.horse_name,
			j.jockey_id, j.jockey_name, t.trainer_id, t.trainer_name,
			run.age, run.lbs, run.or, run.rpr
		FROM racing.races r
		JOIN racing.courses c ON r.course_id = c.course_id
		LEFT JOIN racing.runners run ON run.race_id = r.race_id
		LEFT JOIN racing.horses h ON h.horse_id = run.horse_id
		LEFT JOIN racing.jockeys j ON j.jockey_id = run.jockey_id
		LEFT JOIN racing.trainers t ON t.trainer_id = run.trainer_id
		WHERE r.race_date = $1
		ORDER BY r.race_id, run.num
	`, dateStr)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	racesMap := make(map[int64]*scraper.Race)
	raceIDMap := make(map[string]int64)

	for rows.Next() {
		var raceID int64
		var raceDate, region, course, raceName, raceType string
		var offTime, class, distance, going, surface sql.NullString
		var ran int
		var runnerID, horseID, jockeyID, trainerID sql.NullInt64
		var num, draw, age, lbs, or, rpr sql.NullInt32
		var horse, jockey, trainer sql.NullString

		err := rows.Scan(
			&raceID, &raceDate, &region, &course, &offTime,
			&raceName, &raceType, &class, &distance, &going, &surface, &ran,
			&runnerID, &num, &draw, &horseID, &horse,
			&jockeyID, &jockey, &trainerID, &trainer,
			&age, &lbs, &or, &rpr,
		)
		if err != nil {
			return nil, nil, err
		}

		// Extract just HH:MM:SS from time value
		var offTimeStr string
		if offTime.Valid {
			parts := strings.Split(offTime.String, "T")
			if len(parts) == 2 {
				offTimeStr = strings.TrimSuffix(parts[1], "Z")
			} else {
				offTimeStr = offTime.String
			}
		}

		// Clean date: extract YYYY-MM-DD from potentially timestamped value
		cleanDate := raceDate
		if strings.Contains(cleanDate, "T") {
			cleanDate = strings.Split(cleanDate, "T")[0]
		}

		// Get or create race
		race, exists := racesMap[raceID]
		if !exists {
			race = &scraper.Race{
				RaceID:   int(raceID),
				Date:     cleanDate,
				Region:   region,
				Course:   course,
				OffTime:  offTimeStr,
				RaceName: raceName,
				Type:     raceType,
				Class:    class.String,
				Distance: distance.String,
				Going:    going.String,
				Surface:  surface.String,
				Ran:      ran,
			}
			racesMap[raceID] = race
			raceIDMap[generateRaceKey(*race)] = raceID
		}

		// Add runner if present
		if runnerID.Valid {
			runner := scraper.Runner{
				RunnerID:  int(runnerID.Int64),
				Num:       int(num.Int32),
				Draw:      int(draw.Int32),
				HorseID:   int(horseID.Int64),
				Horse:     horse.String,
				JockeyID:  int(jockeyID.Int64),
				Jockey:    jockey.String,
				TrainerID: int(trainerID.Int64),
				Trainer:   trainer.String,
				Age:       int(age.Int32),
				Lbs:       int(lbs.Int32),
				OR:        int(or.Int32),
				RPR:       int(rpr.Int32),
			}
			race.Runners = append(race.Runners, runner)
		}
	}

	// Convert map to slice
	races := make([]scraper.Race, 0, len(racesMap))
	for _, race := range racesMap {
		races = append(races, *race)
	}

	return races, raceIDMap, nil
}

// fetchAndUpdatePrices fetches live prices and updates database
// Based on internal/services/liveprices.go fetchAndUpdate()
func fetchAndUpdatePrices(ctx context.Context, db *sql.DB, client *betfair.Client,
	mappings map[string]*betfair.RaceMapping, dateStr string) (int, int, error) {

	now := time.Now()

	// Filter markets: only fetch prices for races that haven't finished yet
	var activeMarketIDs []string
	activeMap := make(map[string]*betfair.RaceMapping)

	for marketID, mapping := range mappings {
		// Parse off time (handle both HH:MM and HH:MM:SS formats)
		datetime := fmt.Sprintf("%s %s", dateStr, mapping.OffTime)
		var offTime time.Time
		var err error

		// Try HH:MM:SS first
		offTime, err = time.ParseInLocation("2006-01-02 15:04:05", datetime, scraper.UK())
		if err != nil {
			// Fallback to HH:MM
			offTime, err = time.ParseInLocation("2006-01-02 15:04", datetime, scraper.UK())
			if err != nil {
				log.Printf("   ⚠️  Skipping %s @ %s (invalid time)", mapping.Venue, mapping.OffTime)
				continue
			}
		}

		// Only fetch if race hasn't finished yet (within 30 mins past off time)
		cutoff := offTime.Add(30 * time.Minute)
		if now.Before(cutoff) {
			activeMarketIDs = append(activeMarketIDs, marketID)
			activeMap[marketID] = mapping
		}
	}

	skipped := len(mappings) - len(activeMarketIDs)
	if skipped > 0 {
		log.Printf("   ⏭  Skipping %d markets (past off time + 30min)", skipped)
	}

	if len(activeMarketIDs) == 0 {
		log.Println("   ℹ️  No active markets to fetch (all races finished)")
		return 0, skipped, nil
	}

	log.Printf("   📊 Fetching prices for %d active markets...", len(activeMarketIDs))

	// Fetch live market books in batches (max 10 markets at a time to avoid API limits)
	var marketBooks []betfair.MarketBook
	batchSize := 10
	for i := 0; i < len(activeMarketIDs); i += batchSize {
		end := i + batchSize
		if end > len(activeMarketIDs) {
			end = len(activeMarketIDs)
		}

		batch := activeMarketIDs[i:end]
		log.Printf("      • Fetching batch %d-%d...", i+1, end)

		books, err := client.ListMarketBook(ctx, batch)
		if err != nil {
			log.Printf("      ⚠️  Warning: Batch %d-%d failed: %v", i+1, end, err)
			continue
		}

		marketBooks = append(marketBooks, books...)
	}

	log.Printf("   ✓ Got %d market books", len(marketBooks))

	// Update database with prices
	ts := time.Now()
	totalUpdates := 0

	for _, book := range marketBooks {
		mapping, exists := activeMap[book.MarketID]
		if !exists {
			continue
		}

		racesUpdated := 0

		// Process each runner (SAME logic as liveprices.go)
		for _, runner := range book.Runners {
			runnerID, exists := mapping.Runners[runner.SelectionID]
			if !exists {
				continue
			}

			// Extract prices (SAME as liveprices.go)
			backPrice, layPrice, vwap := extractPrices(runner)

			// Insert into live_prices table (SAME as liveprices.go)
			_, err := db.Exec(`
				INSERT INTO racing.live_prices (race_id, runner_id, ts, back_price, lay_price, vwap, traded_vol)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (runner_id, ts) DO UPDATE SET
					back_price = EXCLUDED.back_price,
					lay_price = EXCLUDED.lay_price,
					vwap = EXCLUDED.vwap,
					traded_vol = EXCLUDED.traded_vol
			`, mapping.RaceID, runnerID, ts, nullFloat(backPrice), nullFloat(layPrice), nullFloat(vwap), runner.TotalMatched)

			if err != nil {
				log.Printf("   ⚠️  Failed to insert price for runner %d: %v", runnerID, err)
				continue
			}

			racesUpdated++
		}

		if racesUpdated > 0 {
			totalUpdates++
		}
	}

	log.Printf("   ✓ Inserted prices for %d markets at %s", totalUpdates, ts.Format("15:04:05"))

	// Mirror latest prices to runners table (SAME as liveprices.go)
	log.Println("   📋 Mirroring latest prices to runners table...")
	if err := mirrorLatestPrices(db, ts); err != nil {
		log.Printf("   ⚠️  Warning: Mirror to runners failed: %v", err)
	} else {
		log.Println("   ✓ Mirrored to runners table")
	}

	return totalUpdates, skipped, nil
}

// extractPrices calculates best back, lay, and VWAP from runner book
// Copied from internal/services/liveprices.go
func extractPrices(runner betfair.RunnerBook) (backPrice, layPrice, vwap float64) {
	if runner.EX == nil {
		return 0, 0, 0
	}

	// Best available back price (highest price punters can back at)
	if len(runner.EX.AvailableToBack) > 0 {
		backPrice = runner.EX.AvailableToBack[0].Price
	}

	// Best available lay price (lowest price punters can lay at)
	if len(runner.EX.AvailableToLay) > 0 {
		layPrice = runner.EX.AvailableToLay[0].Price
	}

	// VWAP = volume-weighted average from traded volume
	if len(runner.EX.TradedVolume) > 0 {
		var totalVol, weightedSum float64
		for _, tv := range runner.EX.TradedVolume {
			totalVol += tv.Size
			weightedSum += tv.Price * tv.Size
		}
		if totalVol > 0 {
			vwap = weightedSum / totalVol
		}
	}

	// Fallback: if no traded volume, use mid-point of back/lay
	if vwap == 0 && backPrice > 0 && layPrice > 0 {
		vwap = (backPrice + layPrice) / 2
	}

	return
}

// mirrorLatestPrices copies latest live prices to runners table
// Copied from internal/services/liveprices.go
func mirrorLatestPrices(db *sql.DB, ts time.Time) error {
	fiveMinutesAgo := ts.Add(-5 * time.Minute)

	_, err := db.Exec(`
		WITH latest AS (
			SELECT DISTINCT ON (lp.runner_id)
				lp.runner_id,
				lp.vwap,
				lp.back_price,
				lp.lay_price
			FROM racing.live_prices lp
			JOIN racing.runners run ON run.runner_id = lp.runner_id
			JOIN racing.races r ON r.race_id = run.race_id
			WHERE lp.ts >= $1
			ORDER BY lp.runner_id, lp.ts DESC
		)
		UPDATE racing.runners run
		SET 
			win_ppwap = COALESCE(latest.vwap, run.win_ppwap),
			win_ppmax = GREATEST(COALESCE(latest.back_price, 0), COALESCE(run.win_ppmax, 0)),
			win_ppmin = LEAST(
				CASE WHEN latest.lay_price > 0 THEN latest.lay_price ELSE 9999 END,
				CASE WHEN run.win_ppmin > 0 THEN run.win_ppmin ELSE 9999 END
			)
		FROM latest
		WHERE run.runner_id = latest.runner_id
	`, fiveMinutesAgo)

	return err
}

// Helper functions
// Must match the format used in internal/betfair/matcher.go generateRaceKeyHelper()
func generateRaceKey(race scraper.Race) string {
	normCourse := strings.ToLower(strings.TrimSpace(race.Course))
	normTime := race.OffTime
	normName := strings.ToLower(strings.TrimSpace(race.RaceName))
	normType := strings.ToLower(strings.TrimSpace(race.Type))
	normRegion := strings.ToUpper(strings.TrimSpace(race.Region))

	return fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
}

func nullFloat(f float64) interface{} {
	if f == 0 {
		return nil
	}
	return f
}
