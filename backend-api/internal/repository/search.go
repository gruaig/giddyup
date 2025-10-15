package repository

import (
	"fmt"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type SearchRepository struct {
	db *database.DB
}

func NewSearchRepository(db *database.DB) *SearchRepository {
	return &SearchRepository{db: db}
}

// GlobalSearch performs trigram search across all entities
func (r *SearchRepository) GlobalSearch(query string, limit int) (*models.SearchResults, error) {
	if limit <= 0 {
		limit = 10
	}

	results := &models.SearchResults{
		Horses:   []models.SearchEntity{},
		Trainers: []models.SearchEntity{},
		Jockeys:  []models.SearchEntity{},
		Owners:   []models.SearchEntity{},
		Courses:  []models.SearchEntity{},
	}

	// Search horses
	horsesQuery := `
		SELECT horse_id AS id, horse_name AS name, 
		       similarity(horse_name, $1) AS score
		FROM racing.horses
		WHERE horse_name % $1
		ORDER BY score DESC
		LIMIT $2
	`
	if err := r.db.Select(&results.Horses, horsesQuery, query, limit); err != nil {
		return nil, fmt.Errorf("failed to search horses: %w", err)
	}
	for i := range results.Horses {
		results.Horses[i].Type = "horse"
	}

	// Search trainers
	trainersQuery := `
		SELECT trainer_id AS id, trainer_name AS name,
		       similarity(trainer_name, $1) AS score
		FROM racing.trainers
		WHERE trainer_name % $1
		ORDER BY score DESC
		LIMIT $2
	`
	if err := r.db.Select(&results.Trainers, trainersQuery, query, limit); err != nil {
		return nil, fmt.Errorf("failed to search trainers: %w", err)
	}
	for i := range results.Trainers {
		results.Trainers[i].Type = "trainer"
	}

	// Search jockeys
	jockeysQuery := `
		SELECT jockey_id AS id, jockey_name AS name,
		       similarity(jockey_name, $1) AS score
		FROM racing.jockeys
		WHERE jockey_name % $1
		ORDER BY score DESC
		LIMIT $2
	`
	if err := r.db.Select(&results.Jockeys, jockeysQuery, query, limit); err != nil {
		return nil, fmt.Errorf("failed to search jockeys: %w", err)
	}
	for i := range results.Jockeys {
		results.Jockeys[i].Type = "jockey"
	}

	// Search owners
	ownersQuery := `
		SELECT owner_id AS id, owner_name AS name,
		       similarity(owner_name, $1) AS score
		FROM racing.owners
		WHERE owner_name % $1
		ORDER BY score DESC
		LIMIT $2
	`
	if err := r.db.Select(&results.Owners, ownersQuery, query, limit); err != nil {
		return nil, fmt.Errorf("failed to search owners: %w", err)
	}
	for i := range results.Owners {
		results.Owners[i].Type = "owner"
	}

	// Search courses
	coursesQuery := `
		SELECT course_id AS id, course_name AS name,
		       similarity(course_name, $1) AS score
		FROM racing.courses
		WHERE course_name % $1
		ORDER BY score DESC
		LIMIT $2
	`
	if err := r.db.Select(&results.Courses, coursesQuery, query, limit); err != nil {
		return nil, fmt.Errorf("failed to search courses: %w", err)
	}
	for i := range results.Courses {
		results.Courses[i].Type = "course"
	}

	results.Total = len(results.Horses) + len(results.Trainers) +
		len(results.Jockeys) + len(results.Owners) + len(results.Courses)

	return results, nil
}

// SearchComments performs full-text search on runner comments
func (r *SearchRepository) SearchComments(params models.CommentSearchParams) ([]models.CommentSearchResult, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}

	query := `
		SELECT 
			ru.runner_id,
			r.race_id,
			r.race_date,
			c.course_name,
			r.race_name,
			h.horse_name,
			ru.comment,
			ts_rank(to_tsvector('english', ru.comment), plainto_tsquery('english', $1)) AS rank
		FROM racing.runners ru
		JOIN racing.races r ON r.race_id = ru.race_id
		JOIN racing.horses h ON h.horse_id = ru.horse_id
		JOIN racing.courses c ON c.course_id = r.course_id
		WHERE to_tsvector('english', ru.comment) @@ plainto_tsquery('english', $1)
	`

	args := []interface{}{params.Query}
	argCount := 1

	// Add default date filter if not provided (last 1 year for performance)
	if params.DateFrom == nil && params.DateTo == nil {
		// Default to last 1 year for performance (reduces scan from 2.2M to ~200k rows)
		query += " AND r.race_date >= CURRENT_DATE - INTERVAL '1 year'"
	}

	if params.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND r.race_date >= $%d", argCount)
		args = append(args, *params.DateFrom)
	}

	if params.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND r.race_date <= $%d", argCount)
		args = append(args, *params.DateTo)
	}

	if params.Region != nil {
		argCount++
		query += fmt.Sprintf(" AND r.region = $%d", argCount)
		args = append(args, *params.Region)
	}

	if params.CourseID != nil {
		argCount++
		query += fmt.Sprintf(" AND r.course_id = $%d", argCount)
		args = append(args, *params.CourseID)
	}

	argCount++
	query += fmt.Sprintf(" ORDER BY rank DESC, r.race_date DESC LIMIT $%d", argCount)
	args = append(args, params.Limit)

	var results []models.CommentSearchResult
	if err := r.db.Select(&results, query, args...); err != nil {
		return nil, fmt.Errorf("failed to search comments: %w", err)
	}

	return results, nil
}
