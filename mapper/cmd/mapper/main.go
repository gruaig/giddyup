package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/spf13/cobra"

	"giddyup/mapper/internal/verify"
)

var (
	// Global flags
	masterDir string
	dbHost    string
	dbPort    int
	dbName    string
	dbUser    string
	dbPass    string

	// Verify flags
	fromDate  string
	toDate    string
	region    string
	code      string
	today     bool
	yesterday bool
	verbose   bool
	autoFix   bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "mapper",
		Short: "GiddyUp Data Mapper - Verification & Fetching Service",
		Long:  `Verify data integrity and fetch fresh Racing Post data`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&masterDir, "master-dir", "/home/smonaghan/rpscrape/master", "Master data directory")
	rootCmd.PersistentFlags().StringVar(&dbHost, "db-host", "localhost", "Database host")
	rootCmd.PersistentFlags().IntVar(&dbPort, "db-port", 5432, "Database port")
	rootCmd.PersistentFlags().StringVar(&dbName, "db-name", "giddyup", "Database name")
	rootCmd.PersistentFlags().StringVar(&dbUser, "db-user", "postgres", "Database user")
	rootCmd.PersistentFlags().StringVar(&dbPass, "db-pass", "password", "Database password")

	// Verify command
	verifyCmd := &cobra.Command{
		Use:   "verify",
		Short: "Verify data integrity between master files and database",
		Long:  `Compare master CSV files with database to find missing data, mismatches, and issues`,
		RunE:  runVerify,
	}

	verifyCmd.Flags().StringVar(&fromDate, "from", "", "Start date (YYYY-MM-DD)")
	verifyCmd.Flags().StringVar(&toDate, "to", "", "End date (YYYY-MM-DD)")
	verifyCmd.Flags().StringVar(&region, "region", "", "Region filter (gb, ire)")
	verifyCmd.Flags().StringVar(&code, "code", "", "Race code filter (flat, jumps)")
	verifyCmd.Flags().BoolVar(&today, "today", false, "Verify today only")
	verifyCmd.Flags().BoolVar(&yesterday, "yesterday", false, "Verify yesterday only")
	verifyCmd.Flags().BoolVar(&verbose, "verbose", false, "Verbose output")
	verifyCmd.Flags().BoolVar(&autoFix, "fix", false, "Auto-fix missing data")

	// Test DB command
	testCmd := &cobra.Command{
		Use:   "test-db",
		Short: "Test database connection",
		RunE:  runTestDB,
	}

	rootCmd.AddCommand(verifyCmd)
	rootCmd.AddCommand(testCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runVerify(cmd *cobra.Command, args []string) error {
	// Parse date range
	var from, to time.Time
	var err error

	if today {
		from = time.Now().UTC().Truncate(24 * time.Hour)
		to = from
	} else if yesterday {
		from = time.Now().UTC().Add(-24 * time.Hour).Truncate(24 * time.Hour)
		to = from
	} else {
		if fromDate == "" {
			// Default: last 7 days
			from = time.Now().UTC().Add(-7 * 24 * time.Hour).Truncate(24 * time.Hour)
		} else {
			from, err = time.Parse("2006-01-02", fromDate)
			if err != nil {
				return fmt.Errorf("invalid from date: %w", err)
			}
		}

		if toDate == "" {
			to = time.Now().UTC().Truncate(24 * time.Hour)
		} else {
			to, err = time.Parse("2006-01-02", toDate)
			if err != nil {
				return fmt.Errorf("invalid to date: %w", err)
			}
		}
	}

	// Connect to database
	db, err := connectDB()
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}
	defer db.Close()

	fmt.Println("🔍 Starting data verification...")
	fmt.Printf("📅 Date range: %s to %s\n", from.Format("2006-01-02"), to.Format("2006-01-02"))
	if region != "" {
		fmt.Printf("🌍 Region: %s\n", region)
	}
	if code != "" {
		fmt.Printf("🏇 Code: %s\n", code)
	}
	fmt.Printf("📁 Master directory: %s\n", masterDir)
	fmt.Println()

	// Run verification
	ctx := context.Background()
	cfg := verify.Config{
		MasterDir: masterDir,
		From:      from,
		To:        to,
		Region:    region,
		Code:      code,
		Verbose:   verbose,
		AutoFix:   autoFix,
	}

	result, err := verify.VerifyData(ctx, db, cfg)
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// Print results
	verify.PrintResult(result, verbose)

	// Exit code based on issues
	if result.Summary.TotalIssues > 0 {
		fmt.Printf("\n💡 Tip: Run with --fix to auto-import missing data\n")
		fmt.Printf("💡 Tip: Run with --verbose for detailed issue list\n")
		os.Exit(1)
	}

	return nil
}

func runTestDB(cmd *cobra.Command, args []string) error {
	fmt.Println("Testing database connection...")
	fmt.Printf("Host: %s:%d\n", dbHost, dbPort)
	fmt.Printf("Database: %s\n", dbName)
	fmt.Printf("User: %s\n", dbUser)

	db, err := connectDB()
	if err != nil {
		return fmt.Errorf("❌ Connection failed: %w", err)
	}
	defer db.Close()

	// Test query
	var count int
	err = db.Get(&count, "SELECT COUNT(*) FROM racing.races")
	if err != nil {
		return fmt.Errorf("❌ Query failed: %w", err)
	}

	fmt.Printf("✅ Connected successfully!\n")
	fmt.Printf("✅ Found %d races in database\n", count)

	// Check recent data
	var maxDate string
	err = db.Get(&maxDate, "SELECT MAX(race_date)::text FROM racing.races")
	if err == nil && maxDate != "" {
		fmt.Printf("✅ Latest race date: %s\n", maxDate)
	}

	return nil
}

func connectDB() (*sqlx.DB, error) {
	connStr := fmt.Sprintf(
		"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable",
		dbHost, dbPort, dbName, dbUser, dbPass,
	)

	db, err := sqlx.Connect("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Set search path
	_, err = db.Exec("SET search_path TO racing, public")
	if err != nil {
		db.Close()
		return nil, err
	}

	return db, nil
}
