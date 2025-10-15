package loader

import (
	"database/sql"
	"fmt"
	"log"
	"strconv"
	"strings"

	"giddyup/api/internal/stitcher"
)

// BulkLoader handles bulk loading of race data into PostgreSQL
type BulkLoader struct {
	db *sql.DB
}

// LoadStats contains statistics about a load operation
type LoadStats struct {
	RacesLoaded   int
	RunnersLoaded int
	Duration      float64
}

// NewBulkLoader creates a new bulk loader
func NewBulkLoader(db *sql.DB) *BulkLoader {
	return &BulkLoader{
		db: db,
	}
}

// LoadRaces loads master races and runners into the database
func (l *BulkLoader) LoadRaces(races []stitcher.MasterRace, runners []stitcher.MasterRunner) (*LoadStats, error) {
	log.Printf("[Loader] Starting to load %d races and %d runners...", len(races), len(runners))

	// Step 1: Load all races first (so they exist for foreign key references)
	racesLoaded := 0
	for i, race := range races {
		err := l.loadRaceInTransaction(race)
		if err != nil {
			log.Printf("[Loader] Warning: Failed to load race %s: %v", race.RaceKey, err)
			continue
		}
		racesLoaded++

		if (i+1)%10 == 0 {
			log.Printf("[Loader] Progress: %d/%d races loaded", i+1, len(races))
		}
	}

	log.Printf("[Loader] Races complete: %d/%d loaded", racesLoaded, len(races))

	// Step 2: Now load runners (races are committed and available)
	runnersLoaded := 0
	batchSize := 50

	// Debug: Show first runner to be loaded
	if len(runners) > 0 {
		r := runners[0]
		log.Printf("[Loader DEBUG] First runner: key=%s, race_key=%s, horse='%s'",
			r.RunnerKey, r.RaceKey, r.Horse)
	}

	for i := 0; i < len(runners); i += batchSize {
		end := i + batchSize
		if end > len(runners) {
			end = len(runners)
		}

		batch := runners[i:end]
		loaded, err := l.loadRunnerBatch(batch)
		if err != nil {
			log.Printf("[Loader DEBUG] Batch load failed: %v, falling back to individual", err)
			// Fall back to individual loads for this batch
			for j, runner := range batch {
				if err := l.loadRunnerInTransaction(runner); err == nil {
					runnersLoaded++
				} else {
					if j < 3 { // Log first 3 errors
						log.Printf("[Loader DEBUG] Individual runner load failed: %v", err)
					}
				}
			}
		} else {
			runnersLoaded += loaded
		}

		if (i+batchSize)%100 == 0 {
			log.Printf("[Loader] Progress: %d/%d runners loaded", runnersLoaded, len(runners))
		}
	}

	log.Printf("[Loader] Successfully loaded %d races and %d runners", racesLoaded, runnersLoaded)

	return &LoadStats{
		RacesLoaded:   racesLoaded,
		RunnersLoaded: runnersLoaded,
	}, nil
}

