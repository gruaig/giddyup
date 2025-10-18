package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: delete_date <YYYY-MM-DD>")
	}

	dateStr := os.Args[1]

	// Get database connection
	connStr := fmt.Sprintf("host=%s port=%s dbname=%s user=%s password=%s sslmode=disable",
		getEnv("DB_HOST", "localhost"),
		getEnv("DB_PORT", "5432"),
		getEnv("DB_NAME", "horse_db"),
		getEnv("DB_USER", "postgres"),
		getEnv("DB_PASSWORD", "password"))

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("❌ Failed to connect: %v", err)
	}
	defer db.Close()

	log.Printf("🗑️  Deleting all data for %s...", dateStr)

	// Delete runners first (foreign key)
	result, err := db.Exec("DELETE FROM racing.runners WHERE race_id IN (SELECT race_id FROM racing.races WHERE race_date = $1)", dateStr)
	if err != nil {
		log.Fatalf("❌ Delete runners failed: %v", err)
	}
	runners, _ := result.RowsAffected()

	// Delete races
	result, err = db.Exec("DELETE FROM racing.races WHERE race_date = $1", dateStr)
	if err != nil {
		log.Fatalf("❌ Delete races failed: %v", err)
	}
	races, _ := result.RowsAffected()

	log.Printf("✅ Deleted %d races and %d runners for %s", races, runners, dateStr)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

