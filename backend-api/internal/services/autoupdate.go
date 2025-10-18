package services

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"giddyup/api/internal/betfair"
	"giddyup/api/internal/scraper"

	"github.com/jmoiron/sqlx"
)

// AutoUpdateService handles automatic data updates on startup
type AutoUpdateService struct {
	db      *sqlx.DB
	enabled bool
	dataDir string
}

// NewAutoUpdateService creates a new auto-update service
func NewAutoUpdateService(db *sqlx.DB, enabled bool, dataDir string) *AutoUpdateService {
	return &AutoUpdateService{
		db:      db,
		enabled: enabled,
		dataDir: dataDir,
	}
}

// RunInBackground starts the auto-update in a goroutine (non-blocking)
func (s *AutoUpdateService) RunInBackground() {
	if !s.enabled {
		log.Println("[AutoUpdate] Disabled - skipping automatic updates")
		return
	}

	// Run in background goroutine so server starts immediately
	go func() {
		// Give server a few seconds to fully start
		time.Sleep(5 * time.Second)

		log.Println("[AutoUpdate] 🔍 Checking for missing data...")

		// ALWAYS fetch today and tomorrow FIRST (critical for live racing)
		log.Println("[AutoUpdate] 📅 Fetching today/tomorrow (always on startup)...")
		s.handleTodaysRaces()

		// Then backfill any missing historical dates
		lastDate, err := s.getLastDateInDatabase()
		if err != nil {
			log.Printf("[AutoUpdate] ❌ Failed to get last date: %v", err)
			return
		}

		// Calculate dates to backfill (from last_date+1 to yesterday)
		yesterday := time.Now().AddDate(0, 0, -1)

		if lastDate.After(yesterday) || lastDate.Equal(yesterday) {
			log.Printf("[AutoUpdate] ✅ Historical data is up to date (last: %s, yesterday: %s)",
				lastDate.Format("2006-01-02"), yesterday.Format("2006-01-02"))
			return
		}

		startDate := lastDate.AddDate(0, 0, 1) // Start from day after last date
		daysToBackfill := int(yesterday.Sub(startDate).Hours()/24) + 1

		log.Printf("[AutoUpdate] 📅 Backfilling %d days (%s to %s)...",
			daysToBackfill, startDate.Format("2006-01-02"), yesterday.Format("2006-01-02"))

		// Backfill each date
		successCount := 0
		failureCount := 0

		for date := startDate; !date.After(yesterday); date = date.AddDate(0, 0, 1) {
			dateStr := date.Format("2006-01-02")
			log.Printf("[AutoUpdate] Processing %s...", dateStr)

			races, runners, err := s.backfillDate(dateStr)
			if err != nil {
				log.Printf("[AutoUpdate] ❌ Failed %s: %v", dateStr, err)
				failureCount++
				continue
			}

			log.Printf("[AutoUpdate] ✅ %s: %d races, %d runners", dateStr, races, runners)
			successCount++

			// Rate limiting: pause between dates (15-30s)
			if !date.Equal(yesterday) {
				pauseDuration := time.Duration(15+time.Now().Unix()%15) * time.Second
				log.Printf("[AutoUpdate] ⏸️  Pausing %v before next date...", pauseDuration)
				time.Sleep(pauseDuration)
			}
		}

		log.Printf("[AutoUpdate] 🎉 Backfill complete! Success: %d, Failed: %d", successCount, failureCount)
	}()
}

