package main

import (
	"fmt"
	"log"
	"time"
	
	"giddyup/api/internal/scraper"
)

func main() {
	today := time.Now().Format("2006-01-02")
	
	log.Printf("Testing full racecard scraper for %s...", today)
	
	rc := scraper.NewRacecardScraper()
	races, err := rc.GetTodaysRaces(today)
	
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	
	fmt.Printf("\n✅ Scraped %d races\n", len(races))
	if len(races) > 0 {
		r := races[0]
		fmt.Printf("\nSample race:\n")
		fmt.Printf("  Course: %s (%s)\n", r.Course, r.Region)
		fmt.Printf("  Off: %s\n", r.OffTime)
		fmt.Printf("  Name: %s\n", r.RaceName)
		fmt.Printf("  Type: %s\n", r.Type)
		fmt.Printf("  Runners: %d\n", len(r.Runners))
		if len(r.Runners) > 0 {
			fmt.Printf("  Sample runner: #%d %s (J: %s, T: %s)\n",
				r.Runners[0].Num, r.Runners[0].Horse, r.Runners[0].Jockey, r.Runners[0].Trainer)
		}
	}
}
