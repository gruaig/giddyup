package services

import (
	"context"
	"crypto/md5"
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

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

		// Find last date in database
		lastDate, err := s.getLastDateInDatabase()
		if err != nil {
			log.Printf("[AutoUpdate] ❌ Failed to get last date: %v", err)
			return
		}

		// Calculate dates to backfill (from last_date+1 to yesterday)
		yesterday := time.Now().AddDate(0, 0, -1)
		if lastDate.After(yesterday) || lastDate.Equal(yesterday) {
			log.Printf("[AutoUpdate] ✅ Database is up to date (last: %s, yesterday: %s)",
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

// backfillDate runs the full pipeline for a single date (using backfill_dates logic)
func (s *AutoUpdateService) backfillDate(dateStr string) (int, int, error) {
	// Step 1: Scrape Racing Post
	log.Printf("[AutoUpdate]   [1/4] Scraping Racing Post for %s...", dateStr)
	rpScraper := scraper.NewResultsScraper()
	rpRaces, err := rpScraper.ScrapeDate(dateStr)
	if err != nil {
		return 0, 0, fmt.Errorf("scrape failed: %w", err)
	}
	log.Printf("[AutoUpdate]   ✓ Got %d races from Racing Post", len(rpRaces))

	// Step 2: Fetch and stitch Betfair data
	log.Printf("[AutoUpdate]   [2/4] Fetching Betfair data...")
	bfStitcher := scraper.NewBetfairStitcher(s.dataDir)
	bfStitcher.StitchBetfairForDate(dateStr, "uk")
	bfStitcher.StitchBetfairForDate(dateStr, "ire")

	bfUK, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "uk")
	bfIRE, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "ire")
	log.Printf("[AutoUpdate]   ✓ Got %d Betfair races (UK: %d, IRE: %d)", len(bfUK)+len(bfIRE), len(bfUK), len(bfIRE))

	// Step 3: Match and merge (simple matching for now)
	log.Printf("[AutoUpdate]   [3/4] Matching Racing Post with Betfair data...")
	mergedRaces := s.matchAndMerge(rpRaces, append(bfUK, bfIRE...))
	log.Printf("[AutoUpdate]   ✓ Merged %d races", len(mergedRaces))

	// Step 4: Insert to database
	log.Printf("[AutoUpdate]   [4/4] Inserting to database...")
	races, runners, err := s.insertToDatabase(dateStr, mergedRaces)
	if err != nil {
		return 0, 0, fmt.Errorf("database insert failed: %w", err)
	}
	log.Printf("[AutoUpdate]   ✓ Inserted %d races, %d runners", races, runners)

	return races, runners, nil
}

// matchAndMerge matches Racing Post races with Betfair prices (simplified)
func (s *AutoUpdateService) matchAndMerge(rpRaces []scraper.Race, bfRaces []scraper.StitchedRace) []scraper.Race {
	// Build Betfair lookup map by (date, normalized race_name, off_time)
	bfMap := make(map[string]scraper.StitchedRace)
	for _, bfRace := range bfRaces {
		normName := scraper.NormalizeName(bfRace.EventName)
		normTime := normalizeTime(bfRace.OffTime)
		key := fmt.Sprintf("%s|%s|%s", bfRace.Date, normName, normTime)
		bfMap[key] = bfRace
	}

	// Match Racing Post races with Betfair
	matchedRaces := 0
	totalRunnerMatches := 0

	for i := range rpRaces {
		race := &rpRaces[i]
		normName := scraper.NormalizeName(race.RaceName)
		normTime := normalizeTime(race.OffTime)
		key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)

		bfRace, found := bfMap[key]
		if !found {
			continue
		}

		// Build runner lookup by normalized horse name
		bfRunnerMap := make(map[string]scraper.StitchedRunner)
		for _, bfRunner := range bfRace.Runners {
			normHorse := scraper.NormalizeName(bfRunner.Horse)
			bfRunnerMap[normHorse] = bfRunner
		}

		// Merge Betfair prices into Racing Post runners
		runnersMatched := 0
		for j := range race.Runners {
			runner := &race.Runners[j]
			normHorse := scraper.NormalizeName(runner.Horse)

			if bfRunner, found := bfRunnerMap[normHorse]; found {
				runner.WinBSP = parseFloat(bfRunner.WinBSP)
				runner.WinPPWAP = parseFloat(bfRunner.WinPPWAP)
				runner.PlaceBSP = parseFloat(bfRunner.PlaceBSP)
				runner.PlacePPWAP = parseFloat(bfRunner.PlacePPWAP)
				runnersMatched++
			}
		}

		if runnersMatched > 0 {
			matchedRaces++
			totalRunnerMatches += runnersMatched
			log.Printf("[AutoUpdate]     ✓ Matched %s @ %s: %d/%d runners with Betfair prices",
				race.Course, race.OffTime, runnersMatched, len(race.Runners))
		}
	}

	log.Printf("[AutoUpdate]   Summary: %d/%d races matched, %d total runners with Betfair prices",
		matchedRaces, len(rpRaces), totalRunnerMatches)

	return rpRaces
}

