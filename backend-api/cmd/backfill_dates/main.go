package main

import (
	"context"
	"crypto/md5"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"giddyup/api/internal/scraper"

	"github.com/lib/pq"
)

var (
	sinceStr = flag.String("since", "", "Start date YYYY-MM-DD (required)")
	untilStr = flag.String("until", "", "End date YYYY-MM-DD (required)")
	dbConn   = flag.String("db", "host=localhost port=5432 dbname=horse_db user=postgres password=password sslmode=disable", "Database connection string")
	dataDir  = flag.String("data-dir", "/home/smonaghan/GiddyUp/data", "Data directory for cache/betfair")
	dryRun   = flag.Bool("dry-run", false, "Don't insert to database, just show what would be done")
	verbose  = flag.Bool("verbose", true, "Detailed logging")
)

func main() {
	flag.Parse()

	if *sinceStr == "" || *untilStr == "" {
		log.Fatal("Both -since and -until dates are required")
	}

	since, err := time.Parse("2006-01-02", *sinceStr)
	if err != nil {
		log.Fatalf("Invalid since date: %v", err)
	}

	until, err := time.Parse("2006-01-02", *untilStr)
	if err != nil {
		log.Fatalf("Invalid until date: %v", err)
	}

	if until.Before(since) {
		log.Fatal("until date must be after since date")
	}

	log.Printf("🚀 Backfill dates: %s to %s", since.Format("2006-01-02"), until.Format("2006-01-02"))
	log.Printf("   Data directory: %s", *dataDir)
	log.Printf("   Dry run: %v", *dryRun)

	// Open database connection
	var db *sql.DB
	if !*dryRun {
		db, err = sql.Open("postgres", *dbConn)
		if err != nil {
			log.Fatalf("Failed to open database: %v", err)
		}
		defer db.Close()

		// Set search path
		_, err = db.Exec("SET search_path TO racing, public")
		if err != nil {
			log.Fatalf("Failed to set search_path: %v", err)
		}

		log.Printf("✅ Connected to database")
	}

	// Initialize scrapers
	rpScraper := scraper.NewResultsScraper()
	bfStitcher := scraper.NewBetfairStitcher(*dataDir)

	// Process each date
	totalRaces := 0
	totalRunners := 0
	successDates := 0

	for date := since; !date.After(until); date = date.AddDate(0, 0, 1) {
		dateStr := date.Format("2006-01-02")
		log.Printf("\n📅 Processing date: %s", dateStr)

		races, runners, err := processDate(rpScraper, bfStitcher, db, dateStr)
		if err != nil {
			log.Printf("❌ Error processing %s: %v", dateStr, err)
			continue
		}

		totalRaces += races
		totalRunners += runners
		successDates++

		// Pause between dates to avoid pattern detection (15-30s)
		if !date.Equal(until) {
			pauseDuration := time.Duration(15+rand.Intn(15)) * time.Second
			log.Printf("⏸️  Pausing %v before next date to avoid rate limiting...", pauseDuration)
			time.Sleep(pauseDuration)
		}
	}

	log.Printf("\n✅ Backfill complete!")
	log.Printf("   Dates processed: %d/%d", successDates, int(until.Sub(since).Hours()/24)+1)
	log.Printf("   Total races: %d", totalRaces)
	log.Printf("   Total runners: %d", totalRunners)
}

