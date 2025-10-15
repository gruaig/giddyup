package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// RaceCacheManager handles caching of scraped race data
type RaceCacheManager struct {
	dataDir string
}

// NewRaceCacheManager creates a new cache manager
func NewRaceCacheManager(dataDir string) *RaceCacheManager {
	return &RaceCacheManager{
		dataDir: dataDir,
	}
}

// SaveRaces saves races to cache files organized by region and type
func (rcm *RaceCacheManager) SaveRaces(date string, races []Race) error {
	// Group races by region and type
	grouped := make(map[string]map[string][]Race)
	
	for _, race := range races {
		region := strings.ToLower(race.Region)
		raceType := strings.ToLower(race.Type)
		
		if grouped[region] == nil {
			grouped[region] = make(map[string][]Race)
		}
		grouped[region][raceType] = append(grouped[region][raceType], race)
	}
	
	// Save each region/type combination to separate file
	for region, types := range grouped {
		for raceType, raceList := range types {
			// Create directory structure: /data/racingpost/gb/flat/
			dir := filepath.Join(rcm.dataDir, "racingpost", region, raceType)
			os.MkdirAll(dir, 0755)
			
			// Save as JSON: /data/racingpost/gb/flat/2025-10-09.json
			filename := filepath.Join(dir, fmt.Sprintf("%s.json", date))
			
			data, err := json.MarshalIndent(raceList, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal races: %w", err)
			}
			
			err = os.WriteFile(filename, data, 0644)
			if err != nil {
				return fmt.Errorf("failed to write file %s: %w", filename, err)
			}
			
			log.Printf("[Cache] Saved %d %s %s races to %s", len(raceList), region, raceType, filename)
		}
	}
	
	return nil
}

// LoadRaces loads races from cache if available
func (rcm *RaceCacheManager) LoadRaces(date string) ([]Race, bool, error) {
	allRaces := []Race{}
	foundAny := false
	
	// Check all possible region/type combinations
	regions := []string{"gb", "ire"}
	types := []string{"flat", "jumps", "nh flat"}
	
	for _, region := range regions {
		for _, raceType := range types {
			filename := filepath.Join(rcm.dataDir, "racingpost", region, raceType, fmt.Sprintf("%s.json", date))
			
			data, err := os.ReadFile(filename)
			if err != nil {
				continue // File doesn't exist, skip
			}
			
			var races []Race
			err = json.Unmarshal(data, &races)
			if err != nil {
				log.Printf("[Cache] Warning: Failed to unmarshal %s: %v", filename, err)
				continue
			}
			
			allRaces = append(allRaces, races...)
			foundAny = true
			log.Printf("[Cache] Loaded %d %s %s races from cache", len(races), region, raceType)
		}
	}
	
	if !foundAny {
		return nil, false, nil
	}
	
	return allRaces, true, nil
}

// CacheExists checks if cached data exists for a date
func (rcm *RaceCacheManager) CacheExists(date string) bool {
	regions := []string{"gb", "ire"}
	types := []string{"flat", "jumps", "nh flat"}
	
	for _, region := range regions {
		for _, raceType := range types {
			filename := filepath.Join(rcm.dataDir, "racingpost", region, raceType, fmt.Sprintf("%s.json", date))
			if _, err := os.Stat(filename); err == nil {
				return true
			}
		}
	}
	
	return false
}

