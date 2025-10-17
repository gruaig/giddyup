package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

func main() {
	// Connect to database
	connStr := "user=postgres dbname=horse_db sslmode=disable host=localhost"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	log.Println("🔍 Finding orphaned course_ids...")

	// Get all orphaned course_ids with sample race names
	rows, err := db.Query(`
		SELECT 
			r.course_id,
			r.region,
			COUNT(*) as race_count,
			STRING_AGG(DISTINCT r.race_name, ' | ') FILTER (WHERE r.race_date >= '2024-01-01') as recent_sample
		FROM racing.races r
		WHERE r.course_id NOT IN (SELECT course_id FROM racing.courses)
		GROUP BY r.course_id, r.region
		ORDER BY race_count DESC
		LIMIT 20
	`)

	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	fmt.Println("\nTop 20 Orphaned Course IDs:")
	fmt.Println("─────────────────────────────────────────────────────────────")

	for rows.Next() {
		var courseID int64
		var region string
		var raceCount int
		var sample sql.NullString

		rows.Scan(&courseID, &region, &raceCount, &sample)

		sampleStr := "No recent races"
		if sample.Valid {
			if len(sample.String) > 100 {
				sampleStr = sample.String[:97] + "..."
			} else {
				sampleStr = sample.String
			}
		}

		fmt.Printf("ID %5d (%3s): %5d races - %s\n", courseID, region, raceCount, sampleStr)
	}

	fmt.Println("\nTotal orphaned course_ids to fix...")
}