// handleTodaysRaces fetches today's racecards and optionally starts live price updates
func (s *AutoUpdateService) handleTodaysRaces() {
	// Run 24/7 - supports global racing (US races happen late UK time)
	log.Printf("[AutoUpdate] Running at %s", time.Now().Format("15:04:05 MST"))

	// Fetch TODAY and TOMORROW in parallel for speed
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")

	log.Printf("[AutoUpdate] 📅 Fetching today/tomorrow in parallel...")

	var races, tomorrowRaces int
	var wg sync.WaitGroup
	wg.Add(2)

	// TODAY goroutine
	go func() {
		defer wg.Done()
		log.Printf("[AutoUpdate] 📅 [Thread 1] Fetching TODAY (%s) [FORCE REFRESH]...", today)
		r, rr, err := s.backfillRacecards(today, true)
		if err != nil {
			log.Printf("[AutoUpdate] ❌ [Thread 1] Failed to fetch today: %v", err)
		} else {
			races = r
			log.Printf("[AutoUpdate] ✅ [Thread 1] TODAY loaded: %d races, %d runners", r, rr)
		}
	}()

	// TOMORROW goroutine
	go func() {
		defer wg.Done()
		log.Printf("[AutoUpdate] 📅 [Thread 2] Fetching TOMORROW (%s) [FORCE REFRESH]...", tomorrow)
		r, rr, err := s.backfillRacecards(tomorrow, true)
		if err != nil {
			log.Printf("[AutoUpdate] ⚠️  [Thread 2] Failed to fetch tomorrow: %v", err)
		} else {
			tomorrowRaces = r
			log.Printf("[AutoUpdate] ✅ [Thread 2] TOMORROW loaded: %d races, %d runners", r, rr)
		}
	}()

	wg.Wait()
	log.Printf("[AutoUpdate] ✅ Parallel load complete: TODAY (%d races) + TOMORROW (%d races)", races, tomorrowRaces)

	// Start live prices if enabled
	enableLivePrices := os.Getenv("ENABLE_LIVE_PRICES") == "true"
	if !enableLivePrices {
		log.Println("[AutoUpdate] Live prices disabled")
		return
	}

	if races > 0 {
		log.Println("[AutoUpdate] 🔴 Starting live prices for TODAY...")
		if err := s.startLivePrices(today); err != nil {
			log.Printf("[AutoUpdate] ❌ Failed to start live prices for today: %v", err)
		} else {
			log.Println("[AutoUpdate] ✅ Live prices running for today")
		}
	}

	if tomorrowRaces > 0 {
		log.Println("[AutoUpdate] 🔴 Starting live prices for TOMORROW...")
		if err := s.startLivePrices(tomorrow); err != nil {
			log.Printf("[AutoUpdate] ⚠️  Failed to start live prices for tomorrow: %v", err)
		} else {
			log.Println("[AutoUpdate] ✅ Live prices running for tomorrow")
		}
	}
}

// backfillRacecards fetches and inserts racecards (preliminary data) - FORCE REFRESH for today/tomorrow
func (s *AutoUpdateService) backfillRacecards(dateStr string, forceRefresh bool) (int, int, error) {
	// DON'T delete - just upsert to preserve prices set by live updater
	// The ON CONFLICT clauses in insertToDatabase will update race metadata without wiping prices
	if forceRefresh {
		log.Printf("[AutoUpdate]   🔄 Upserting racecards for %s (preserving prices)...", dateStr)
	}

	// ALWAYS use Sporting Life API V2 - works for all dates!
	log.Printf("[AutoUpdate]   [1/2] Fetching racecards from Sporting Life API for %s...", dateStr)
	slScraper := scraper.NewSportingLifeAPIV2()
	rpRaces, err := slScraper.GetRacesForDate(dateStr)
	if err != nil {
		return 0, 0, fmt.Errorf("Sporting Life API failed: %w", err)
	}

	log.Printf("[AutoUpdate]   ✓ Got %d UK/IRE races from Sporting Life", len(rpRaces))

	// Insert to database (without Betfair prices initially)
	log.Printf("[AutoUpdate]   [2/2] Inserting %d races to database (prelim=true)...", len(rpRaces))
	races, runners, err := s.insertToDatabase(dateStr, rpRaces, true) // true = prelim
	if err != nil {
		return 0, 0, fmt.Errorf("insert failed: %w", err)
	}
	log.Printf("[AutoUpdate]   ✓ Inserted %d races, %d runners (preliminary)", races, runners)

	return races, runners, nil
}

