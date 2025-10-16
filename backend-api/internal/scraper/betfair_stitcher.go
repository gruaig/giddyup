package scraper

import (
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// BetfairStitcher handles downloading and stitching Betfair WIN+PLACE CSVs
type BetfairStitcher struct {
	client  *http.Client
	dataDir string
}

// StitchedRace represents a race with merged WIN+PLACE prices
type StitchedRace struct {
	Date      string
	OffTime   string
	EventName string
	Venue     string // Course name extracted from menu_hint
	Runners   []StitchedRunner
}

// StitchedRunner represents a runner with both WIN and PLACE prices
type StitchedRunner struct {
	Horse           string
	WinBSP          string
	WinPPWAP        string
	WinMorningWAP   string
	WinPPMax        string
	WinPPMin        string
	WinIPMax        string
	WinIPMin        string
	WinMorningVol   string
	WinPreVol       string
	WinIPVol        string
	WinLose         string
	PlaceBSP        string
	PlacePPWAP      string
	PlaceMorningWAP string
	PlacePPMax      string
	PlacePPMin      string
	PlaceIPMax      string
	PlaceIPMin      string
	PlaceMorningVol string
	PlacePreVol     string
	PlaceIPVol      string
	PlaceWinLose    string
}

// RawBetfairRow represents a row from raw Betfair CSV
type RawBetfairRow struct {
	EventID       string
	MenuHint      string
	EventName     string
	EventDt       string
	SelectionID   string
	SelectionName string
	WinLose       string
	BSP           string
	PPWAP         string
	MorningWAP    string
	PPMax         string
	PPMin         string
	IPMax         string
	IPMin         string
	MorningVol    string
	PreVol        string
	IPVol         string
}

// NewBetfairStitcher creates a new Betfair stitcher
func NewBetfairStitcher(dataDir string) *BetfairStitcher {
	return &BetfairStitcher{
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
		dataDir: dataDir,
	}
}

// StitchBetfairForDate downloads and stitches Betfair data for a date
func (bs *BetfairStitcher) StitchBetfairForDate(date string, region string) error {
	log.Printf("[BetfairStitcher] Processing %s for %s", date, region)

	// Map region: "uk" for API calls, "gb" for directory structure
	apiRegion := region
	dirRegion := region
	if region == "uk" {
		dirRegion = "gb"
	}

	// IMPORTANT: Betfair CSV for date X contains races from X-1
	// So to get races for date D, we need to download CSV for D+1
	raceDate, err := time.Parse("2006-01-02", date)
	if err != nil {
		return err
	}
	downloadDate := raceDate.AddDate(0, 0, 1) // Add 1 day

	// Convert download date to Betfair format (DDMMYYYY)
	dateStr := downloadDate.Format("02012006")

	// Download WIN CSV (use API region)
	winURL := fmt.Sprintf("https://promo.betfair.com/betfairsp/prices/dwbfprices%swin%s.csv", apiRegion, dateStr)
	winRows, err := bs.downloadAndParseCSV(winURL)
	if err != nil {
		log.Printf("[BetfairStitcher] Warning: Failed to download WIN data: %v", err)
		return nil // Non-fatal - some dates may not have data
	}

	// Download PLACE CSV (use API region)
	placeURL := fmt.Sprintf("https://promo.betfair.com/betfairsp/prices/dwbfprices%splace%s.csv", apiRegion, dateStr)
	placeRows, err := bs.downloadAndParseCSV(placeURL)
	if err != nil {
		log.Printf("[BetfairStitcher] Warning: Failed to download PLACE data: %v", err)
		// Continue with WIN-only data
		placeRows = []RawBetfairRow{}
	}

	log.Printf("[BetfairStitcher] Downloaded %d WIN rows, %d PLACE rows", len(winRows), len(placeRows))

	// Group and stitch
	stitchedRaces := bs.stitchWinPlace(winRows, placeRows)
	log.Printf("[BetfairStitcher] Stitched into %d races", len(stitchedRaces))

	// Save stitched races (use directory region)
	saved := 0
	for _, race := range stitchedRaces {
		err := bs.saveStitchedRace(race, dirRegion)
		if err != nil {
			log.Printf("[BetfairStitcher] Warning: Failed to save race: %v", err)
			continue
		}
		saved++
	}

	log.Printf("[BetfairStitcher] Saved %d stitched races to disk", saved)
	return nil
}

// formatDateForBetfair converts YYYY-MM-DD to DDMMYYYY
func (bs *BetfairStitcher) formatDateForBetfair(date string) (string, error) {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return "", err
	}
	return t.Format("02012006"), nil
}