// loadRaceInTransaction loads a single race in its own transaction
func (l *BulkLoader) loadRaceInTransaction(race stitcher.MasterRace) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = l.loadRace(tx, race)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// loadRunnerInTransaction loads a single runner in its own transaction
func (l *BulkLoader) loadRunnerInTransaction(runner stitcher.MasterRunner) error {
	tx, err := l.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	err = l.loadRunner(tx, runner)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// loadRunnerBatch loads multiple runners in a single transaction
func (l *BulkLoader) loadRunnerBatch(runners []stitcher.MasterRunner) (int, error) {
	tx, err := l.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	loaded := 0
	for _, runner := range runners {
		err := l.loadRunner(tx, runner)
		if err != nil {
			return 0, err // Return error to trigger individual loads
		}
		loaded++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return loaded, nil
}

// loadRace inserts or updates a single race
func (l *BulkLoader) loadRace(tx *sql.Tx, race stitcher.MasterRace) error {
	// First ensure course exists and get course_id
	var courseID int

	// Use upsert with correct unique constraint
	err := tx.QueryRow(`
		WITH ins AS (
			INSERT INTO racing.courses (course_name, region)
			VALUES ($1, $2)
			ON CONFLICT ON CONSTRAINT courses_uniq DO NOTHING
			RETURNING course_id
		)
		SELECT course_id FROM ins
		UNION ALL
		SELECT course_id FROM racing.courses 
		WHERE racing.norm_text(course_name) = racing.norm_text($1) AND region = $2
		LIMIT 1
	`, race.Course, race.Region).Scan(&courseID)

	if err != nil {
		return fmt.Errorf("failed to get/create course: %w", err)
	}

	// Insert or update race
	_, err = tx.Exec(`
		INSERT INTO racing.races (
			race_key, race_date, region, course_id, off_time, race_name, race_type, class, 
			dist_raw, going, surface, ran
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (race_key, race_date) DO UPDATE SET
			region = EXCLUDED.region,
			course_id = EXCLUDED.course_id,
			off_time = EXCLUDED.off_time,
			race_name = EXCLUDED.race_name,
			race_type = EXCLUDED.race_type,
			class = EXCLUDED.class,
			dist_raw = EXCLUDED.dist_raw,
			going = EXCLUDED.going,
			surface = EXCLUDED.surface,
			ran = EXCLUDED.ran
	`, race.RaceKey, race.Date, race.Region, courseID, nullString(race.OffTime), race.RaceName,
		race.Type, nullString(race.Class), race.Distance, nullString(race.Going), race.Surface, race.Ran)

	return err
}

// loadRunner inserts or updates a single runner
func (l *BulkLoader) loadRunner(tx *sql.Tx, runner stitcher.MasterRunner) error {
	// Use race_date from runner struct
	raceDate := runner.RaceDate

	// Get race_id from races table using race_key
	var raceID int64
	err := tx.QueryRow(`
		SELECT race_id FROM racing.races 
		WHERE race_key = $1 AND race_date = $2
		LIMIT 1
	`, runner.RaceKey, raceDate).Scan(&raceID)
	
	if err != nil {
		return fmt.Errorf("failed to find race_id for race_key %s: %w", runner.RaceKey, err)
	}

	// Get or create horse_id
	horseID, err := l.getOrCreateHorse(tx, runner.Horse, runner.HorseID)
	if err != nil {
		return fmt.Errorf("failed to get/create horse: %w", err)
	}

	// Get or create jockey_id
	jockeyID, err := l.getOrCreateJockey(tx, runner.Jockey, runner.JockeyID)
	if err != nil {
		return fmt.Errorf("failed to get/create jockey '%s': %w", runner.Jockey, err)
	}
	
	// Validate jockey_id is not 0 if jockey name is not empty
	if runner.Jockey != "" && jockeyID == 0 {
		return fmt.Errorf("getOrCreateJockey returned 0 for jockey '%s'", runner.Jockey)
	}

	// Get or create trainer_id
	trainerID, err := l.getOrCreateTrainer(tx, runner.Trainer, runner.TrainerID)
	if err != nil {
		return fmt.Errorf("failed to get/create trainer '%s': %w", runner.Trainer, err)
	}
	
	// Validate trainer_id is not 0 if trainer name is not empty
	if runner.Trainer != "" && trainerID == 0 {
		return fmt.Errorf("getOrCreateTrainer returned 0 for trainer '%s'", runner.Trainer)
	}

	// Insert or update runner (use race_id instead of race_key)
	// NOTE: "or" is a SQL reserved keyword, must be quoted as "or"
	_, err = tx.Exec(`
		INSERT INTO racing.runners (
			runner_key, race_id, race_date, num, pos_raw, draw,
			horse_id, age, jockey_id, trainer_id, lbs, 
			"or", rpr, comment,
			win_bsp, win_ppwap
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (runner_key, race_date) DO UPDATE SET
			num = EXCLUDED.num,
			pos_raw = EXCLUDED.pos_raw,
			draw = EXCLUDED.draw,
			horse_id = EXCLUDED.horse_id,
			age = EXCLUDED.age,
			jockey_id = EXCLUDED.jockey_id,
			trainer_id = EXCLUDED.trainer_id,
			lbs = EXCLUDED.lbs,
			"or" = EXCLUDED."or",
			rpr = EXCLUDED.rpr,
			comment = EXCLUDED.comment,
			win_bsp = EXCLUDED.win_bsp,
			win_ppwap = EXCLUDED.win_ppwap
	`, runner.RunnerKey, raceID, raceDate, nullInt(runner.Num), runner.Pos, nullInt(runner.Draw),
		horseID, nullInt(runner.Age), jockeyID, trainerID, nullInt(parseWeight(runner.Weight)),
		nullInt(runner.OR), nullInt(runner.RPR), runner.Comment,
		nullFloat(runner.WinBSP), nullFloat(runner.WinPPWAP))

	return err
}

// getOrCreateHorse gets or creates a horse record
func (l *BulkLoader) getOrCreateHorse(tx *sql.Tx, horseName string, externalID int) (int, error) {
	if horseName == "" {
		return 0, fmt.Errorf("horse name is empty")
	}

	var horseID int

	// Use upsert with correct unique constraint (only horse_name, no external_id)
	err := tx.QueryRow(`
		WITH ins AS (
			INSERT INTO racing.horses (horse_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT horses_uniq DO NOTHING
			RETURNING horse_id
		)
		SELECT horse_id FROM ins
		UNION ALL
		SELECT horse_id FROM racing.horses 
		WHERE racing.norm_text(horse_name) = racing.norm_text($1)
		LIMIT 1
	`, horseName).Scan(&horseID)

	return horseID, err
}

// getOrCreateJockey gets or creates a jockey record
func (l *BulkLoader) getOrCreateJockey(tx *sql.Tx, jockeyName string, externalID int) (int, error) {
	if jockeyName == "" {
		return 0, nil // NULL jockey ID
	}

	var jockeyID int

	// Use upsert with correct unique constraint (only jockey_name, no external_id)
	err := tx.QueryRow(`
		WITH ins AS (
			INSERT INTO racing.jockeys (jockey_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT jockeys_uniq DO NOTHING
			RETURNING jockey_id
		)
		SELECT jockey_id FROM ins
		UNION ALL
		SELECT jockey_id FROM racing.jockeys 
		WHERE racing.norm_text(jockey_name) = racing.norm_text($1)
		LIMIT 1
	`, jockeyName).Scan(&jockeyID)

	return jockeyID, err
}

// getOrCreateTrainer gets or creates a trainer record
func (l *BulkLoader) getOrCreateTrainer(tx *sql.Tx, trainerName string, externalID int) (int, error) {
	if trainerName == "" {
		return 0, nil // NULL trainer ID
	}

	var trainerID int

	// Use upsert with correct unique constraint (only trainer_name, no external_id)
	err := tx.QueryRow(`
		WITH ins AS (
			INSERT INTO racing.trainers (trainer_name)
			VALUES ($1)
			ON CONFLICT ON CONSTRAINT trainers_uniq DO NOTHING
			RETURNING trainer_id
		)
		SELECT trainer_id FROM ins
		UNION ALL
		SELECT trainer_id FROM racing.trainers 
		WHERE racing.norm_text(trainer_name) = racing.norm_text($1)
		LIMIT 1
	`, trainerName).Scan(&trainerID)

	return trainerID, err
}

// nullInt converts int to sql.NullInt64
func nullInt(val int) interface{} {
	if val == 0 {
		return nil
	}
	return val
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

// nullFloat converts float64 to sql.NullFloat64
func nullFloat(val float64) interface{} {
	if val == 0.0 {
		return nil
	}
	return val
}

// nullString converts empty string to NULL
func nullString(val string) interface{} {
	if val == "" {
		return nil
	}
	return val
}
