package verify

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
)

// Config holds verification configuration
type Config struct {
	MasterDir string
	From      time.Time
	To        time.Time
	Region    string // "gb", "ire", or "" for all
	Code      string // "flat", "jumps", or "" for all
	Verbose   bool
	AutoFix   bool
}

// Result represents verification results
type Result struct {
	DateRange      string
	TotalDays      int
	MasterRaces    int
	DBRaces        int
	MissingInDB    []MissingRace
	ExtraInDB      []ExtraRace
	Mismatches     []RunnerMismatch
	UnresolvedDims []UnresolvedDimension
	Summary        Summary
}

type MissingRace struct {
	Region   string
	Code     string
	Date     string
	RaceKey  string
	Course   string
	Off      string
	RaceName string
}

type ExtraRace struct {
	RaceID  int
	RaceKey string
	Date    string
	Course  string
}

type RunnerMismatch struct {
	RaceID        int
	RaceKey       string
	Date          string
	Course        string
	MasterRunners int
	DBRunners     int
	Difference    int
}

type UnresolvedDimension struct {
	Type        string // "horse", "trainer", "jockey"
	Name        string
	RaceKey     string
	Occurrences int
}

type Summary struct {
	TotalIssues        int
	MissingRaces       int
	ExtraRaces         int
	RunnerMismatches   int
	UnresolvedHorses   int
	UnresolvedTrainers int
	UnresolvedJockeys  int
}

// VerifyData compares master CSV files with database
func VerifyData(ctx context.Context, db *sqlx.DB, cfg Config) (*Result, error) {
	result := &Result{
		DateRange: fmt.Sprintf("%s to %s", cfg.From.Format("2006-01-02"), cfg.To.Format("2006-01-02")),
	}

	// 1. Scan master CSV files
	masterRaces, err := scanMasterFiles(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to scan master files: %w", err)
	}

	if cfg.Verbose {
		fmt.Printf("📁 Scanned master directory: %d races found\n", len(masterRaces))
	}

	result.MasterRaces = len(masterRaces)

	// 2. Query database for same period
	dbRaces, err := queryDBRaces(ctx, db, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to query database: %w", err)
	}

	if cfg.Verbose {
		fmt.Printf("🗄️  Queried database: %d races found\n", len(dbRaces))
	}

	result.DBRaces = len(dbRaces)

	// 3. Compare race keys
	masterKeys := make(map[string]MasterRace)
	for _, r := range masterRaces {
		masterKeys[r.RaceKey] = r
	}

	dbKeys := make(map[string]DBRace)
	for _, r := range dbRaces {
		dbKeys[r.RaceKey] = r
	}

	// Find missing in DB
	for key, master := range masterKeys {
		if _, exists := dbKeys[key]; !exists {
			result.MissingInDB = append(result.MissingInDB, MissingRace{
				Region:   master.Region,
				Code:     master.Code,
				Date:     master.Date,
				RaceKey:  master.RaceKey,
				Course:   master.Course,
				Off:      master.Off,
				RaceName: master.RaceName,
			})
		}
	}

	// Find extra in DB (not in master)
	for key, dbRace := range dbKeys {
		if _, exists := masterKeys[key]; !exists {
			result.ExtraInDB = append(result.ExtraInDB, ExtraRace{
				RaceID:  dbRace.RaceID,
				RaceKey: dbRace.RaceKey,
				Date:    dbRace.Date,
				Course:  dbRace.Course,
			})
		}
	}

	// 4. Check runner counts for matching races
	for key, master := range masterKeys {
		if dbRace, exists := dbKeys[key]; exists {
			// Compare runner counts
			if master.Ran > 0 && dbRace.RunnerCount != master.Ran {
				result.Mismatches = append(result.Mismatches, RunnerMismatch{
					RaceID:        dbRace.RaceID,
					RaceKey:       key,
					Date:          master.Date,
					Course:        master.Course,
					MasterRunners: master.Ran,
					DBRunners:     dbRace.RunnerCount,
					Difference:    master.Ran - dbRace.RunnerCount,
				})
			}
		}
	}

	// 5. Check unresolved dimensions
	unresolved, err := checkUnresolvedDimensions(ctx, db, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to check unresolved dimensions: %w", err)
	}
	result.UnresolvedDims = unresolved

	// 6. Build summary
	result.Summary = Summary{
		TotalIssues:      len(result.MissingInDB) + len(result.ExtraInDB) + len(result.Mismatches) + len(result.UnresolvedDims),
		MissingRaces:     len(result.MissingInDB),
		ExtraRaces:       len(result.ExtraInDB),
		RunnerMismatches: len(result.Mismatches),
	}

	for _, u := range unresolved {
		switch u.Type {
		case "horse":
			result.Summary.UnresolvedHorses++
		case "trainer":
			result.Summary.UnresolvedTrainers++
		case "jockey":
			result.Summary.UnresolvedJockeys++
		}
	}

	return result, nil
}