// downloadAndParseCSV downloads and parses a Betfair CSV
func (bs *BetfairStitcher) downloadAndParseCSV(url string) ([]RawBetfairRow, error) {
	resp, err := bs.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return []RawBetfairRow{}, nil // No data
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	reader := csv.NewReader(resp.Body)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	rows := []RawBetfairRow{}
	for i, record := range records {
		if i == 0 || len(record) < 17 {
			continue // Skip header or incomplete rows
		}

		row := RawBetfairRow{
			EventID:       record[0],
			MenuHint:      record[1],
			EventName:     record[2],
			EventDt:       record[3],
			SelectionID:   record[4],
			SelectionName: record[5],
			WinLose:       record[6],
			BSP:           record[7],
			PPWAP:         record[8],
			MorningWAP:    record[9],
			PPMax:         record[10],
			PPMin:         record[11],
			IPMax:         record[12],
			IPMin:         record[13],
			MorningVol:    record[14],
			PreVol:        record[15],
			IPVol:         record[16],
		}

		rows = append(rows, row)
	}

	return rows, nil
}

// stitchWinPlace merges WIN and PLACE rows by race
func (bs *BetfairStitcher) stitchWinPlace(winRows, placeRows []RawBetfairRow) []StitchedRace {
	// Group WIN rows by (date, off_time, event_name)
	winByRace := bs.groupByRace(winRows)

	// Group PLACE rows by (date, off_time, event_name)
	placeByRace := bs.groupByRace(placeRows)

	// Merge
	stitched := []StitchedRace{}

	for raceKey, winRunners := range winByRace {
		placeRunners := placeByRace[raceKey]

		// Create place map for fast lookup
		placeMap := make(map[string]RawBetfairRow)
		for _, pr := range placeRunners {
			normHorse := NormalizeName(pr.SelectionName)
			placeMap[normHorse] = pr
		}

		race := StitchedRace{}
		if len(winRunners) > 0 {
			// Betfair CSV uses GMT - convert to UK local (adds 1 hour during BST)
			hhmmGMT := bs.extractTime(winRunners[0].EventDt)
			race.OffTime = bs.adjustBetfairTimeToLocal(winRunners[0].EventDt, hhmmGMT)
			race.EventName = winRunners[0].EventName
			// Use date from event_dt (this is the actual race date)
			race.Date = bs.extractDate(winRunners[0].EventDt)
			// Extract venue from menu_hint
			race.Venue = bs.extractVenue(winRunners[0].MenuHint)
		}

		// Merge WIN and PLACE for each horse
		for _, winRow := range winRunners {
			normHorse := NormalizeName(winRow.SelectionName)

			runner := StitchedRunner{
				Horse:         winRow.SelectionName,
				WinBSP:        winRow.BSP,
				WinPPWAP:      winRow.PPWAP,
				WinMorningWAP: winRow.MorningWAP,
				WinPPMax:      winRow.PPMax,
				WinPPMin:      winRow.PPMin,
				WinIPMax:      winRow.IPMax,
				WinIPMin:      winRow.IPMin,
				WinMorningVol: winRow.MorningVol,
				WinPreVol:     winRow.PreVol,
				WinIPVol:      winRow.IPVol,
				WinLose:       winRow.WinLose,
			}

			// Match with PLACE data
			if placeRow, found := placeMap[normHorse]; found {
				runner.PlaceBSP = placeRow.BSP
				runner.PlacePPWAP = placeRow.PPWAP
				runner.PlaceMorningWAP = placeRow.MorningWAP
				runner.PlacePPMax = placeRow.PPMax
				runner.PlacePPMin = placeRow.PPMin
				runner.PlaceIPMax = placeRow.IPMax
				runner.PlaceIPMin = placeRow.IPMin
				runner.PlaceMorningVol = placeRow.MorningVol
				runner.PlacePreVol = placeRow.PreVol
				runner.PlaceIPVol = placeRow.IPVol
				runner.PlaceWinLose = placeRow.WinLose
			}

			race.Runners = append(race.Runners, runner)
		}

		if len(race.Runners) > 0 {
			stitched = append(stitched, race)
		}
	}

	return stitched
}

