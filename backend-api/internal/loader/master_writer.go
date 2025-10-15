package loader

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"giddyup/api/internal/stitcher"
)

// MasterWriter writes matched data to master CSV files
type MasterWriter struct {
	dataDir string
}

// NewMasterWriter creates a new master writer
func NewMasterWriter(dataDir string) *MasterWriter {
	return &MasterWriter{
		dataDir: dataDir,
	}
}

// SaveMasterData saves races and runners to master CSV files
func (mw *MasterWriter) SaveMasterData(
	date string,
	races []stitcher.MasterRace,
	runners []stitcher.MasterRunner,
) error {
	// Group by region and type
	type GroupKey struct {
		Region   string
		RaceType string
	}

	racesByGroup := make(map[GroupKey][]stitcher.MasterRace)
	runnersByGroup := make(map[GroupKey][]stitcher.MasterRunner)

	for _, race := range races {
		key := GroupKey{
			Region:   strings.ToLower(race.Region),
			RaceType: strings.ToLower(race.Type),
		}
		racesByGroup[key] = append(racesByGroup[key], race)
	}

	for _, runner := range runners {
		// Find the race to get region/type
		var region, raceType string
		for _, race := range races {
			if race.RaceKey == runner.RaceKey {
				region = strings.ToLower(race.Region)
				raceType = strings.ToLower(race.Type)
				break
			}
		}

		if region != "" {
			key := GroupKey{Region: region, RaceType: raceType}
			runnersByGroup[key] = append(runnersByGroup[key], runner)
		}
	}

	// Save each group
	for key, groupRaces := range racesByGroup {
		groupRunners := runnersByGroup[key]

		// Create directory: /data/master/gb/flat/2025-10/
		yearMonth := date[:7] // "2025-10-09" → "2025-10"
		dir := filepath.Join(mw.dataDir, "master", key.Region, key.RaceType, yearMonth)
		os.MkdirAll(dir, 0755)

		// Save races CSV
		racesFile := filepath.Join(dir, fmt.Sprintf("races_%s_%s_%s.csv", key.Region, key.RaceType, yearMonth))
		err := mw.writeRacesCSV(racesFile, groupRaces)
		if err != nil {
			return err
		}

		// Save runners CSV
		runnersFile := filepath.Join(dir, fmt.Sprintf("runners_%s_%s_%s.csv", key.Region, key.RaceType, yearMonth))
		err = mw.writeRunnersCSV(runnersFile, groupRunners)
		if err != nil {
			return err
		}

		log.Printf("[MasterWriter] Saved %d races and %d runners to %s", len(groupRaces), len(groupRunners), dir)
	}

	return nil
}

// writeRacesCSV writes races to CSV matching Python format
func (mw *MasterWriter) writeRacesCSV(filename string, races []stitcher.MasterRace) error {
	// Read existing races first
	existingRaces := make(map[string]bool)
	if file, err := os.Open(filename); err == nil {
		reader := csv.NewReader(file)
		records, _ := reader.ReadAll()
		file.Close()

		for i, record := range records {
			if i == 0 || len(record) < 18 {
				continue
			}
			raceKey := record[17]
			existingRaces[raceKey] = true
		}
	}

	// Open for append or create
	var file *os.File
	var err error
	writeHeader := false

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		file, err = os.Create(filename)
		writeHeader = true
	} else {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	}

	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header only for new files
	if writeHeader {
		header := []string{
			"date", "region", "course", "off", "race_name", "type", "class", "pattern",
			"rating_band", "age_band", "sex_rest", "dist", "dist_f", "dist_m",
			"going", "surface", "ran", "race_key",
		}
		writer.Write(header)
	}

	// Data rows (only new races)
	for _, race := range races {
		if existingRaces[race.RaceKey] {
			continue // Skip duplicates
		}

		row := []string{
			race.Date,
			race.Region,
			race.Course,
			race.OffTime,
			race.RaceName,
			race.Type,
			race.Class,
			race.Pattern,
			race.RatingBand,
			race.AgeBand,
			race.SexRest,
			race.Distance,
			fmt.Sprintf("%.1f", race.DistanceF),
			"", // dist_m not in struct
			race.Going,
			race.Surface,
			fmt.Sprintf("%d", race.Ran),
			race.RaceKey,
		}
		writer.Write(row)
	}

	return nil
}

// writeRunnersCSV writes runners to CSV matching Python format
func (mw *MasterWriter) writeRunnersCSV(filename string, runners []stitcher.MasterRunner) error {
	// Read existing runners first
	existingRunners := make(map[string]bool)
	if file, err := os.Open(filename); err == nil {
		reader := csv.NewReader(file)
		records, _ := reader.ReadAll()
		file.Close()

		for i, record := range records {
			if i == 0 || len(record) < 1 {
				continue
			}
			runnerKey := record[0]
			existingRunners[runnerKey] = true
		}
	}

	// Open for append or create
	var file *os.File
	var err error
	writeHeader := false

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		file, err = os.Create(filename)
		writeHeader = true
	} else {
		file, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0644)
	}

	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header only for new files
	if writeHeader {
		header := []string{
			"runner_key", "race_key", "num", "pos", "draw", "horse", "age",
			"jockey", "trainer", "lbs", "or", "rpr", "sp", "comment",
			"win_bsp", "win_ppwap", "place_bsp", "place_ppwap",
			"match_jaccard", "match_time_diff_min", "match_reason",
		}
		writer.Write(header)
	}

	// Data rows (only new runners)
	for _, runner := range runners {
		if existingRunners[runner.RunnerKey] {
			continue // Skip duplicates
		}

		row := []string{
			runner.RunnerKey,
			runner.RaceKey,
			fmt.Sprintf("%d", runner.Num),
			runner.Pos,
			fmt.Sprintf("%d", runner.Draw),
			runner.Horse,
			fmt.Sprintf("%d", runner.Age),
			runner.Jockey,
			runner.Trainer,
			runner.Weight,
			fmt.Sprintf("%d", runner.OR),
			fmt.Sprintf("%d", runner.RPR),
			runner.SP,
			runner.Comment,
			fmt.Sprintf("%.8f", runner.WinBSP),
			fmt.Sprintf("%.4f", runner.WinPPWAP),
			fmt.Sprintf("%.8f", runner.PlaceBSP),
			fmt.Sprintf("%.4f", runner.PlacePPWAP),
			fmt.Sprintf("%.4f", runner.MatchJaccard),
			fmt.Sprintf("%d", runner.MatchTimeDiff),
			runner.MatchReason,
		}
		writer.Write(row)
	}

	return nil
}