// insertToDatabase inserts races and runners into the database
func (s *AutoUpdateService) insertToDatabase(dateStr string, races []scraper.Race) (int, int, error) {
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// STEP 1: Upsert all dimension tables first (courses, horses, trainers, jockeys, owners)
	log.Printf("[AutoUpdate]      Upserting dimension tables...")
	if err := s.upsertDimensions(tx, races); err != nil {
		return 0, 0, fmt.Errorf("failed to upsert dimensions: %w", err)
	}

	// STEP 2: Look up IDs for all entities and populate foreign keys (including courses!)
	if err := s.populateForeignKeys(tx, races); err != nil {
		return 0, 0, fmt.Errorf("failed to populate foreign keys: %w", err)
	}

	raceCount := 0
	runnerCount := 0

	for _, race := range races {
		// Generate race_key
		raceKey := generateRaceKey(race)

		// Insert race
		var raceID int64
		err := tx.QueryRowContext(ctx, `
			INSERT INTO racing.races (
				race_key, race_date, region, course_id, off_time,
				race_name, race_type, class, dist_raw, dist_f, dist_m,
				going, surface, ran
			) VALUES (
				$1, $2, $3, $4, $5,
				$6, $7, $8, $9, $10, $11,
				$12, $13, $14
			)
			ON CONFLICT (race_key, race_date) DO UPDATE SET
				race_name = EXCLUDED.race_name,
				ran = EXCLUDED.ran
			RETURNING race_id
		`, raceKey, race.Date, race.Region, nullInt64(race.CourseID), nullString(race.OffTime),
			race.RaceName, race.Type, nullString(race.Class), nullString(race.Distance),
			nullFloat64(race.DistanceF), nullInt(race.DistanceM),
			nullString(race.Going), nullString(race.Surface), race.Ran).Scan(&raceID)

		if err != nil {
			return 0, 0, fmt.Errorf("failed to insert race %s: %w", raceKey, err)
		}

		raceCount++

		// Insert runners
		for _, runner := range race.Runners {
			runnerKey := generateRunnerKey(raceKey, runner)

			_, err := tx.ExecContext(ctx, `
				INSERT INTO racing.runners (
					runner_key, race_id, race_date,
					horse_id, trainer_id, jockey_id, owner_id,
					num, pos_raw, draw, age, lbs, "or", rpr, comment,
					win_bsp, win_ppwap, place_bsp, place_ppwap
				) VALUES (
					$1, $2, $3,
					$4, $5, $6, $7,
					$8, $9, $10, $11, $12, $13, $14, $15,
					$16, $17, $18, $19
				)
				ON CONFLICT (runner_key, race_date) DO UPDATE SET
					pos_raw = EXCLUDED.pos_raw,
					win_bsp = COALESCE(EXCLUDED.win_bsp, racing.runners.win_bsp),
					win_ppwap = COALESCE(EXCLUDED.win_ppwap, racing.runners.win_ppwap),
					place_bsp = COALESCE(EXCLUDED.place_bsp, racing.runners.place_bsp),
					place_ppwap = COALESCE(EXCLUDED.place_ppwap, racing.runners.place_ppwap)
			`,
				runnerKey, raceID, race.Date,
				nullInt64(runner.HorseID), nullInt64(runner.TrainerID), nullInt64(runner.JockeyID), nullInt64(runner.OwnerID),
				nullInt(runner.Num), nullString(runner.Pos), nullInt(runner.Draw), nullInt(runner.Age),
				nullInt(runner.Lbs), nullInt(runner.OR), nullInt(runner.RPR), nullString(runner.Comment),
				nullFloat64BSP(runner.WinBSP), nullFloat64(runner.WinPPWAP),
				nullFloat64BSP(runner.PlaceBSP), nullFloat64(runner.PlacePPWAP),
			)

			if err != nil {
				return 0, 0, fmt.Errorf("failed to insert runner %s: %w", runnerKey, err)
			}

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
	normTime := normalizeTime(race.OffTime)
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

func normalizeTime(t string) string {
	t = strings.TrimSpace(t)
	if len(t) == 5 && t[2] == ':' {
		return t
	}
	if len(t) == 4 && t[1] == ':' {
		return "0" + t
	}
	return t
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0
	}
	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return val
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

// upsertDimensions inserts/updates dimension tables (courses, horses, trainers, jockeys, owners)
func (s *AutoUpdateService) upsertDimensions(tx *sql.Tx, races []scraper.Race) error {
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

// populateForeignKeys looks up IDs and populates foreign key fields in the race data
func (s *AutoUpdateService) populateForeignKeys(tx *sql.Tx, races []scraper.Race) error {
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