// MasterRace represents a race from master CSV
type MasterRace struct {
	Region   string
	Code     string
	Date     string
	Course   string
	Off      string
	RaceName string
	RaceKey  string
	Ran      int
}

// DBRace represents a race from database
type DBRace struct {
	RaceID      int
	RaceKey     string
	Date        string
	Course      string
	Ran         sql.NullInt32
	RunnerCount int
}

func scanMasterFiles(cfg Config) ([]MasterRace, error) {
	var races []MasterRace

	regions := []string{"gb", "ire"}
	if cfg.Region != "" {
		regions = []string{cfg.Region}
	}

	codes := []string{"flat", "jumps"}
	if cfg.Code != "" {
		codes = []string{cfg.Code}
	}

	for _, region := range regions {
		for _, code := range codes {
			// Find all monthly CSV files in date range
			baseDir := filepath.Join(cfg.MasterDir, region, code)

			if _, err := os.Stat(baseDir); os.IsNotExist(err) {
				if cfg.Verbose {
					fmt.Printf("⚠️  Directory not found: %s\n", baseDir)
				}
				continue
			}

			// List all month directories
			entries, err := os.ReadDir(baseDir)
			if err != nil {
				return nil, fmt.Errorf("failed to read directory %s: %w", baseDir, err)
			}

			for _, entry := range entries {
				if !entry.IsDir() {
					continue
				}

				// Check if this month is in our date range
				monthDir := entry.Name() // e.g., "2024-01"

				// Find races CSV
				raceCSV := filepath.Join(baseDir, monthDir, fmt.Sprintf("races_%s_%s_%s.csv", region, code, monthDir))

				if _, err := os.Stat(raceCSV); os.IsNotExist(err) {
					continue
				}

				// Read CSV
				monthRaces, err := readRaceCSV(raceCSV, region, code, cfg.From, cfg.To)
				if err != nil {
					if cfg.Verbose {
						fmt.Printf("⚠️  Failed to read %s: %v\n", raceCSV, err)
					}
					continue
				}

				races = append(races, monthRaces...)
			}
		}
	}

	return races, nil
}

func readRaceCSV(filepath string, region, code string, from, to time.Time) ([]MasterRace, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, err
	}

	// Find column indexes
	colIdx := make(map[string]int)
	for i, col := range header {
		colIdx[col] = i
	}

	var races []MasterRace

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Parse date
		dateStr := record[colIdx["date"]]
		raceDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		// Check if in range
		if raceDate.Before(from) || raceDate.After(to) {
			continue
		}

		ran := 0
		if ranStr := record[colIdx["ran"]]; ranStr != "" {
			fmt.Sscanf(ranStr, "%d", &ran)
		}

		race := MasterRace{
			Region:   region,
			Code:     code,
			Date:     dateStr,
			Course:   record[colIdx["course"]],
			Off:      record[colIdx["off"]],
			RaceName: record[colIdx["race_name"]],
			RaceKey:  record[colIdx["race_key"]],
			Ran:      ran,
		}

		races = append(races, race)
	}

	return races, nil
}

func queryDBRaces(ctx context.Context, db *sqlx.DB, cfg Config) ([]DBRace, error) {
	query := `
		SELECT 
			r.race_id,
			r.race_key,
			r.race_date::text AS date,
			c.course_name AS course,
			r.ran,
			COUNT(run.runner_id) AS runner_count
		FROM racing.races r
		JOIN racing.courses c ON c.course_id = r.course_id
		LEFT JOIN racing.runners run ON run.race_id = r.race_id AND run.pos_num IS NOT NULL
		WHERE r.race_date BETWEEN $1 AND $2
		GROUP BY r.race_id, r.race_key, r.race_date, c.course_name, r.ran
		ORDER BY r.race_date, r.race_key
	`

	var races []DBRace
	err := db.SelectContext(ctx, &races, query, cfg.From, cfg.To)
	if err != nil {
		return nil, err
	}

	return races, nil
}

func checkUnresolvedDimensions(ctx context.Context, db *sqlx.DB, cfg Config) ([]UnresolvedDimension, error) {
	// Check for NULL horse_id, trainer_id, jockey_id in runners
	query := `
		WITH unresolved AS (
			SELECT 
				CASE 
					WHEN r.horse_id IS NULL THEN 'horse'
					WHEN r.trainer_id IS NULL THEN 'trainer'
					WHEN r.jockey_id IS NULL THEN 'jockey'
				END AS dim_type,
				CASE 
					WHEN r.horse_id IS NULL THEN r.horse_name
					WHEN r.trainer_id IS NULL THEN r.trainer_name
					WHEN r.jockey_id IS NULL THEN r.jockey_name
				END AS name,
				ra.race_key
			FROM racing.runners r
			JOIN racing.races ra ON ra.race_id = r.race_id
			WHERE ra.race_date BETWEEN $1 AND $2
			  AND (r.horse_id IS NULL OR r.trainer_id IS NULL OR r.jockey_id IS NULL)
			  AND r.pos_num IS NOT NULL
		)
		SELECT 
			dim_type AS type,
			name,
			race_key,
			COUNT(*) AS occurrences
		FROM unresolved
		WHERE name IS NOT NULL AND name != ''
		GROUP BY dim_type, name, race_key
		ORDER BY occurrences DESC, type, name
	`

	var unresolved []UnresolvedDimension
	err := db.SelectContext(ctx, &unresolved, query, cfg.From, cfg.To)
	if err != nil {
		return nil, err
	}

	return unresolved, nil
}

