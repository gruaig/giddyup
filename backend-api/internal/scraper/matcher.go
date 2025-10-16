package scraper

import (
	"fmt"
	"log"
	"strconv"
	"strings"
)

// MatchAndMerge matches Sporting Life races with Betfair stitched data
// Uses course+time as primary key, race name+time as fallback, with ±1 minute tolerance
func MatchAndMerge(slRaces []Race, bfRaces []StitchedRace) []Race {
	// Build TWO Betfair lookup maps
	bfByCourse := make(map[string]StitchedRace) // PRIMARY: course+time
	bfByName := make(map[string]StitchedRace)   // FALLBACK: name+time

	for _, bfRace := range bfRaces {
		date := bfRace.Date
		normCourse := NormalizeCourseName(bfRace.Venue)
		normName := NormalizeName(bfRace.EventName)

		// Build keys with ±1 minute tolerance
		for _, timeVariant := range TimeVariants(bfRace.OffTime) {
			courseKey := fmt.Sprintf("%s|%s|%s", date, normCourse, timeVariant)
			bfByCourse[courseKey] = bfRace

			nameKey := fmt.Sprintf("%s|%s|%s", date, normName, timeVariant)
			bfByName[nameKey] = bfRace
		}
	}

	// Warn if no Betfair data
	if len(bfByCourse) == 0 {
		log.Println("   ⚠️  WARNING: No Betfair data loaded - check CSV files exist")
		return slRaces
	}

	// DEBUG: Show ALL unique venues
	bfVenues := make(map[string]bool)
	for _, bfRace := range bfRaces {
		if bfRace.Venue != "" {
			bfVenues[NormalizeCourseName(bfRace.Venue)] = true
		}
	}

	slVenues := make(map[string]bool)
	for _, slRace := range slRaces {
		if slRace.Course != "" {
			slVenues[NormalizeCourseName(slRace.Course)] = true
		}
	}

	log.Printf("   🔍 DEBUG: Betfair has %d unique venues, Sporting Life has %d unique venues", len(bfVenues), len(slVenues))

	// Find overlaps
	overlaps := 0
	for venue := range slVenues {
		if bfVenues[venue] {
			overlaps++
		}
	}
	log.Printf("   🔍 DEBUG: %d venues overlap between datasets", overlaps)

	// Show non-matching venues
	log.Println("   🔍 DEBUG: Sporting Life venues NOT in Betfair:")
	for venue := range slVenues {
		if !bfVenues[venue] {
			log.Printf("      Missing: %s", venue)
		}
	}

	// Match Sporting Life races
	matchedCount := 0
	matchedByCourse := 0
	matchedByName := 0

	for i := range slRaces {
		race := &slRaces[i]
		date := race.Date
		normCourse := NormalizeCourseName(race.Course)
		normName := NormalizeName(race.RaceName)
		normTime := normalizeTimeToHHMM(race.OffTime)

		var bfRace StitchedRace
		var found bool

		// PRIMARY: Course + time (more reliable!)
		for _, timeVariant := range TimeVariants(normTime) {
			courseKey := fmt.Sprintf("%s|%s|%s", date, normCourse, timeVariant)
			if r, ok := bfByCourse[courseKey]; ok {
				bfRace = r
				found = true
				matchedByCourse++
				break
			}
		}

		// FALLBACK: Race name + time
		if !found {
			for _, timeVariant := range TimeVariants(normTime) {
				nameKey := fmt.Sprintf("%s|%s|%s", date, normName, timeVariant)
				if r, ok := bfByName[nameKey]; ok {
					bfRace = r
					found = true
					matchedByName++
					break
				}
			}
		}

		if !found {
			// DEBUG: Show why this race didn't match
			log.Printf("      ❌ No match: %s @ %s (%s)",
				race.Course, race.OffTime, race.RaceName[:min(30, len(race.RaceName))])
			continue
		}

		matchedCount++
		log.Printf("      ✅ Matched: %s @ %s", race.Course, race.OffTime)

		// Build Betfair runner map by horse name
		bfRunnerMap := make(map[string]StitchedRunner)
		for _, bfRunner := range bfRace.Runners {
			normHorse := NormalizeName(bfRunner.Horse)
			bfRunnerMap[normHorse] = bfRunner
		}

		// Merge Betfair prices into runners
		runnerMatches := 0
		for j := range race.Runners {
			runner := &race.Runners[j]
			normHorse := NormalizeName(runner.Horse)

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
				runnerMatches++
			}
		}
	}

	log.Printf("   • Matched %d/%d races (by course: %d, by name: %d)",
		matchedCount, len(slRaces), matchedByCourse, matchedByName)
	return slRaces
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizeTimeToHHMM handles all time formats and returns canonical "HH:MM"
func normalizeTimeToHHMM(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	
	// Fast-path: exact "HH:MM"
	if len(s) == 5 && s[2] == ':' && s[0] >= '0' && s[0] <= '2' {
		return s
	}
	
	// Accept "HH:MM:SS"
	if len(s) >= 7 && strings.Count(s, ":") >= 2 {
		parts := strings.SplitN(s, ":", 3)
		if len(parts) >= 2 {
			h, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
				return fmt.Sprintf("%02d:%02d", h, m)
			}
		}
	}
	
	// Accept "H:MM" and "HH:MM" (non-zero-padded hour)
	if strings.Count(s, ":") == 1 {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			h, err1 := strconv.Atoi(parts[0])
			m, err2 := strconv.Atoi(parts[1])
			if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
				return fmt.Sprintf("%02d:%02d", h, m)
			}
		}
	}
	
	// Accept "HHMM" and "HMM"
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	
	if len(digits) == 4 {
		h, err1 := strconv.Atoi(digits[:2])
		m, err2 := strconv.Atoi(digits[2:])
		if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
			return fmt.Sprintf("%02d:%02d", h, m)
		}
	}
	
	if len(digits) == 3 { // e.g. "935" => "09:35"
		h, err1 := strconv.Atoi(digits[:1])
		m, err2 := strconv.Atoi(digits[1:])
		if err1 == nil && err2 == nil && h >= 0 && h < 24 && m >= 0 && m < 60 {
			return fmt.Sprintf("%02d:%02d", h, m)
		}
	}
	
	// Last resort: return as-is (better to see it in debug)
	return s
}

// parseFloat safely converts string to float64
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0.0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0.0
	}
	return f
}