// startLivePrices discovers Betfair markets and starts the live price updater
func (s *AutoUpdateService) startLivePrices(dateStr string) error {
	// Get Betfair credentials
	appKey := os.Getenv("BETFAIR_APP_KEY")
	sessionToken := os.Getenv("BETFAIR_SESSION_TOKEN")
	username := os.Getenv("BETFAIR_USERNAME")
	password := os.Getenv("BETFAIR_PASSWORD")

	if appKey == "" {
		return fmt.Errorf("BETFAIR_APP_KEY not set")
	}

	// Authenticate if needed
	if sessionToken == "" {
		if username == "" || password == "" {
			return fmt.Errorf("BETFAIR_SESSION_TOKEN or credentials not set")
		}

		log.Println("[AutoUpdate] Authenticating with Betfair...")
		auth := betfair.NewAuthenticator(appKey, username, password)
		var err error
		sessionToken, err = auth.Login()
		if err != nil {
			return fmt.Errorf("betfair login failed: %w", err)
		}
		log.Println("[AutoUpdate] ✅ Authenticated with Betfair")
	}

	// Create Betfair client
	bfClient := betfair.NewClient(appKey, sessionToken)

	// Discover markets
	log.Println("[AutoUpdate] Discovering Betfair markets...")
	matcher := betfair.NewMatcher(bfClient)

	ctx := context.Background()
	markets, err := matcher.FindTodaysMarkets(ctx, dateStr)
	if err != nil {
		return fmt.Errorf("find markets failed: %w", err)
	}

	if len(markets) == 0 {
		log.Println("[AutoUpdate] No Betfair markets found for today")
		return nil
	}

	log.Printf("[AutoUpdate] Found %d Betfair markets", len(markets))

	// Load races from database to get race IDs and runner IDs
	rpRaces, raceIDMap, err := s.loadRacesFromDB(dateStr)
	if err != nil {
		return fmt.Errorf("load races from DB failed: %w", err)
	}

	// Match races to markets
	log.Println("[AutoUpdate] Matching Sporting Life ↔ Betfair...")
	mappings := matcher.MatchRacesToMarkets(rpRaces, markets, raceIDMap)

	if len(mappings) == 0 {
		log.Println("[AutoUpdate] Warning: No races matched with Betfair markets")
		return nil
	}

	log.Printf("[AutoUpdate] Matched %d races with Betfair markets", len(mappings))

	// Get update interval
	intervalSecs := 60 // default
	if envInterval := os.Getenv("LIVE_PRICE_INTERVAL"); envInterval != "" {
		if parsed, err := strconv.Atoi(envInterval); err == nil && parsed > 0 {
			intervalSecs = parsed
		}
	}

	// Start live prices service (pass credentials for fresh login each cycle)
	log.Printf("[AutoUpdate] DEBUG: Passing credentials to LivePrices - appKey: %s, username: %s, password: %s",
		appKey, username, maskPassword(password))
	livePrices := NewLivePricesService(s.db, appKey, username, password, time.Duration(intervalSecs)*time.Second)
	livePrices.SetMarketMappings(mappings)

	// Run in background goroutine
	go func() {
		ctx := context.Background()
		if err := livePrices.Run(ctx); err != nil && err != context.Canceled {
			log.Printf("[AutoUpdate] Live prices service error: %v", err)
		}
	}()

	return nil
}

// getLastDateInDatabase finds the most recent race date
func (s *AutoUpdateService) getLastDateInDatabase() (time.Time, error) {
	var lastDate time.Time
	query := `SELECT MAX(race_date) FROM racing.races`

	err := s.db.QueryRow(query).Scan(&lastDate)
	if err != nil {
		// If no races exist, start from a default date
		if err == sql.ErrNoRows {
			return time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), nil
		}
		return time.Time{}, err
	}

	return lastDate, nil
}