// PrintResult formats and prints the verification result
func PrintResult(result *Result, verbose bool) {
	fmt.Println("\n" + strings.Repeat("━", 60))
	fmt.Println("🔍 Data Verification Report")
	fmt.Println(strings.Repeat("━", 60))

	fmt.Printf("\n📅 Date Range: %s\n", result.DateRange)
	fmt.Printf("📁 Master Races: %d\n", result.MasterRaces)
	fmt.Printf("🗄️  DB Races: %d\n", result.DBRaces)

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("📊 Summary")
	fmt.Println(strings.Repeat("─", 60))

	if result.Summary.TotalIssues == 0 {
		fmt.Println("✅ No issues found - data integrity verified!")
		return
	}

	fmt.Printf("Total Issues: %d\n\n", result.Summary.TotalIssues)

	if result.Summary.MissingRaces > 0 {
		fmt.Printf("❌ Missing in DB: %d races\n", result.Summary.MissingRaces)
	} else {
		fmt.Println("✅ No missing races")
	}

	if result.Summary.ExtraRaces > 0 {
		fmt.Printf("⚠️  Extra in DB: %d races (not in master)\n", result.Summary.ExtraRaces)
	} else {
		fmt.Println("✅ No extra races")
	}

	if result.Summary.RunnerMismatches > 0 {
		fmt.Printf("⚠️  Runner Count Mismatches: %d races\n", result.Summary.RunnerMismatches)
	} else {
		fmt.Println("✅ All runner counts match")
	}

	if result.Summary.UnresolvedHorses > 0 || result.Summary.UnresolvedTrainers > 0 || result.Summary.UnresolvedJockeys > 0 {
		fmt.Printf("⚠️  Unresolved Dimensions:\n")
		if result.Summary.UnresolvedHorses > 0 {
			fmt.Printf("   - Horses: %d\n", result.Summary.UnresolvedHorses)
		}
		if result.Summary.UnresolvedTrainers > 0 {
			fmt.Printf("   - Trainers: %d\n", result.Summary.UnresolvedTrainers)
		}
		if result.Summary.UnresolvedJockeys > 0 {
			fmt.Printf("   - Jockeys: %d\n", result.Summary.UnresolvedJockeys)
		}
	}

	// Detailed output if verbose
	if verbose {
		printDetailedIssues(result)
	}

	fmt.Println("\n" + strings.Repeat("━", 60))
}

func printDetailedIssues(result *Result) {
	if len(result.MissingInDB) > 0 {
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("❌ Missing Races in Database")
		fmt.Println(strings.Repeat("─", 60))

		// Group by date
		byDate := make(map[string][]MissingRace)
		for _, r := range result.MissingInDB {
			byDate[r.Date] = append(byDate[r.Date], r)
		}

		dates := make([]string, 0, len(byDate))
		for date := range byDate {
			dates = append(dates, date)
		}
		sort.Strings(dates)

		for _, date := range dates {
			races := byDate[date]
			fmt.Printf("\n📅 %s (%d races):\n", date, len(races))
			for _, r := range races {
				fmt.Printf("   %s - %s %s - %s\n", r.Off, r.Course, r.Region, r.RaceKey)
			}
		}
	}

	if len(result.Mismatches) > 0 {
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("⚠️  Runner Count Mismatches")
		fmt.Println(strings.Repeat("─", 60))

		for _, m := range result.Mismatches {
			fmt.Printf("%s - %s: Master=%d, DB=%d (diff: %+d)\n",
				m.Date, m.Course, m.MasterRunners, m.DBRunners, m.Difference)
		}
	}

	if len(result.UnresolvedDims) > 0 && len(result.UnresolvedDims) <= 20 {
		fmt.Println("\n" + strings.Repeat("─", 60))
		fmt.Println("⚠️  Unresolved Dimensions (top 20)")
		fmt.Println(strings.Repeat("─", 60))

		for i, u := range result.UnresolvedDims {
			if i >= 20 {
				break
			}
			fmt.Printf("%s: %s (race: %s)\n", u.Type, u.Name, u.RaceKey)
		}
	}
}
