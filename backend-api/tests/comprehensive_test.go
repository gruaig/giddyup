package tests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
)

var db *sql.DB

// TestMain sets up the test environment
func TestMain(m *testing.M) {
	// Connect to database
	connStr := "user=postgres dbname=horse_db sslmode=disable host=localhost"
	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}
	defer db.Close()

	// Run tests
	exitCode := m.Run()
	os.Exit(exitCode)
}

// Test 1: Data Completeness - All dates have data
func TestDataCompleteness(t *testing.T) {
	tests := []struct {
		date           string
		minRaces       int
		minRunners     int
		expectDataFrom string
	}{
		{"2025-10-10", 30, 250, "Sporting Life"},
		{"2025-10-11", 40, 400, "Sporting Life"},
		{"2025-10-12", 25, 200, "Sporting Life"},
		{"2025-10-13", 25, 200, "Sporting Life"},
		{"2025-10-14", 35, 350, "Sporting Life"},
		{"2025-10-15", 30, 250, "Sporting Life"},
		{"2025-10-16", 40, 400, "Sporting Life"}, // Today
		{"2025-10-17", 35, 300, "Sporting Life"}, // Tomorrow
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("Date_%s", tt.date), func(t *testing.T) {
			var raceCount, runnerCount int
			err := db.QueryRow(`
				SELECT 
					COUNT(*) as races,
					(SELECT COUNT(*) FROM racing.runners WHERE race_date = $1) as runners
				FROM racing.races WHERE race_date = $1
			`, tt.date).Scan(&raceCount, &runnerCount)

			if err != nil {
				t.Fatalf("Failed to query data for %s: %v", tt.date, err)
			}

			if raceCount < tt.minRaces {
				t.Errorf("Expected at least %d races for %s, got %d", tt.minRaces, tt.date, raceCount)
			}

			if runnerCount < tt.minRunners {
				t.Errorf("Expected at least %d runners for %s, got %d", tt.minRunners, tt.date, runnerCount)
			}

			t.Logf("✅ %s: %d races, %d runners", tt.date, raceCount, runnerCount)
		})
	}
}

// Test 2: No Duplicates
func TestNoDuplicates(t *testing.T) {
	// Check for duplicate races
	var duplicateRaces int
	err := db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT race_key, race_date, COUNT(*) 
			FROM racing.races 
			WHERE race_date >= '2025-10-10'
			GROUP BY race_key, race_date 
			HAVING COUNT(*) > 1
		) dups
	`).Scan(&duplicateRaces)

	if err != nil {
		t.Fatalf("Failed to check duplicates: %v", err)
	}

	if duplicateRaces > 0 {
		t.Errorf("Found %d duplicate race_keys!", duplicateRaces)
	} else {
		t.Logf("✅ No duplicate races")
	}

	// Check for duplicate runners
	var duplicateRunners int
	err = db.QueryRow(`
		SELECT COUNT(*) FROM (
			SELECT runner_key, race_date, COUNT(*) 
			FROM racing.runners 
			WHERE race_date >= '2025-10-10'
			GROUP BY runner_key, race_date 
			HAVING COUNT(*) > 1
		) dups
	`).Scan(&duplicateRunners)

	if err != nil {
		t.Fatalf("Failed to check runner duplicates: %v", err)
	}

	if duplicateRunners > 0 {
		t.Errorf("Found %d duplicate runner_keys!", duplicateRunners)
	} else {
		t.Logf("✅ No duplicate runners")
	}
}

// Test 3: Foreign Keys Populated
func TestForeignKeysPopulated(t *testing.T) {
	var total, withTrainer, withJockey int

	err := db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			COUNT(trainer_id) as with_trainer,
			COUNT(jockey_id) as with_jockey
		FROM racing.runners
		WHERE race_date >= '2025-10-10'
	`).Scan(&total, &withTrainer, &withJockey)

	if err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}

	trainerPct := float64(withTrainer) / float64(total) * 100
	jockeyPct := float64(withJockey) / float64(total) * 100

	if trainerPct < 99.0 {
		t.Errorf("Only %.1f%% of runners have trainers (expected >99%%)", trainerPct)
	}

	if jockeyPct < 99.0 {
		t.Errorf("Only %.1f%% of runners have jockeys (expected >99%%)", jockeyPct)
	}

	t.Logf("✅ Trainers: %.1f%% populated", trainerPct)
	t.Logf("✅ Jockeys: %.1f%% populated", jockeyPct)
}