// backfillDate runs the full pipeline for a single date (using Sporting Life + Betfair)
func (s *AutoUpdateService) backfillDate(dateStr string) (int, int, error) {
	// Use Sporting Life API for historical dates
	log.Printf("[AutoUpdate]   [1/4] Scraping Sporting Life for %s...", dateStr)
	slScraper := scraper.NewSportingLifeAPIV2()
	rpRaces, err := slScraper.GetRacesForDate(dateStr)
	if err != nil {
		return 0, 0, fmt.Errorf("scrape failed: %w", err)
	}
	log.Printf("[AutoUpdate]   ✓ Got %d races from Sporting Life", len(rpRaces))

	// Step 2: Fetch and stitch Betfair data
	log.Printf("[AutoUpdate]   [2/4] Fetching Betfair data...")
	bfStitcher := scraper.NewBetfairStitcher(s.dataDir)
	bfStitcher.StitchBetfairForDate(dateStr, "uk")
	bfStitcher.StitchBetfairForDate(dateStr, "ire")

	bfUK, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "uk")
	bfIRE, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "ire")
	log.Printf("[AutoUpdate]   ✓ Got %d Betfair races (UK: %d, IRE: %d)", len(bfUK)+len(bfIRE), len(bfUK), len(bfIRE))

	// Step 3: Match and merge (using shared course+time logic)
	log.Printf("[AutoUpdate]   [3/4] Matching Sporting Life with Betfair data...")
	mergedRaces := scraper.MatchAndMerge(rpRaces, append(bfUK, bfIRE...))
	log.Printf("[AutoUpdate]   ✓ Merged %d races", len(mergedRaces))

	// Step 4: Insert to database
	log.Printf("[AutoUpdate]   [4/4] Inserting to database...")
	races, runners, err := s.insertToDatabase(dateStr, mergedRaces, false) // false = not prelim
	if err != nil {
		return 0, 0, fmt.Errorf("database insert failed: %w", err)
	}
	log.Printf("[AutoUpdate]   ✓ Inserted %d races, %d runners", races, runners)

	return races, runners, nil
}

// matchAndMerge moved to shared scraper.MatchAndMerge() in internal/scraper/matcher.go

