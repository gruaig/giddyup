package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// SportingLifeCache handles caching of Sporting Life race data
type SportingLifeCache struct {
	dataDir string
}

// NewSportingLifeCache creates a new cache manager
func NewSportingLifeCache(dataDir string) *SportingLifeCache {
	return &SportingLifeCache{
		dataDir: dataDir,
	}
}

// SaveRaces saves races to cache file
func (slc *SportingLifeCache) SaveRaces(date string, races []Race) error {
	// Create directory: /data/sportinglife/
	dir := filepath.Join(slc.dataDir, "sportinglife")
	os.MkdirAll(dir, 0755)
	
	// Save as JSON: /data/sportinglife/2025-10-16.json
	filename := filepath.Join(dir, fmt.Sprintf("%s.json", date))
	
	data, err := json.MarshalIndent(races, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal races: %w", err)
	}
	
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filename, err)
	}
	
	log.Printf("[SportingLife Cache] Saved %d races to %s", len(races), filename)
	return nil
}

// LoadRaces loads races from cache if available
func (slc *SportingLifeCache) LoadRaces(date string) ([]Race, bool, error) {
	filename := filepath.Join(slc.dataDir, "sportinglife", fmt.Sprintf("%s.json", date))
	
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, false, nil // File doesn't exist
	}
	
	var races []Race
	err = json.Unmarshal(data, &races)
	if err != nil {
		return nil, false, fmt.Errorf("failed to unmarshal cache: %w", err)
	}
	
	return races, true, nil
}

// CacheExists checks if cached data exists for a date
func (slc *SportingLifeCache) CacheExists(date string) bool {
	filename := filepath.Join(slc.dataDir, "sportinglife", fmt.Sprintf("%s.json", date))
	_, err := os.Stat(filename)
	return err == nil
}