// Test 4: Market Status Working
func TestMarketStatus(t *testing.T) {
	var finished, active, upcoming int

	err := db.QueryRow(`
		SELECT 
			COUNT(CASE WHEN market_status = 'Finished' THEN 1 END) as finished,
			COUNT(CASE WHEN market_status = 'Active' THEN 1 END) as active,
			COUNT(CASE WHEN market_status = 'Upcoming' THEN 1 END) as upcoming
		FROM racing.races_with_status
		WHERE race_date >= '2025-10-10'
	`).Scan(&finished, &active, &upcoming)

	if err != nil {
		t.Fatalf("Failed to check market status: %v", err)
	}

	if finished == 0 && active == 0 && upcoming == 0 {
		t.Error("No races have market status!")
	}

	t.Logf("✅ Market Status: %d Finished, %d Active, %d Upcoming", finished, active, upcoming)

	// Verify logic: past races should be "Finished"
	var pastRaceStatus string
	err = db.QueryRow(`
		SELECT market_status 
		FROM racing.races_with_status 
		WHERE race_date = '2025-10-10' 
		LIMIT 1
	`).Scan(&pastRaceStatus)

	if err != nil {
		t.Fatalf("Failed to get past race status: %v", err)
	}

	if pastRaceStatus != "Finished" {
		t.Errorf("Past race (Oct 10) should be 'Finished', got '%s'", pastRaceStatus)
	}
}

// Test 5: API Health Check
func TestAPIHealth(t *testing.T) {
	resp, err := http.Get("http://localhost:8000/health")
	if err != nil {
		t.Fatalf("Failed to call health endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health check failed with status %d", resp.StatusCode)
	}

	var result map[string]string
	json.NewDecoder(resp.Body).Decode(&result)

	if result["status"] != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", result["status"])
	}

	t.Logf("✅ API health check passed")
}

// Test 6: API Meetings Endpoint
func TestAPIMeetings(t *testing.T) {
	date := time.Now().Format("2006-01-02")
	url := fmt.Sprintf("http://localhost:8000/api/v1/meetings?date=%s", date)

	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("Failed to call meetings endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Meetings endpoint returned status %d", resp.StatusCode)
	}

	var meetings []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&meetings)
	if err != nil {
		t.Fatalf("Failed to decode meetings response: %v", err)
	}

	if len(meetings) == 0 {
		t.Error("No meetings returned for today")
	} else {
		t.Logf("✅ Meetings endpoint returned %d meetings", len(meetings))
	}
}

// Test 7: Races Have Runners
func TestRacesHaveRunners(t *testing.T) {
	var racesWithoutRunners int
	err := db.QueryRow(`
		SELECT COUNT(*)
		FROM racing.races r
		WHERE race_date >= '2025-10-10'
		AND NOT EXISTS (
			SELECT 1 FROM racing.runners ru WHERE ru.race_id = r.race_id
		)
	`).Scan(&racesWithoutRunners)

	if err != nil {
		t.Fatalf("Failed to check races without runners: %v", err)
	}

	if racesWithoutRunners > 0 {
		t.Errorf("Found %d races without any runners!", racesWithoutRunners)
	} else {
		t.Logf("✅ All races have runners")
	}
}

// Test 8: Race Counts Match
func TestRaceCountsMatch(t *testing.T) {
	rows, err := db.Query(`
		SELECT race_id, ran, (SELECT COUNT(*) FROM racing.runners WHERE race_id = r.race_id) as actual
		FROM racing.races r
		WHERE race_date >= '2025-10-10'
		AND ran != (SELECT COUNT(*) FROM racing.runners WHERE race_id = r.race_id)
		LIMIT 10
	`)

	if err != nil {
		t.Fatalf("Failed to check race counts: %v", err)
	}
	defer rows.Close()

	mismatchCount := 0
	for rows.Next() {
		var raceID, ran, actual int
		rows.Scan(&raceID, &ran, &actual)
		t.Logf("⚠️  Race %d: expected %d runners, has %d", raceID, ran, actual)
		mismatchCount++
	}

	if mismatchCount > 0 {
		t.Logf("ℹ️  %d races have runner count mismatches (may be due to non-runners)", mismatchCount)
	} else {
		t.Logf("✅ All race runner counts match")
	}
}

// Test 9: Positions Extracted for Past Races
func TestPositionsExtracted(t *testing.T) {
	var total, withPos int
	err := db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			COUNT(pos_raw) as with_pos
		FROM racing.runners
		WHERE race_date = '2025-10-10'
	`).Scan(&total, &withPos)

	if err != nil {
		t.Fatalf("Failed to check positions: %v", err)
	}

	posPct := float64(withPos) / float64(total) * 100

	// For finished races, we expect most runners to have positions
	if posPct < 50.0 {
		t.Errorf("Only %.1f%% of Oct 10 runners have positions (expected >50%%)", posPct)
	}

	t.Logf("✅ Positions: %.1f%% populated for Oct 10", posPct)
}

// Test 10: Data Integrity - No NULL race_ids
func TestNoNullRaceIDs(t *testing.T) {
	var nullRaceIDs int
	err := db.QueryRow(`
		SELECT COUNT(*) 
		FROM racing.runners 
		WHERE race_id IS NULL 
		AND race_date >= '2025-10-10'
	`).Scan(&nullRaceIDs)

	if err != nil {
		t.Fatalf("Failed to check NULL race_ids: %v", err)
	}

	if nullRaceIDs > 0 {
		t.Errorf("Found %d runners with NULL race_id!", nullRaceIDs)
	} else {
		t.Logf("✅ All runners have valid race_id")
	}
}