// insertToDatabase inserts races and runners into the database
func (s *AutoUpdateService) insertToDatabase(dateStr string, races []scraper.Race, prelim bool) (int, int, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// STEP 1 & 2: Batch upsert dimensions and get IDs back (optimized!)
	log.Printf("[AutoUpdate]      📊 Upserting dimension tables (courses, horses, jockeys, trainers, owners)...")

	// Set performance knobs for this transaction
	tx.Exec(`SET LOCAL synchronous_commit = off`) // Safe for batch ETL
	tx.Exec(`SET LOCAL statement_timeout = 0`)

	courseIDs, horseIDs, trainerIDs, jockeyIDs, ownerIDs, err := s.batchUpsertDimensions(tx, races)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to batch upsert dimensions: %w", err)
	}

	log.Printf("[AutoUpdate]      ✓ Upserted %d courses, %d horses, %d trainers, %d jockeys, %d owners",
		len(courseIDs), len(horseIDs), len(trainerIDs), len(jockeyIDs), len(ownerIDs))

	// Populate foreign keys using the returned ID maps (no extra queries!)
	s.populateForeignKeysFromMaps(courseIDs, horseIDs, trainerIDs, jockeyIDs, ownerIDs, races)

	raceCount := 0
	runnerCount := 0

	progressInterval := 5 // Log every 5 races
	for raceIdx, race := range races {
		if raceIdx > 0 && raceIdx%progressInterval == 0 {
			log.Printf("[AutoUpdate]      📝 Progress: %d/%d races (%.0f%%), %d runners so far",
				raceIdx, len(races), float64(raceIdx)/float64(len(races))*100, runnerCount)
		}

		// Generate race_key
		raceKey := generateRaceKey(race)

		// Insert race with prelim flag
		var raceID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO racing.races (
				race_key, race_date, region, course_id, off_time,
				race_name, race_type, class, dist_raw, dist_f, dist_m,
				going, surface, ran, prelim
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9, $10, $11,
				$12, $13, $14, $15
			)
			ON CONFLICT (race_key, race_date) DO UPDATE SET
				race_name = EXCLUDED.race_name,
				ran = EXCLUDED.ran,
				prelim = EXCLUDED.prelim
			RETURNING race_id
		`, raceKey, race.Date, race.Region, nullInt64(race.CourseID), nullString(race.OffTime),
			race.RaceName, race.Type, nullString(race.Class), nullString(race.Distance),
			nullFloat64(race.DistanceF), nullInt(race.DistanceM),
			nullString(race.Going), nullString(race.Surface), race.Ran, prelim).Scan(&raceID)

		if err != nil {
			log.Printf("[AutoUpdate]      ❌ Failed to insert race %d (%s %s): %v", raceIdx, race.Course, race.OffTime, err)
			return 0, 0, fmt.Errorf("failed to insert race %s: %w", raceKey, err)
		}

		raceCount++

		// Insert runners and capture runner_ids
		for j, runner := range race.Runners {
			if j == 0 && raceIdx%10 == 0 {
				log.Printf("[AutoUpdate]         Inserting %d runners for race %d...", len(race.Runners), raceIdx)
			}
			runnerKey := generateRunnerKey(raceKey, runner)

			var runnerID int64
			err := tx.QueryRowContext(ctx, `
				INSERT INTO racing.runners (
					runner_key, race_id, race_date,
					horse_id, trainer_id, jockey_id, owner_id,
					num, pos_raw, draw, age, lbs, "or", rpr, comment,
					win_bsp, win_ppwap, place_bsp, place_ppwap,
					betfair_selection_id, best_odds, best_bookmaker
				) VALUES (
					$1, $2, $3,
					$4, $5, $6, $7,
					$8, $9, $10, $11, $12, $13, $14, $15,
					$16, $17, $18, $19,
					$20, $21, $22
				)
				ON CONFLICT (runner_key, race_date) DO UPDATE SET
					pos_raw = COALESCE(EXCLUDED.pos_raw, racing.runners.pos_raw),
					win_bsp = COALESCE(EXCLUDED.win_bsp, racing.runners.win_bsp),
					win_ppwap = COALESCE(EXCLUDED.win_ppwap, racing.runners.win_ppwap),
					place_bsp = COALESCE(EXCLUDED.place_bsp, racing.runners.place_bsp),
					place_ppwap = COALESCE(EXCLUDED.place_ppwap, racing.runners.place_ppwap),
					betfair_selection_id = COALESCE(EXCLUDED.betfair_selection_id, racing.runners.betfair_selection_id),
					best_odds = COALESCE(EXCLUDED.best_odds, racing.runners.best_odds),
					best_bookmaker = COALESCE(EXCLUDED.best_bookmaker, racing.runners.best_bookmaker)
				RETURNING runner_id
			`,
				runnerKey, raceID, race.Date,
				nullInt64(runner.HorseID), nullInt64(runner.TrainerID), nullInt64(runner.JockeyID), nullInt64(runner.OwnerID),
				nullInt(runner.Num), nullString(runner.Pos), nullInt(runner.Draw), nullInt(runner.Age),
				nullInt(runner.Lbs), nullInt(runner.OR), nullInt(runner.RPR), nullString(runner.Comment),
				nullFloat64BSP(runner.WinBSP), nullFloat64(runner.WinPPWAP),
				nullFloat64BSP(runner.PlaceBSP), nullFloat64(runner.PlacePPWAP),
				nullInt64(int(runner.BetfairSelectionID)), nullFloat64(runner.BestOdds), nullString(runner.BestBookmaker),
			).Scan(&runnerID)

			if err != nil {
				return 0, 0, fmt.Errorf("failed to insert runner %s: %w", runnerKey, err)
			}

			// Store runner_id back in the race data for Betfair matching
			race.Runners[j].RunnerID = int(runnerID)
			runnerCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return raceCount, runnerCount, nil
}

// Helper functions

func generateRaceKey(race scraper.Race) string {
	normCourse := strings.ToLower(strings.TrimSpace(race.Course))
	normTime := race.OffTime
	if len(normTime) >= 5 {
		normTime = normTime[:5] // Strip seconds: "12:35:00" → "12:35"
	}
	normName := strings.ToLower(strings.TrimSpace(race.RaceName))
	normType := strings.ToLower(strings.TrimSpace(race.Type))
	normRegion := strings.ToUpper(strings.TrimSpace(race.Region))

	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func generateRunnerKey(raceKey string, runner scraper.Runner) string {
	normHorse := strings.ToLower(strings.TrimSpace(runner.Horse))
	data := fmt.Sprintf("%s|%s|%d|%d", raceKey, normHorse, runner.Num, runner.Draw)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

// normalizeTime and parseFloat moved to internal/scraper/matcher.go (shared functions)

// loadRacesFromDB loads races from database with runner IDs populated
func (s *AutoUpdateService) loadRacesFromDB(dateStr string) ([]scraper.Race, map[string]int64, error) {
	rows, err := s.db.Query(`
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

		// Get or create race
		race, exists := racesMap[raceID]
		if !exists {
			// Extract just time part from off_time (PostgreSQL TIME returns "0000-01-01T15:04:05")
			cleanOffTime := offTime.String
			if strings.Contains(cleanOffTime, "T") {
				// Extract time after "T": "0000-01-01T15:04:05" → "15:04:05"
				parts := strings.Split(cleanOffTime, "T")
				if len(parts) == 2 {
					cleanOffTime = parts[1]
				}
			}

			race = &scraper.Race{
				RaceID:   int(raceID),
				Date:     raceDate,
				Region:   region,
				Course:   course,
				OffTime:  cleanOffTime,
				RaceName: raceName,
				Type:     raceType,
				Class:    class.String,
				Distance: distance.String,
				Going:    going.String,
				Surface:  surface.String,
				Ran:      ran,
				Runners:  []scraper.Runner{},
			}
			racesMap[raceID] = race

			// Generate race key for mapping
			raceKey := generateRaceKey(*race)
			raceIDMap[raceKey] = raceID
		}

		// Add runner if exists
		if runnerID.Valid && horse.Valid {
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
	var races []scraper.Race
	for _, race := range racesMap {
		races = append(races, *race)
	}

	return races, raceIDMap, nil
}

func nullString(s string) interface{} {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return s
}

func nullInt(i int) interface{} {
	if i == 0 {
		return nil
	}
	return i
}

func nullInt64(i int) interface{} {
	if i == 0 {
		return nil
	}
	return int64(i)
}

func maskPassword(password string) string {
	if len(password) == 0 {
		return "(empty)"
	}
	if len(password) <= 4 {
		return "****"
	}
	return password[:2] + "****" + password[len(password)-2:]
}

func nullFloat64(f float64) interface{} {
	if f == 0.0 {
		return nil
	}
	return f
}

func nullFloat64BSP(f float64) interface{} {
	if f < 1.01 {
		return nil
	}
	return f
}

// batchUpsertDimensions performs optimized batch upsert and returns ID maps
// Replaces ~5,000 individual queries with ~15 batch queries (300x faster!)
func (s *AutoUpdateService) batchUpsertDimensions(tx *sql.Tx, races []scraper.Race) (
	map[string]int64, map[string]int64, map[string]int64, map[string]int64, map[string]int64, error) {

	// Collect unique entities
	courses := make(map[string]string) // name -> region
	horseSet := make(map[string]struct{})
	trainerSet := make(map[string]struct{})
	jockeySet := make(map[string]struct{})
	ownerSet := make(map[string]struct{})

	for _, race := range races {
		if cn := strings.TrimSpace(race.Course); cn != "" {
			courses[cn] = race.Region
		}
		for _, runner := range race.Runners {
			if v := strings.TrimSpace(runner.Horse); v != "" {
				horseSet[v] = struct{}{}
			}
			if v := strings.TrimSpace(runner.Trainer); v != "" {
				trainerSet[v] = struct{}{}
			}
			if v := strings.TrimSpace(runner.Jockey); v != "" {
				jockeySet[v] = struct{}{}
			}
			if v := strings.TrimSpace(runner.Owner); v != "" {
				ownerSet[v] = struct{}{}
			}
		}
	}

	// Convert sets to slices
	toSlice := func(m map[string]struct{}) []string {
		out := make([]string, 0, len(m))
		for k := range m {
			out = append(out, k)
		}
		return out
	}

	// Batch upsert all entities (3 queries each instead of N*2)
	courseIDs, err := UpsertCoursesAndFetchIDs(tx, courses)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	horseIDs, err := UpsertNamesAndFetchIDs(tx, "horses", "horse_id", "horse_name", "horses_uniq", toSlice(horseSet))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	trainerIDs, err := UpsertNamesAndFetchIDs(tx, "trainers", "trainer_id", "trainer_name", "trainers_uniq", toSlice(trainerSet))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	jockeyIDs, err := UpsertNamesAndFetchIDs(tx, "jockeys", "jockey_id", "jockey_name", "jockeys_uniq", toSlice(jockeySet))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	ownerIDs, err := UpsertNamesAndFetchIDs(tx, "owners", "owner_id", "owner_name", "owners_uniq", toSlice(ownerSet))
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	return courseIDs, horseIDs, trainerIDs, jockeyIDs, ownerIDs, nil
}

// OLD upsertDimensions (kept for reference, will be removed)
func (s *AutoUpdateService) upsertDimensionsOld(tx *sql.Tx, races []scraper.Race) error {
	// Collect all unique names
	courses := make(map[string]string) // courseName -> region
	horses := make(map[string]bool)
	trainers := make(map[string]bool)
	jockeys := make(map[string]bool)
	owners := make(map[string]bool)

	for _, race := range races {
		// Collect course names
		if race.Course != "" {
			courses[strings.TrimSpace(race.Course)] = race.Region
		}

		for _, runner := range race.Runners {
			if runner.Horse != "" {
				horses[strings.TrimSpace(runner.Horse)] = true
			}
			if runner.Trainer != "" {
				trainers[strings.TrimSpace(runner.Trainer)] = true
			}
			if runner.Jockey != "" {
				jockeys[strings.TrimSpace(runner.Jockey)] = true
			}
			if runner.Owner != "" {
				owners[strings.TrimSpace(runner.Owner)] = true
			}
		}
	}

	// Upsert courses
	for courseName, region := range courses {
		_, err := tx.Exec(`
			INSERT INTO racing.courses (course_name, region)
			VALUES ($1, $2)
			ON CONFLICT ON CONSTRAINT courses_uniq DO NOTHING
		`, courseName, region)
		if err != nil {
			return fmt.Errorf("failed to upsert course %s: %w", courseName, err)
		}
	}

	// Upsert horses
	for horse := range horses {
		_, err := tx.Exec(`
			INSERT INTO racing.horses (horse_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT horses_uniq DO NOTHING
		`, horse)
		if err != nil {
			return fmt.Errorf("failed to upsert horse %s: %w", horse, err)
		}
	}

	// Upsert trainers
	for trainer := range trainers {
		_, err := tx.Exec(`
			INSERT INTO racing.trainers (trainer_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT trainers_uniq DO NOTHING
		`, trainer)
		if err != nil {
			return fmt.Errorf("failed to upsert trainer %s: %w", trainer, err)
		}
	}

	// Upsert jockeys
	for jockey := range jockeys {
		_, err := tx.Exec(`
			INSERT INTO racing.jockeys (jockey_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT jockeys_uniq DO NOTHING
		`, jockey)
		if err != nil {
			return fmt.Errorf("failed to upsert jockey %s: %w", jockey, err)
		}
	}

	// Upsert owners
	for owner := range owners {
		_, err := tx.Exec(`
			INSERT INTO racing.owners (owner_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT owners_uniq DO NOTHING
		`, owner)
		if err != nil {
			return fmt.Errorf("failed to upsert owner %s: %w", owner, err)
		}
	}

	log.Printf("[AutoUpdate]      ✓ Upserted %d courses, %d horses, %d trainers, %d jockeys, %d owners",
		len(courses), len(horses), len(trainers), len(jockeys), len(owners))

	return nil
}

// populateForeignKeysFromMaps populates foreign keys using pre-fetched ID maps (no queries!)
func (s *AutoUpdateService) populateForeignKeysFromMaps(
	courseIDs, horseIDs, trainerIDs, jockeyIDs, ownerIDs map[string]int64,
	races []scraper.Race) {

	for i := range races {
		// Populate course_id
		if id, ok := courseIDs[strings.TrimSpace(races[i].Course)]; ok {
			races[i].CourseID = int(id)
		}

		// Populate runner foreign keys
		for j := range races[i].Runners {
			runner := &races[i].Runners[j]

			if id, ok := horseIDs[strings.TrimSpace(runner.Horse)]; ok {
				runner.HorseID = int(id)
			}
			if id, ok := trainerIDs[strings.TrimSpace(runner.Trainer)]; ok {
				runner.TrainerID = int(id)
			}
			if id, ok := jockeyIDs[strings.TrimSpace(runner.Jockey)]; ok {
				runner.JockeyID = int(id)
			}
			if id, ok := ownerIDs[strings.TrimSpace(runner.Owner)]; ok {
				runner.OwnerID = int(id)
			}
		}
	}
}

// OLD populateForeignKeys (kept for reference, will be removed)
func (s *AutoUpdateService) populateForeignKeysOld(tx *sql.Tx, races []scraper.Race) error {
	// Create lookup maps
	courseIDs := make(map[string]int64)
	horseIDs := make(map[string]int64)
	trainerIDs := make(map[string]int64)
	jockeyIDs := make(map[string]int64)
	ownerIDs := make(map[string]int64)

	// Collect all unique names to look up
	courses := make(map[string]bool)
	horses := make(map[string]bool)
	trainers := make(map[string]bool)
	jockeys := make(map[string]bool)
	owners := make(map[string]bool)

	for _, race := range races {
		if race.Course != "" {
			courses[strings.TrimSpace(race.Course)] = true
		}

		for _, runner := range race.Runners {
			if runner.Horse != "" {
				horses[strings.TrimSpace(runner.Horse)] = true
			}
			if runner.Trainer != "" {
				trainers[strings.TrimSpace(runner.Trainer)] = true
			}
			if runner.Jockey != "" {
				jockeys[strings.TrimSpace(runner.Jockey)] = true
			}
			if runner.Owner != "" {
				owners[strings.TrimSpace(runner.Owner)] = true
			}
		}
	}

	// Look up course IDs
	for courseName := range courses {
		var id int64
		err := tx.QueryRow(`
			SELECT course_id FROM racing.courses
			WHERE racing.norm_text(course_name) = racing.norm_text($1)
		`, courseName).Scan(&id)
		if err == nil {
			courseIDs[strings.TrimSpace(courseName)] = id
		}
	}

	// Look up horse IDs
	for horse := range horses {
		var id int64
		err := tx.QueryRow(`
			SELECT horse_id FROM racing.horses
			WHERE racing.norm_text(horse_name) = racing.norm_text($1)
		`, horse).Scan(&id)
		if err == nil {
			horseIDs[strings.TrimSpace(horse)] = id
		}
	}

	// Look up trainer IDs
	for trainer := range trainers {
		var id int64
		err := tx.QueryRow(`
			SELECT trainer_id FROM racing.trainers
			WHERE racing.norm_text(trainer_name) = racing.norm_text($1)
		`, trainer).Scan(&id)
		if err == nil {
			trainerIDs[strings.TrimSpace(trainer)] = id
		}
	}

	// Look up jockey IDs
	for jockey := range jockeys {
		var id int64
		err := tx.QueryRow(`
			SELECT jockey_id FROM racing.jockeys
			WHERE racing.norm_text(jockey_name) = racing.norm_text($1)
		`, jockey).Scan(&id)
		if err == nil {
			jockeyIDs[strings.TrimSpace(jockey)] = id
		}
	}

	// Look up owner IDs
	for owner := range owners {
		var id int64
		err := tx.QueryRow(`
			SELECT owner_id FROM racing.owners
			WHERE racing.norm_text(owner_name) = racing.norm_text($1)
		`, owner).Scan(&id)
		if err == nil {
			ownerIDs[strings.TrimSpace(owner)] = id
		}
	}

	// Populate foreign keys in the race data
	for i := range races {
		// Populate course_id for the race
		if id, ok := courseIDs[strings.TrimSpace(races[i].Course)]; ok {
			races[i].CourseID = int(id)
		}

		for j := range races[i].Runners {
			runner := &races[i].Runners[j]

			if id, ok := horseIDs[strings.TrimSpace(runner.Horse)]; ok {
				runner.HorseID = int(id)
			}
			if id, ok := trainerIDs[strings.TrimSpace(runner.Trainer)]; ok {
				runner.TrainerID = int(id)
			}
			if id, ok := jockeyIDs[strings.TrimSpace(runner.Jockey)]; ok {
				runner.JockeyID = int(id)
			}
			if id, ok := ownerIDs[strings.TrimSpace(runner.Owner)]; ok {
				runner.OwnerID = int(id)
			}
		}
	}

	log.Printf("[AutoUpdate]      ✓ Populated foreign keys: %d courses, %d horses, %d trainers, %d jockeys, %d owners",
		len(courseIDs), len(horseIDs), len(trainerIDs), len(jockeyIDs), len(ownerIDs))

	return nil
}
