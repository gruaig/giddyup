package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"giddyup/api/internal/scraper"

	_ "github.com/lib/pq"
)

func main() {
	// Parse command line arguments
	dateFlag := flag.String("date", "", "Date to fetch (YYYY-MM-DD)")
	forceFlag := flag.Bool("force", false, "Force refresh (delete existing data)")
	flag.Parse()

	// If no flag provided, check positional argument
	if *dateFlag == "" && len(flag.Args()) > 0 {
		*dateFlag = flag.Args()[0]
	}

	// Validate date
	if *dateFlag == "" {
		fmt.Println("❌ Error: Date is required")
		fmt.Println("")
		fmt.Println("Usage:")
		fmt.Println("  ./fetch_all <date>              # e.g., ./fetch_all 2024-10-15")
		fmt.Println("  ./fetch_all --date 2024-10-15")
		fmt.Println("  ./fetch_all --date 2024-10-15 --force")
		fmt.Println("")
		fmt.Println("Options:")
		fmt.Println("  --force    Delete existing data before fetching")
		os.Exit(1)
	}

	dateStr := *dateFlag

	// Validate date format
	if _, err := time.Parse("2006-01-02", dateStr); err != nil {
		log.Fatalf("❌ Invalid date format: %s (expected YYYY-MM-DD)", dateStr)
	}

	log.Printf("🏇 GiddyUp Data Fetcher")
	log.Printf("📅 Date: %s", dateStr)
	log.Printf("🔄 Force refresh: %v", *forceFlag)
	log.Println("")

	// Get database connection from environment
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

	// Get data directory
	dataDir := getEnv("DATA_DIR", "/home/smonaghan/GiddyUp/data")

	// Force refresh if requested
	if *forceFlag {
		log.Println("🗑️  Force refresh enabled - deleting existing data...")
		tx, _ := db.Begin()
		tx.Exec("DELETE FROM racing.runners WHERE race_id IN (SELECT race_id FROM racing.races WHERE race_date = $1)", dateStr)
		tx.Exec("DELETE FROM racing.races WHERE race_date = $1", dateStr)
		tx.Commit()
		log.Println("✅ Existing data deleted")
		log.Println("")
	}

	// Step 1: Fetch from Sporting Life
	log.Printf("📥 [1/4] Fetching race data from Sporting Life for %s...", dateStr)
	slScraper := scraper.NewSportingLifeAPIV2()
	races, err := slScraper.GetRacesForDate(dateStr)
	if err != nil {
		log.Fatalf("❌ Sporting Life fetch failed: %v", err)
	}
	log.Printf("✅ Got %d UK/IRE races from Sporting Life", len(races))
	log.Println("")

	// Step 2: Fetch and stitch Betfair data
	log.Println("📥 [2/4] Fetching Betfair CSV data...")
	bfStitcher := scraper.NewBetfairStitcher(dataDir)
	
	log.Printf("   • Stitching UK Betfair data...")
	bfStitcher.StitchBetfairForDate(dateStr, "uk")
	
	log.Printf("   • Stitching IRE Betfair data...")
	bfStitcher.StitchBetfairForDate(dateStr, "ire")

	bfUK, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "uk")
	bfIRE, _ := bfStitcher.LoadStitchedRacesForDate(dateStr, "ire")
	allBetfair := append(bfUK, bfIRE...)
	
	log.Printf("✅ Got %d Betfair races (UK: %d, IRE: %d)", len(allBetfair), len(bfUK), len(bfIRE))
	log.Println("")

	// Step 3: Match and merge
	log.Println("🔀 [3/4] Matching and merging Sporting Life ↔ Betfair...")
	mergedRaces := matchAndMerge(races, allBetfair)
	log.Printf("✅ Merged %d races", len(mergedRaces))
	log.Println("")

	// Step 4: Insert to database
	log.Println("💾 [4/4] Inserting to database...")
	racesInserted, runnersInserted, err := insertToDatabase(db, dateStr, mergedRaces)
	if err != nil {
		log.Fatalf("❌ Database insert failed: %v", err)
	}
	
	log.Println("")
	log.Println("🎉 SUCCESS!")
	log.Printf("✅ Inserted %d races with %d runners for %s", racesInserted, runnersInserted, dateStr)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func matchAndMerge(slRaces []scraper.Race, bfRaces []scraper.StitchedRace) []scraper.Race {
	// Build Betfair lookup map
	bfMap := make(map[string]scraper.StitchedRace)
	for _, bfRace := range bfRaces {
		normName := scraper.NormalizeName(bfRace.EventName)
		normTime := normalizeTime(bfRace.OffTime)
		key := fmt.Sprintf("%s|%s|%s", bfRace.Date, normName, normTime)
		bfMap[key] = bfRace
	}

	// Match and merge
	matchedCount := 0
	for i := range slRaces {
		race := &slRaces[i]
		normName := scraper.NormalizeName(race.RaceName)
		normTime := normalizeTime(race.OffTime)
		key := fmt.Sprintf("%s|%s|%s", race.Date, normName, normTime)

		bfRace, found := bfMap[key]
		if !found {
			continue
		}

		matchedCount++

		// Build Betfair runner map by horse name
		bfRunnerMap := make(map[string]scraper.StitchedRunner)
		for _, bfRunner := range bfRace.Runners {
			normHorse := scraper.NormalizeName(bfRunner.Horse)
			bfRunnerMap[normHorse] = bfRunner
		}

		// Merge Betfair prices into runners
		for j := range race.Runners {
			runner := &race.Runners[j]
			normHorse := scraper.NormalizeName(runner.Horse)

			if bfRunner, found := bfRunnerMap[normHorse]; found {
				runner.WinBSP = parseFloat(bfRunner.WinBSP)
				runner.WinPPWAP = parseFloat(bfRunner.WinPPWAP)
				runner.WinMorningWAP = parseFloat(bfRunner.WinMorningWAP)
				runner.WinPPMax = parseFloat(bfRunner.WinPPMax)
				runner.WinPPMin = parseFloat(bfRunner.WinPPMin)
				runner.PlaceBSP = parseFloat(bfRunner.PlaceBSP)
				runner.PlacePPWAP = parseFloat(bfRunner.PlacePPWAP)
				runner.PlaceMorningWAP = parseFloat(bfRunner.PlaceMorningWAP)
				runner.PlacePPMax = parseFloat(bfRunner.PlacePPMax)
				runner.PlacePPMin = parseFloat(bfRunner.PlacePPMin)
			}
		}
	}

	log.Printf("   • Matched %d/%d races with Betfair data", matchedCount, len(slRaces))
	return slRaces
}

