package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"time"

	"giddyup/api/internal/config"
	"giddyup/api/internal/database"

	_ "github.com/lib/pq"
)

func main() {
	// Parse flags
	targetDate := flag.String("date", time.Now().Format("2006-01-02"), "Target date (YYYY-MM-DD)")
	flag.Parse()

	log.Println("🏇 GiddyUp Betfair Price Updater")
	log.Printf("📅 Target Date: %s\n", *targetDate)
	log.Println()

	// Load config
	cfg := config.MustLoad()

	// Connect to database
	log.Println("🔌 Connecting to database...")
	db, err := database.NewDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()
	log.Println("✅ Database connected")
	log.Println()

	// Step 1: Get races for target date
	log.Printf("📥 [1/4] Getting races for %s...\n", *targetDate)
	races, err := getRacesForDate(db, *targetDate)
	if err != nil {
		log.Fatalf("Failed to get races: %v", err)
	}
	log.Printf("✅ Found %d races\n", len(races))
	log.Println()

	// Step 2: Get runners with Betfair selection IDs
	log.Println("📥 [2/4] Getting runners with Betfair selection IDs...")
	runners, err := getRunnersWithSelectionIDs(db, *targetDate)
	if err != nil {
		log.Fatalf("Failed to get runners: %v", err)
	}
	log.Printf("✅ Found %d runners with Betfair IDs\n", len(runners))
	log.Println()

	if len(runners) == 0 {
		log.Println("⚠️  No runners with Betfair selection IDs found")
		log.Println("   → Ensure betfair_selection_id is populated in racing.runners")
		log.Println("   → Run Sporting Life scraper to get selection IDs")
		return
	}

	// Step 3: Fetch prices from Betfair
	log.Println("💰 [3/4] Fetching prices from Betfair...")
	log.Println("   ⚠️  NOTE: This requires Betfair API credentials")
	log.Println("   ⚠️  Betfair API integration needed - see TODO below")
	log.Println()

	// TODO: Implement Betfair API price fetching
	// For now, show what needs to be done
	log.Println("📋 TODO: Implement Betfair API Integration")
	log.Println("   1. Get Betfair API session token")
	log.Println("   2. List markets for date")
	log.Println("   3. Get market books (prices) for each market")
	log.Println("   4. Match selection IDs to runners")
	log.Println("   5. Extract back/lay prices")
	log.Println("   6. Calculate PPWAP (weighted average)")
	log.Println("   7. Update racing.runners with win_ppwap")
	log.Println()

	// Step 4: Show what data we have
	log.Println("📊 [4/4] Current Status:")
	showPriceStatus(db, *targetDate)
}

type Race struct {
	RaceID    int64
	RaceName  string
	OffTime   time.Time
	CourseName string
}

type Runner struct {
	RunnerID         int64
	RaceID           int64
	BetfairSelectionID sql.NullInt64
	WinPPWAP         sql.NullFloat64
}

func getRacesForDate(db *database.DB, date string) ([]Race, error) {
	query := `
		SELECT 
			r.race_id,
			r.race_name,
			r.off_time,
			COALESCE(c.course_name, 'Unknown') as course_name
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE r.race_date = $1
		ORDER BY r.off_time
	`

	var races []Race
	rows, err := db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var r Race
		if err := rows.Scan(&r.RaceID, &r.RaceName, &r.OffTime, &r.CourseName); err != nil {
			return nil, err
		}
		races = append(races, r)
	}

	return races, rows.Err()
}

func getRunnersWithSelectionIDs(db *database.DB, date string) ([]Runner, error) {
	query := `
		SELECT 
			ru.runner_id,
			ru.race_id,
			ru.betfair_selection_id,
			ru.win_ppwap
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE r.race_date = $1
		AND ru.betfair_selection_id IS NOT NULL
		ORDER BY ru.race_id, ru.runner_id
	`

	var runners []Runner
	rows, err := db.Query(query, date)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ru Runner
		if err := rows.Scan(&ru.RunnerID, &ru.RaceID, &ru.BetfairSelectionID, &ru.WinPPWAP); err != nil {
			return nil, err
		}
		runners = append(runners, ru)
	}

	return runners, rows.Err()
}

func showPriceStatus(db *database.DB, date string) {
	query := `
		SELECT 
			COUNT(DISTINCT r.race_id) as races,
			COUNT(ru.runner_id) as total_runners,
			COUNT(*) FILTER (WHERE ru.betfair_selection_id IS NOT NULL) as have_betfair_id,
			COUNT(*) FILTER (WHERE ru.win_ppwap IS NOT NULL) as have_ppwap,
			COUNT(*) FILTER (WHERE ru.dec IS NOT NULL) as have_dec,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.betfair_selection_id IS NOT NULL) / COUNT(*), 1) as pct_betfair_id,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_ppwap IS NOT NULL) / COUNT(*), 1) as pct_ppwap,
			ROUND(100.0 * COUNT(*) FILTER (WHERE ru.win_ppwap IS NOT NULL OR ru.dec IS NOT NULL) / COUNT(*), 1) as pct_any_odds
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE r.race_date = $1
	`

	var stats struct {
		Races          int
		TotalRunners   int
		HaveBetfairID  int
		HavePPWAP      int
		HaveDec        int
		PctBetfairID   float64
		PctPPWAP       float64
		PctAnyOdds     float64
	}

	err := db.QueryRow(query, date).Scan(
		&stats.Races,
		&stats.TotalRunners,
		&stats.HaveBetfairID,
		&stats.HavePPWAP,
		&stats.HaveDec,
		&stats.PctBetfairID,
		&stats.PctPPWAP,
		&stats.PctAnyOdds,
	)

	if err != nil {
		log.Printf("Error getting stats: %v\n", err)
		return
	}

	fmt.Printf("   Races: %d\n", stats.Races)
	fmt.Printf("   Total runners: %d\n", stats.TotalRunners)
	fmt.Println()
	fmt.Printf("   Betfair selection IDs: %d/%d (%.1f%%)\n", stats.HaveBetfairID, stats.TotalRunners, stats.PctBetfairID)
	fmt.Printf("   Win PPWAP (Betfair): %d/%d (%.1f%%) ⭐ NEEDED FOR BETTING\n", stats.HavePPWAP, stats.TotalRunners, stats.PctPPWAP)
	fmt.Printf("   Decimal odds (fallback): %d/%d\n", stats.HaveDec, stats.TotalRunners)
	fmt.Println()

	if stats.PctAnyOdds >= 80 {
		fmt.Println("✅ READY: Betting script can run (>80% odds coverage)")
	} else if stats.PctAnyOdds >= 50 {
		fmt.Println("⚠️  PARTIAL: Some bets possible but not optimal")
	} else {
		fmt.Println("❌ NOT READY: Need prices before betting script can run")
	}
	fmt.Println()

	if stats.PctBetfairID < 80 {
		fmt.Println("⚠️  Low Betfair selection ID coverage")
		fmt.Println("   → Re-scrape with Sporting Life V2 to get selection IDs")
	}
}

