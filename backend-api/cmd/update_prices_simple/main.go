package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"giddyup/api/internal/betfair"

	_ "github.com/lib/pq"
)

func main() {
	// Parse flags
	targetDate := flag.String("date", time.Now().AddDate(0, 0, 1).Format("2006-01-02"), "Target date")
	continuous := flag.Bool("continuous", false, "Run continuously (every 30 mins)")
	flag.Parse()

	log.Println("🏇 Betfair Price Updater")
	log.Printf("📅 Date: %s", *targetDate)
	log.Println()

	// Connect to database
	dbURL := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "horse_db"))

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("❌ Database connection failed: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database ping failed: %v", err)
	}
	log.Println("✅ Database connected")
	log.Println()

	// Get Betfair credentials
	appKey := os.Getenv("BETFAIR_APP_KEY")
	sessionToken := os.Getenv("BETFAIR_SESSION_TOKEN")

	if appKey == "" {
		log.Fatal("❌ BETFAIR_APP_KEY not set")
	}

	// Authenticate if needed
	if sessionToken == "" {
		username := os.Getenv("BETFAIR_USERNAME")
		password := os.Getenv("BETFAIR_PASSWORD")

		if username == "" || password == "" {
			log.Fatal("❌ Need BETFAIR_SESSION_TOKEN or BETFAIR_USERNAME/PASSWORD")
		}

		log.Println("🔐 Authenticating with Betfair...")
		auth := betfair.NewAuthenticator(appKey, username, password)
		sessionToken, err = auth.Login()
		if err != nil {
			log.Fatalf("❌ Betfair auth failed: %v", err)
		}
		log.Println("✅ Authenticated")
	}

	bfClient := betfair.NewClient(appKey, sessionToken)
	updater := &PriceUpdater{
		db:         db,
		bfClient:   bfClient,
		targetDate: *targetDate,
	}

	// Run once or continuously
	if *continuous {
		log.Println("🔄 Starting continuous updates (every 30 minutes)")
		log.Println("   Press Ctrl+C to stop")
		log.Println()

		for {
			if err := updater.updatePrices(); err != nil {
				log.Printf("⚠️  Update failed: %v", err)
			}
			log.Printf("⏳ Next update in 30 minutes...")
			time.Sleep(30 * time.Minute)
		}
	} else {
		if err := updater.updatePrices(); err != nil {
			log.Fatalf("❌ Update failed: %v", err)
		}
	}
}

type PriceUpdater struct {
	db         *sql.DB
	bfClient   *betfair.Client
	targetDate string
}

func (u *PriceUpdater) updatePrices() error {
	ctx := context.Background()
	startTime := time.Now()

	log.Printf("═══════════════════════════════════════════════════════════")
	log.Printf("🔄 Updating prices at %s", startTime.Format("15:04:05"))
	log.Printf("═══════════════════════════════════════════════════════════")

	// Step 1: Discover Betfair markets
	log.Println("🔍 Discovering Betfair markets...")
	matcher := betfair.NewMatcher(u.bfClient)
	markets, err := matcher.FindTodaysMarkets(ctx, u.targetDate)
	if err != nil {
		return fmt.Errorf("find markets: %w", err)
	}

	log.Printf("✅ Found %d Betfair markets", len(markets))

	if len(markets) == 0 {
		log.Println("⚠️  No markets found - might be too early or markets not formed yet")
		return nil
	}

	// Step 2: Get market IDs
	marketIDs := make([]string, 0, len(markets))
	for _, m := range markets {
		marketIDs = append(marketIDs, m.MarketID)
	}

	// Step 3: Fetch market books (prices)
	log.Printf("💰 Fetching prices for %d markets...", len(marketIDs))
	marketBooks, err := u.bfClient.ListMarketBook(ctx, marketIDs)
	if err != nil {
		return fmt.Errorf("fetch prices: %w", err)
	}

	log.Printf("✅ Received %d market books", len(marketBooks))

	// Step 4: Update database
	totalUpdates := 0
	marketsUpdated := 0

	for _, book := range marketBooks {
		runnersUpdated := 0

		for _, runner := range book.Runners {
			// Calculate PPWAP (best back price)
			price := calculatePrice(runner)
			if price == 0 {
				continue
			}

			// Update runner by selection ID
			result, err := u.db.Exec(`
				UPDATE racing.runners ru
				SET win_ppwap = $1
				FROM racing.races r
				WHERE ru.race_id = r.race_id
				AND r.race_date = $2
				AND ru.betfair_selection_id = $3
			`, price, u.targetDate, runner.SelectionID)

			if err != nil {
				log.Printf("⚠️  Failed to update selection %d: %v", runner.SelectionID, err)
				continue
			}

			rowsAffected, _ := result.RowsAffected()
			if rowsAffected > 0 {
				runnersUpdated++
				totalUpdates++
			}
		}

		if runnersUpdated > 0 {
			marketsUpdated++
		}
	}

	elapsed := time.Since(startTime)
	log.Printf("✅ Updated %d runners across %d markets (took %v)", totalUpdates, marketsUpdated, elapsed.Round(time.Millisecond))

	// Show current coverage
	u.showCoverage()
	log.Println()

	return nil
}

func (u *PriceUpdater) showCoverage() {
	var total, havePPWAP, haveDec, haveEither int
	err := u.db.QueryRow(`
		SELECT 
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL) as have_ppwap,
			COUNT(*) FILTER (WHERE dec IS NOT NULL) as have_dec,
			COUNT(*) FILTER (WHERE win_ppwap IS NOT NULL OR dec IS NOT NULL) as have_either
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		WHERE r.race_date = $1
	`, u.targetDate).Scan(&total, &havePPWAP, &haveDec, &haveEither)

	if err != nil {
		log.Printf("⚠️  Failed to get coverage: %v", err)
		return
	}

	pctPPWAP := 0.0
	pctEither := 0.0
	if total > 0 {
		pctPPWAP = float64(havePPWAP) / float64(total) * 100
		pctEither = float64(haveEither) / float64(total) * 100
	}

	log.Printf("📊 Coverage: %d/%d runners (%.1f%%) have Betfair prices", havePPWAP, total, pctPPWAP)
	log.Printf("   Total with any odds: %d/%d (%.1f%%)", haveEither, total, pctEither)

	if pctEither >= 80 {
		log.Println("   ✅ READY for betting script!")
	} else if pctEither >= 50 {
		log.Println("   ⚠️  PARTIAL - getting there...")
	} else {
		log.Println("   ❌ NOT READY - markets still developing")
	}
}

func calculatePrice(runner betfair.RunnerBook) float64 {
	// Use best back price
	if runner.EX != nil && len(runner.EX.AvailableToBack) > 0 {
		return runner.EX.AvailableToBack[0].Price
	}

	// Fallback to last price traded
	if runner.LastPriceTraded != nil && *runner.LastPriceTraded > 0 {
		return *runner.LastPriceTraded
	}

	return 0
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