// processDate handles scraping and insertion for a single date
func processDate(rpScraper *scraper.ResultsScraper, bfStitcher *scraper.BetfairStitcher, db *sql.DB, dateStr string) (int, int, error) {
	// Step 1: Scrape Racing Post
	log.Printf("  [1/4] Scraping Racing Post for %s...", dateStr)
	rpRaces, err := rpScraper.ScrapeDate(dateStr)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to scrape Racing Post: %w", err)
	}
	log.Printf("  ✓ Got %d races from Racing Post", len(rpRaces))

	// Step 2: Fetch and stitch Betfair data
	log.Printf("  [2/4] Fetching Betfair data...")
	err = bfStitcher.StitchBetfairForDate(dateStr, "uk") // GB races
	if err != nil {
		log.Printf("  ⚠️  Betfair UK: %v (continuing without Betfair data)", err)
	}

	err = bfStitcher.StitchBetfairForDate(dateStr, "ire") // IRE races
	if err != nil {
		log.Printf("  ⚠️  Betfair IRE: %v (continuing without Betfair data)", err)
	}

	// Load stitched Betfair data
	bfUK, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "uk")
	bfIRE, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "ire")
	log.Printf("  ✓ Got %d Betfair races (UK: %d, IRE: %d)", len(bfUK)+len(bfIRE), len(bfUK), len(bfIRE))

	// Step 3: Match and merge
	log.Printf("  [3/4] Matching Racing Post with Betfair data...")
	mergedRaces := matchAndMerge(rpRaces, append(bfUK, bfIRE...))
	log.Printf("  ✓ Merged %d races", len(mergedRaces))

	// Step 4: Insert to database
	if *dryRun {
		log.Printf("  [4/4] DRY RUN - would insert %d races", len(mergedRaces))
		runnerCount := 0
		for _, r := range mergedRaces {
			runnerCount += len(r.Runners)
		}
		return len(mergedRaces), runnerCount, nil
	}

	log.Printf("  [4/4] Inserting to database...")
	races, runners, err := insertToDatabase(db, dateStr, mergedRaces)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to insert to database: %w", err)
	}
	log.Printf("  ✅ Inserted %d races, %d runners", races, runners)

	return races, runners, nil
}

// matchAndMerge matches Racing Post races with Betfair prices
func matchAndMerge(rpRaces []scraper.Race, bfRaces []scraper.StitchedRace) []scraper.Race {
	// Build Betfair lookup map by (date, normalized race_name, off_time)
	// Since Betfair event names don't have course names, we match on race name + time only
	bfMap := make(map[string]scraper.StitchedRace)
	for _, bfRace := range bfRaces {
		// Normalize race name and time for matching
		normName := scraper.NormalizeName(bfRace.EventName)
		normTime := normalizeTime(bfRace.OffTime)
		key := fmt.Sprintf("%s|%s|%s", bfRace.Date, normName, normTime)
		bfMap[key] = bfRace
		if *verbose {
			log.Printf("    [BF] %s → key: %s", bfRace.EventName, key)
		}
	}

	// Match Racing Post races with Betfair
	matched := 0
	for i := range rpRaces {
		race := &rpRaces[i]
		normName := scraper.NormalizeName(race.RaceName)
		normTime := normalizeTime(race.OffTime)
		key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)

		if *verbose {
			log.Printf("    [RP] %s @ %s → key: %s", race.RaceName, race.OffTime, key)
		}

		bfRace, found := bfMap[key]
		if !found {
			// Try without normalizing (direct match)
			key2 := fmt.Sprintf("%s|%s|%s", race.Date, strings.ToLower(strings.TrimSpace(race.RaceName)), normTime)
			bfRace, found = bfMap[key2]
			if !found {
				if *verbose {
					log.Printf("    [RP] No match for: %s", key)
				}
				continue
			}
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
				// Merge Betfair prices
				runner.WinBSP = parseFloat(bfRunner.WinBSP)
				runner.WinPPWAP = parseFloat(bfRunner.WinPPWAP)
				runner.WinMorningWAP = parseFloat(bfRunner.WinMorningWAP)
				runner.WinPPMax = parseFloat(bfRunner.WinPPMax)
				runner.WinPPMin = parseFloat(bfRunner.WinPPMin)
				runner.WinIPMax = parseFloat(bfRunner.WinIPMax)
				runner.WinIPMin = parseFloat(bfRunner.WinIPMin)
				runner.WinMorningVol = parseFloat(bfRunner.WinMorningVol)
				runner.WinPreVol = parseFloat(bfRunner.WinPreVol)
				runner.WinIPVol = parseFloat(bfRunner.WinIPVol)
				runner.PlaceBSP = parseFloat(bfRunner.PlaceBSP)
				runner.PlacePPWAP = parseFloat(bfRunner.PlacePPWAP)
				runner.PlaceMorningWAP = parseFloat(bfRunner.PlaceMorningWAP)
				runner.PlacePPMax = parseFloat(bfRunner.PlacePPMax)
				runner.PlacePPMin = parseFloat(bfRunner.PlacePPMin)
				runner.PlaceIPMax = parseFloat(bfRunner.PlaceIPMax)
				runner.PlaceIPMin = parseFloat(bfRunner.PlaceIPMin)
				runner.PlaceMorningVol = parseFloat(bfRunner.PlaceMorningVol)
				runner.PlacePreVol = parseFloat(bfRunner.PlacePreVol)
				runner.PlaceIPVol = parseFloat(bfRunner.PlaceIPVol)
				runnersMatched++
			}
		}

		if runnersMatched > 0 {
			matched++
			if *verbose {
				log.Printf("    ✓ Matched race %s @ %s (%d/%d runners with prices)", race.Course, race.OffTime, runnersMatched, len(race.Runners))
			}
		}
	}

	if *verbose {
		log.Printf("    Matched %d/%d races with Betfair data", matched, len(rpRaces))
	}

	return rpRaces
}

