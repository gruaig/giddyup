package scraper

import (
	"strings"
	"time"
)

// UK timezone singleton
var ukLoc *time.Location

// UK returns the Europe/London timezone (handles BST/GMT automatically)
func UK() *time.Location {
	if ukLoc == nil {
		loc, err := time.LoadLocation("Europe/London")
		if err != nil {
			// Fallback to UTC if timezone data missing
			return time.UTC
		}
		ukLoc = loc
	}
	return ukLoc
}

// ParseBetfairTimeToLocal converts Betfair event_dt (UTC) to UK local time
// Input: "10-10-2025 13:55" (DD-MM-YYYY HH:MM in UTC)
// Output: "12:55" (HH:MM in Europe/London - auto-handles BST/GMT)
func ParseBetfairTimeToLocal(eventDt string) string {
	// Parse Betfair format: "DD-MM-YYYY HH:MM"
	utcTime, err := time.Parse("02-01-2006 15:04", eventDt)
	if err != nil {
		// Fallback: just extract the time part
		parts := strings.Fields(eventDt)
		if len(parts) >= 2 {
			return parts[1]
		}
		return ""
	}
	
	// Convert to UK local time (handles BST/GMT automatically)
	localTime := utcTime.In(UK())
	
	// Return HH:MM
	return localTime.Format("15:04")
}

// ParseSportingLifeTimeToLocal converts Sporting Life time to UK local
// Input: "13:40:00+00" or "13:40:00" with a date context
// Output: "13:40" in UK local time
func ParseSportingLifeTimeToLocal(dateISO, raceTime string) string {
	// If it has timezone info (+00), parse it
	if len(raceTime) > 8 {
		datetime := dateISO + "T" + raceTime
		tUTC, err := time.Parse("2006-01-02T15:04:05-07:00", datetime)
		if err == nil {
			return tUTC.In(UK()).Format("15:04")
		}
	}
	
	// Otherwise treat as already local, just normalize
	return NormalizeHHMM(raceTime)
}

// NormalizeHHMM handles all time formats and returns canonical "HH:MM"
// This is the bulletproof version that handles HH:MM:SS, H:MM, HHMM, etc.
func NormalizeHHMM(s string) string {
	// Use the existing normalizeTimeToHHMM from matcher.go
	// (We could move it here, but keeping it in matcher for now)
	return normalizeTimeToHHMM(s)
}

