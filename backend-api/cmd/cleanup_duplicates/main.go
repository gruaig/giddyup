package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/lib/pq"
)

func main() {
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

	log.Println("🧹 Cleaning up duplicate runners...")

	// Delete runners with NULL horse_id (bad autoupdate duplicates)
	result, err := db.Exec(`
		DELETE FROM racing.runners 
		WHERE horse_id IS NULL
		AND race_date < CURRENT_DATE  -- Only clean historical dates
	`)
	if err != nil {
		log.Fatalf("❌ Delete failed: %v", err)
	}

	deleted, _ := result.RowsAffected()
	log.Printf("✅ Deleted %d duplicate runners (NULL horse_id)", deleted)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