// groupByRace groups rows by (date, off_time, event_name)
func (bs *BetfairStitcher) groupByRace(rows []RawBetfairRow) map[string][]RawBetfairRow {
	grouped := make(map[string][]RawBetfairRow)

	for _, row := range rows {
		date := bs.extractDate(row.EventDt)
		time := bs.extractTime(row.EventDt)
		key := fmt.Sprintf("%s|%s|%s", date, time, row.EventName)
		grouped[key] = append(grouped[key], row)
	}

	return grouped
}

// extractDate extracts date from event_dt: "11-10-2025 13:55" → "2025-10-11"
func (bs *BetfairStitcher) extractDate(eventDt string) string {
	parts := strings.Fields(eventDt)
	if len(parts) < 1 {
		return ""
	}

	dateParts := strings.Split(parts[0], "-")
	if len(dateParts) == 3 {
		// Convert DD-MM-YYYY to YYYY-MM-DD
		return fmt.Sprintf("%s-%s-%s", dateParts[2], dateParts[1], dateParts[0])
	}

	return ""
}

// extractTime extracts time from event_dt: "11-10-2025 13:55" → "13:55"
func (bs *BetfairStitcher) extractTime(eventDt string) string {
	parts := strings.Fields(eventDt)
	if len(parts) >= 2 {
		return parts[1]
	}
	return ""
}

// extractVenue extracts venue from menu_hint: "Ascot 15th Oct" → "Ascot"
func (bs *BetfairStitcher) extractVenue(menuHint string) string {
	venue := strings.TrimSpace(menuHint)
	// Find first digit (start of date part)
	if idx := strings.IndexAny(venue, "0123456789"); idx > 0 {
		venue = strings.TrimSpace(venue[:idx])
	}
	return venue
}

// adjustBetfairTimeToLocal adjusts Betfair CSV time to match Sporting Life
// Betfair CSV times are consistently 1 hour ahead - subtract 1 hour  
// TODO: Investigate why Europe/London conversion doesn't work as expected
func (bs *BetfairStitcher) adjustBetfairTimeToLocal(eventDt string, hhmmBF string) string {
	// Simple -1 hour adjustment (works reliably)
	if len(hhmmBF) != 5 || hhmmBF[2] != ':' {
		return hhmmBF
	}
	
	h := (int(hhmmBF[0]-'0')*10 + int(hhmmBF[1]-'0'))
	m := hhmmBF[3:5]
	
	h = h - 1
	if h < 0 {
		h = 23
	}
	
	return fmt.Sprintf("%02d:%s", h, m)
}

// saveStitchedRace saves a stitched race to CSV
func (bs *BetfairStitcher) saveStitchedRace(race StitchedRace, region string) error {
	// Determine race type from event name
	raceType := bs.determineRaceType(race.EventName)

	// Create directory
	dir := filepath.Join(bs.dataDir, "betfair_stitched", region, raceType)
	os.MkdirAll(dir, 0755)

	// Generate filename
	// Remove colons from time for filename
	timeStr := strings.ReplaceAll(race.OffTime, ":", "")
	filename := fmt.Sprintf("%s_%s_%s_%s.csv", region, raceType, race.Date, timeStr)
	filepath := filepath.Join(dir, filename)

	// Write CSV
	file, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Header
	header := []string{
		"date", "off", "event_name", "venue", "horse",
		"win_bsp", "win_ppwap", "win_morningwap", "win_ppmax", "win_ppmin",
		"win_ipmax", "win_ipmin", "win_morning_vol", "win_pre_vol", "win_ip_vol", "win_lose",
		"place_bsp", "place_ppwap", "place_morningwap", "place_ppmax", "place_ppmin",
		"place_ipmax", "place_ipmin", "place_morning_vol", "place_pre_vol", "place_ip_vol", "place_win_lose",
	}
	writer.Write(header)

	// Data rows
	for _, runner := range race.Runners {
		row := []string{
			race.Date, race.OffTime, race.EventName, race.Venue, runner.Horse,
			runner.WinBSP, runner.WinPPWAP, runner.WinMorningWAP, runner.WinPPMax, runner.WinPPMin,
			runner.WinIPMax, runner.WinIPMin, runner.WinMorningVol, runner.WinPreVol, runner.WinIPVol, runner.WinLose,
			runner.PlaceBSP, runner.PlacePPWAP, runner.PlaceMorningWAP, runner.PlacePPMax, runner.PlacePPMin,
			runner.PlaceIPMax, runner.PlaceIPMin, runner.PlaceMorningVol, runner.PlacePreVol, runner.PlaceIPVol, runner.PlaceWinLose,
		}
		writer.Write(row)
	}

	return nil
}