// insertToDatabase inserts races and runners into the database
func insertToDatabase(db *sql.DB, dateStr string, races []scraper.Race) (int, int, error) {
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// STEP 1: Upsert dimension tables and populate foreign keys
	log.Println("  🔄 Upserting dimension tables...")
	if err := upsertDimensions(tx, races); err != nil {
		return 0, 0, fmt.Errorf("failed to upsert dimensions: %w", err)
	}

	if err := populateForeignKeys(tx, races); err != nil {
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
			race.RaceName, race.Type, nullString(race.Class), nullString(race.Distance), nullFloat64(race.DistanceF), nullInt(race.DistanceM),
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
					win_bsp, win_ppwap, win_morningwap, win_ppmax, win_ppmin,
					win_ipmax, win_ipmin, win_morning_vol, win_pre_vol, win_ip_vol,
					place_bsp, place_ppwap, place_morningwap, place_ppmax, place_ppmin,
					place_ipmax, place_ipmin, place_morning_vol, place_pre_vol, place_ip_vol
				) VALUES (
					$1, $2, $3,
					$4, $5, $6, $7,
					$8, $9, $10, $11, $12, $13, $14, $15,
					$16, $17, $18, $19, $20,
					$21, $22, $23, $24, $25,
					$26, $27, $28, $29, $30,
					$31, $32, $33, $34, $35
				)
				ON CONFLICT (runner_key, race_date) DO UPDATE SET
					pos_raw = EXCLUDED.pos_raw,
					win_bsp = COALESCE(EXCLUDED.win_bsp, runners.win_bsp),
					win_ppwap = COALESCE(EXCLUDED.win_ppwap, runners.win_ppwap),
					place_bsp = COALESCE(EXCLUDED.place_bsp, runners.place_bsp),
					place_ppwap = COALESCE(EXCLUDED.place_ppwap, runners.place_ppwap)
			`,
				runnerKey, raceID, race.Date,
				nullInt64(runner.HorseID), nullInt64(runner.TrainerID), nullInt64(runner.JockeyID), nullInt64(runner.OwnerID),
				nullInt(runner.Num), nullString(runner.Pos), nullInt(runner.Draw), nullInt(runner.Age), nullInt(runner.Lbs), nullInt(runner.OR), nullInt(runner.RPR), nullString(runner.Comment),
				nullFloat64BSP(runner.WinBSP), nullFloat64(runner.WinPPWAP), nullFloat64(runner.WinMorningWAP), nullFloat64(runner.WinPPMax), nullFloat64(runner.WinPPMin),
				nullFloat64(runner.WinIPMax), nullFloat64(runner.WinIPMin), nullFloat64(runner.WinMorningVol), nullFloat64(runner.WinPreVol), nullFloat64(runner.WinIPVol),
				nullFloat64BSP(runner.PlaceBSP), nullFloat64(runner.PlacePPWAP), nullFloat64(runner.PlaceMorningWAP), nullFloat64(runner.PlacePPMax), nullFloat64(runner.PlacePPMin),
				nullFloat64(runner.PlaceIPMax), nullFloat64(runner.PlaceIPMin), nullFloat64(runner.PlaceMorningVol), nullFloat64(runner.PlacePreVol), nullFloat64(runner.PlaceIPVol),
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
	// Normalize components
	normCourse := strings.ToLower(strings.TrimSpace(race.Course))
	normTime := normalizeTime(race.OffTime)
	normName := strings.ToLower(strings.TrimSpace(race.RaceName))
	normType := strings.ToLower(strings.TrimSpace(race.Type))
	normRegion := strings.ToUpper(strings.TrimSpace(race.Region))

	// Generate MD5 hash
	data := fmt.Sprintf("%s|%s|%s|%s|%s|%s", race.Date, normRegion, normCourse, normTime, normName, normType)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func generateRunnerKey(raceKey string, runner scraper.Runner) string {
	normHorse := strings.ToLower(strings.TrimSpace(runner.Horse))
	num := runner.Num
	draw := runner.Draw

	data := fmt.Sprintf("%s|%s|%d|%d", raceKey, normHorse, num, draw)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("%x", hash)
}

func normalizeTime(t string) string {
	// Ensure HH:MM format
	t = strings.TrimSpace(t)
	if len(t) == 5 && t[2] == ':' {
		return t
	}
	if len(t) == 4 && t[1] == ':' {
		return "0" + t
	}
	return t
}

func extractCourseName(eventName string) string {
	// Event name format: "Course 3m Hcap Chs" -> extract "Course"
	parts := strings.Fields(eventName)
	if len(parts) > 0 {
		return parts[0]
	}
	return eventName
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

// SQL helper functions

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
	// Betfair BSP must be >= 1.01
	if f < 1.01 {
		return nil
	}
	return f
}

// upsertDimensions inserts/updates dimension tables (courses, horses, trainers, jockeys, owners)
func upsertDimensions(tx *sql.Tx, races []scraper.Race) error {
	// Collect all unique names
	courses := make(map[string]string) // courseName -> region
	horses := make(map[string]bool)
	trainers := make(map[string]bool)
	jockeys := make(map[string]bool)
	owners := make(map[string]bool)

	for _, race := range races {
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

	log.Printf("    ✓ Upserted %d courses, %d horses, %d trainers, %d jockeys, %d owners",
		len(courses), len(horses), len(trainers), len(jockeys), len(owners))

	return nil
}

// populateForeignKeys looks up IDs and populates foreign key fields in the race data
func populateForeignKeys(tx *sql.Tx, races []scraper.Race) error {
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

	// ✅ OPTIMIZED: Batch lookup using ANY($1) - 4 queries instead of 900+

	// Look up course IDs (batch)
	if len(courses) > 0 {
		courseList := make([]string, 0, len(courses))
		for courseName := range courses {
			courseList = append(courseList, courseName)
		}
		rows, err := tx.Query(`
			SELECT course_id, course_name 
			FROM racing.courses 
			WHERE course_name = ANY($1)
		`, pq.Array(courseList))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				courseIDs[strings.TrimSpace(name)] = id
			}
		}
	}

	// Look up horse IDs (batch)
	if len(horses) > 0 {
		horseList := make([]string, 0, len(horses))
		for horse := range horses {
			horseList = append(horseList, horse)
		}
		rows, err := tx.Query(`
			SELECT horse_id, horse_name 
			FROM racing.horses 
			WHERE horse_name = ANY($1)
		`, pq.Array(horseList))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				horseIDs[strings.TrimSpace(name)] = id
			}
		}
	}

	// Look up trainer IDs (batch)
	if len(trainers) > 0 {
		trainerList := make([]string, 0, len(trainers))
		for trainer := range trainers {
			trainerList = append(trainerList, trainer)
		}
		rows, err := tx.Query(`
			SELECT trainer_id, trainer_name 
			FROM racing.trainers 
			WHERE trainer_name = ANY($1)
		`, pq.Array(trainerList))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				trainerIDs[strings.TrimSpace(name)] = id
			}
		}
	}

	// Look up jockey IDs (batch)
	if len(jockeys) > 0 {
		jockeyList := make([]string, 0, len(jockeys))
		for jockey := range jockeys {
			jockeyList = append(jockeyList, jockey)
		}
		rows, err := tx.Query(`
			SELECT jockey_id, jockey_name 
			FROM racing.jockeys 
			WHERE jockey_name = ANY($1)
		`, pq.Array(jockeyList))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				jockeyIDs[strings.TrimSpace(name)] = id
			}
		}
	}

	// Look up owner IDs (batch)
	if len(owners) > 0 {
		ownerList := make([]string, 0, len(owners))
		for owner := range owners {
			ownerList = append(ownerList, owner)
		}
		rows, err := tx.Query(`
			SELECT owner_id, owner_name 
			FROM racing.owners 
			WHERE owner_name = ANY($1)
		`, pq.Array(ownerList))
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var id int64
				var name string
				rows.Scan(&id, &name)
				ownerIDs[strings.TrimSpace(name)] = id
			}
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

	log.Printf("    ✓ Populated foreign keys: %d courses, %d horses, %d trainers, %d jockeys, %d owners",
		len(courseIDs), len(horseIDs), len(trainerIDs), len(jockeyIDs), len(ownerIDs))

	return nil
}