func normalizeTime(t string) string {
	// Normalize time to HH:MM format
	if len(t) >= 5 {
		return t[:5] // "12:35:00" -> "12:35"
	}
	return t
}

func insertToDatabase(db *sql.DB, dateStr string, races []scraper.Race) (int, int, error) {
	ctx := context.Background()

	// Start transaction
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert dimension tables first
	log.Println("   • Upserting courses, horses, jockeys, trainers, owners...")
	if err := upsertDimensions(tx, races); err != nil {
		return 0, 0, fmt.Errorf("upsert dimensions: %w", err)
	}

	// Populate foreign keys
	log.Println("   • Looking up foreign key IDs...")
	if err := populateForeignKeys(tx, races); err != nil {
		return 0, 0, fmt.Errorf("populate foreign keys: %w", err)
	}

	// Insert races and runners
	raceCount := 0
	runnerCount := 0

	for _, race := range races {
		raceKey := generateRaceKey(race)

		// Insert race
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
			nullString(race.Going), nullString(race.Surface), race.Ran, false).Scan(&raceID)

		if err != nil {
			return 0, 0, fmt.Errorf("insert race %s: %w", raceKey, err)
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
			`,
				runnerKey, raceID, race.Date,
				nullInt64(runner.HorseID), nullInt64(runner.TrainerID), nullInt64(runner.JockeyID), nullInt64(runner.OwnerID),
				nullInt(runner.Num), nullString(runner.Pos), nullInt(runner.Draw), nullInt(runner.Age),
				nullInt(runner.Lbs), nullInt(runner.OR), nullInt(runner.RPR), nullString(runner.Comment),
				nullFloat64BSP(runner.WinBSP), nullFloat64(runner.WinPPWAP),
				nullFloat64BSP(runner.PlaceBSP), nullFloat64(runner.PlacePPWAP),
				nullInt64(int(runner.BetfairSelectionID)), nullFloat64(runner.BestOdds), nullString(runner.BestBookmaker),
			)

			if err != nil {
				return 0, 0, fmt.Errorf("insert runner %s: %w", runnerKey, err)
			}

			runnerCount++
		}
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return 0, 0, fmt.Errorf("commit transaction: %w", err)
	}

	return raceCount, runnerCount, nil
}

// Helper functions (copied from autoupdate.go)
func upsertDimensions(tx *sql.Tx, races []scraper.Race) error {
	courses := make(map[string]string)
	horses := make(map[string]bool)
	trainers := make(map[string]bool)
	jockeys := make(map[string]bool)
	owners := make(map[string]bool)

	for _, race := range races {
		if race.Course != "" {
			courses[race.Course] = race.Region
		}
		for _, runner := range race.Runners {
			if runner.Horse != "" {
				horses[runner.Horse] = true
			}
			if runner.Trainer != "" {
				trainers[runner.Trainer] = true
			}
			if runner.Jockey != "" {
				jockeys[runner.Jockey] = true
			}
			if runner.Owner != "" {
				owners[runner.Owner] = true
			}
		}
	}

	for courseName, region := range courses {
		_, err := tx.Exec(`INSERT INTO racing.courses (course_name, region) VALUES ($1, $2) ON CONFLICT ON CONSTRAINT courses_uniq DO NOTHING`, courseName, region)
		if err != nil {
			return err
		}
	}

	for horse := range horses {
		_, err := tx.Exec(`INSERT INTO racing.horses (horse_name) VALUES ($1) ON CONFLICT ON CONSTRAINT horses_uniq DO NOTHING`, horse)
		if err != nil {
			return err
		}
	}

	for trainer := range trainers {
		_, err := tx.Exec(`INSERT INTO racing.trainers (trainer_name) VALUES ($1) ON CONFLICT ON CONSTRAINT trainers_uniq DO NOTHING`, trainer)
		if err != nil {
			return err
		}
	}

	for jockey := range jockeys {
		_, err := tx.Exec(`INSERT INTO racing.jockeys (jockey_name) VALUES ($1) ON CONFLICT ON CONSTRAINT jockeys_uniq DO NOTHING`, jockey)
		if err != nil {
			return err
		}
	}

	for owner := range owners {
		_, err := tx.Exec(`INSERT INTO racing.owners (owner_name) VALUES ($1) ON CONFLICT ON CONSTRAINT owners_uniq DO NOTHING`, owner)
		if err != nil {
			return err
		}
	}

	return nil
}

func populateForeignKeys(tx *sql.Tx, races []scraper.Race) error {
	courseIDs := make(map[string]int64)
	horseIDs := make(map[string]int64)
	trainerIDs := make(map[string]int64)
	jockeyIDs := make(map[string]int64)
	ownerIDs := make(map[string]int64)

	// Look up all IDs
	for _, race := range races {
		if race.Course != "" && courseIDs[race.Course] == 0 {
			var id int64
			tx.QueryRow(`SELECT course_id FROM racing.courses WHERE racing.norm_text(course_name) = racing.norm_text($1)`, race.Course).Scan(&id)
			courseIDs[race.Course] = id
		}

		for _, runner := range race.Runners {
			if runner.Horse != "" && horseIDs[runner.Horse] == 0 {
				var id int64
				tx.QueryRow(`SELECT horse_id FROM racing.horses WHERE racing.norm_text(horse_name) = racing.norm_text($1)`, runner.Horse).Scan(&id)
				horseIDs[runner.Horse] = id
			}
			if runner.Trainer != "" && trainerIDs[runner.Trainer] == 0 {
				var id int64
				tx.QueryRow(`SELECT trainer_id FROM racing.trainers WHERE racing.norm_text(trainer_name) = racing.norm_text($1)`, runner.Trainer).Scan(&id)
				trainerIDs[runner.Trainer] = id
			}
			if runner.Jockey != "" && jockeyIDs[runner.Jockey] == 0 {
				var id int64
				tx.QueryRow(`SELECT jockey_id FROM racing.jockeys WHERE racing.norm_text(jockey_name) = racing.norm_text($1)`, runner.Jockey).Scan(&id)
				jockeyIDs[runner.Jockey] = id
			}
			if runner.Owner != "" && ownerIDs[runner.Owner] == 0 {
				var id int64
				tx.QueryRow(`SELECT owner_id FROM racing.owners WHERE racing.norm_text(owner_name) = racing.norm_text($1)`, runner.Owner).Scan(&id)
				ownerIDs[runner.Owner] = id
			}
		}
	}

	// Populate foreign keys in the race data
	for i := range races {
		races[i].CourseID = int(courseIDs[races[i].Course])

		for j := range races[i].Runners {
			races[i].Runners[j].HorseID = int(horseIDs[races[i].Runners[j].Horse])
			races[i].Runners[j].TrainerID = int(trainerIDs[races[i].Runners[j].Trainer])
			races[i].Runners[j].JockeyID = int(jockeyIDs[races[i].Runners[j].Jockey])
			races[i].Runners[j].OwnerID = int(ownerIDs[races[i].Runners[j].Owner])
		}
	}

	return nil
}

func generateRaceKey(race scraper.Race) string {
	return fmt.Sprintf("%s|%s|%s|%s", race.Date, scraper.NormalizeName(race.Course), race.OffTime, scraper.NormalizeName(race.RaceName))
}

func generateRunnerKey(raceKey string, runner scraper.Runner) string {
	return fmt.Sprintf("%s|%s|%d", raceKey, scraper.NormalizeName(runner.Horse), runner.Num)
}

func nullString(s string) interface{} {
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

func parseFloat(s string) float64 {
	if s == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}