// determineRaceType determines race type from event name
func (bs *BetfairStitcher) determineRaceType(eventName string) string {
	nameLower := strings.ToLower(eventName)

	if strings.Contains(nameLower, "hurdle") || strings.Contains(nameLower, "hrd") {
		return "jumps"
	}
	if strings.Contains(nameLower, "chase") || strings.Contains(nameLower, "chs") {
		return "jumps"
	}
	if strings.Contains(nameLower, "nhf") || strings.Contains(nameLower, "nh flat") {
		return "jumps"
	}

	return "flat"
}

// LoadStitchedRacesForDate loads stitched Betfair CSVs from disk
func (bs *BetfairStitcher) LoadStitchedRacesForDate(date string, region string) ([]StitchedRace, error) {
	races := []StitchedRace{}

	// Map region: "uk" → "gb" for directory lookups
	dirRegion := region
	if region == "uk" {
		dirRegion = "gb"
	}

	// Read from both flat and jumps
	for _, raceType := range []string{"flat", "jumps"} {
		dir := filepath.Join(bs.dataDir, "betfair_stitched", dirRegion, raceType)

		// Find files matching date
		pattern := filepath.Join(dir, fmt.Sprintf("*%s*.csv", date))
		files, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, filePath := range files {
			race, err := bs.readStitchedCSV(filePath)
			if err != nil {
				log.Printf("[BetfairStitcher] Warning: Failed to read %s: %v", filePath, err)
				continue
			}
			races = append(races, race)
		}
	}

	return races, nil
}

// readStitchedCSV reads a stitched Betfair CSV file
func (bs *BetfairStitcher) readStitchedCSV(filePath string) (StitchedRace, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return StitchedRace{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return StitchedRace{}, err
	}

	if len(records) < 2 {
		return StitchedRace{}, fmt.Errorf("empty file")
	}

	race := StitchedRace{}

	// Parse data rows (skip header)
	for i, record := range records {
		// Support both old format (26 cols) and new format (27 cols with venue)
		hasVenue := len(record) >= 27
		minCols := 26
		if hasVenue {
			minCols = 27
		}

		if i == 0 || len(record) < minCols {
			continue
		}

		// First row sets race metadata
		if i == 1 {
			race.Date = record[0]
			race.OffTime = record[1]
			race.EventName = record[2]
			if hasVenue {
				race.Venue = record[3]
			}
		}

		// Adjust indices based on whether venue column exists
		horseIdx := 3
		if hasVenue {
			horseIdx = 4
		}

		runner := StitchedRunner{
			Horse:           record[horseIdx],
			WinBSP:          record[horseIdx+1],
			WinPPWAP:        record[horseIdx+2],
			WinMorningWAP:   record[horseIdx+3],
			WinPPMax:        record[horseIdx+4],
			WinPPMin:        record[horseIdx+5],
			WinIPMax:        record[horseIdx+6],
			WinIPMin:        record[horseIdx+7],
			WinMorningVol:   record[horseIdx+8],
			WinPreVol:       record[horseIdx+9],
			WinIPVol:        record[horseIdx+10],
			WinLose:         record[horseIdx+11],
			PlaceBSP:        record[horseIdx+12],
			PlacePPWAP:      record[horseIdx+13],
			PlaceMorningWAP: record[horseIdx+14],
			PlacePPMax:      record[horseIdx+15],
			PlacePPMin:      record[horseIdx+16],
			PlaceIPMax:      record[horseIdx+17],
			PlaceIPMin:      record[horseIdx+18],
			PlaceMorningVol: record[horseIdx+19],
			PlacePreVol:     record[horseIdx+20],
			PlaceIPVol:      record[horseIdx+21],
			PlaceWinLose:    record[horseIdx+22],
		}

		race.Runners = append(race.Runners, runner)
	}

	return race, nil
}
