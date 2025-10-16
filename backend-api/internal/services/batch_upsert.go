package services

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lib/pq"
)

// upsertNamesAndFetchIDs performs batch upsert for single-column entity tables
// Returns map[name]id for all provided names
// This replaces N individual INSERT+SELECT queries with 3 total queries
func upsertNamesAndFetchIDs(tx *sql.Tx, table, idCol, nameCol, uniqConstraint string, names []string) (map[string]int64, error) {
	if len(names) == 0 {
		return map[string]int64{}, nil
	}

	// 1) Create TEMP table
	tmp := fmt.Sprintf("tmp_%s_names", table)
	createSQL := fmt.Sprintf(`CREATE TEMP TABLE %s (name text) ON COMMIT DROP`, pq.QuoteIdentifier(tmp))
	if _, err := tx.Exec(createSQL); err != nil {
		return nil, fmt.Errorf("create temp %s: %w", tmp, err)
	}

	// 2) COPY values (super fast bulk insert)
	stmt, err := tx.Prepare(pq.CopyIn(tmp, "name"))
	if err != nil {
		return nil, fmt.Errorf("copyin prep %s: %w", tmp, err)
	}

	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		if _, err := stmt.Exec(n); err != nil {
			stmt.Close()
			return nil, fmt.Errorf("copyin %s exec: %w", tmp, err)
		}
	}

	// Flush COPY
	if _, err := stmt.Exec(); err != nil {
		stmt.Close()
		return nil, err
	}
	if err := stmt.Close(); err != nil {
		return nil, err
	}

	// 3) INSERT new rows (single query with ON CONFLICT)
	insSQL := fmt.Sprintf(`
		WITH src AS (
			SELECT DISTINCT name FROM %s WHERE name IS NOT NULL AND name <> ''
		)
		INSERT INTO racing.%s (%s)
		SELECT name FROM src
		ON CONFLICT ON CONSTRAINT %s DO NOTHING
	`, pq.QuoteIdentifier(tmp), pq.QuoteIdentifier(table), pq.QuoteIdentifier(nameCol), uniqConstraint)

	if _, err := tx.Exec(insSQL); err != nil {
		return nil, fmt.Errorf("insert %s: %w", table, err)
	}

	// 4) SELECT all IDs using normalized join (single query)
	selSQL := fmt.Sprintf(`
		SELECT t.%s, t.%s
		FROM racing.%s t
		JOIN %s s ON racing.norm_text(t.%s) = racing.norm_text(s.name)
	`, pq.QuoteIdentifier(idCol), pq.QuoteIdentifier(nameCol),
		pq.QuoteIdentifier(table), pq.QuoteIdentifier(tmp), pq.QuoteIdentifier(nameCol))

	rows, err := tx.Query(selSQL)
	if err != nil {
		return nil, fmt.Errorf("select %s ids: %w", table, err)
	}
	defer rows.Close()

	out := make(map[string]int64, len(names))
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(name)] = id
	}

	return out, rows.Err()
}

// upsertCoursesAndFetchIDs performs batch upsert for courses (two columns: name + region)
func upsertCoursesAndFetchIDs(tx *sql.Tx, courses map[string]string) (map[string]int64, error) {
	if len(courses) == 0 {
		return map[string]int64{}, nil
	}

	tmp := "tmp_courses"
	if _, err := tx.Exec(`CREATE TEMP TABLE ` + tmp + ` (course_name text, region text) ON COMMIT DROP`); err != nil {
		return nil, fmt.Errorf("create temp courses: %w", err)
	}

	stmt, err := tx.Prepare(pq.CopyIn(tmp, "course_name", "region"))
	if err != nil {
		return nil, fmt.Errorf("copyin prep courses: %w", err)
	}

	for name, region := range courses {
		name = strings.TrimSpace(name)
		region = strings.TrimSpace(region)
		if name == "" {
			continue
		}
		if _, err := stmt.Exec(name, region); err != nil {
			stmt.Close()
			return nil, err
		}
	}

	if _, err := stmt.Exec(); err != nil {
		stmt.Close()
		return nil, err
	}
	if err := stmt.Close(); err != nil {
		return nil, err
	}

	// Insert with region update on conflict
	ins := `
		WITH src AS (
			SELECT DISTINCT course_name, region FROM tmp_courses
			WHERE course_name IS NOT NULL AND course_name <> ''
		)
		INSERT INTO racing.courses (course_name, region)
		SELECT course_name, region FROM src
		ON CONFLICT ON CONSTRAINT courses_uniq DO UPDATE SET region = EXCLUDED.region
	`
	if _, err := tx.Exec(ins); err != nil {
		return nil, fmt.Errorf("insert courses: %w", err)
	}

	// Fetch IDs
	sel := `
		SELECT c.course_id, c.course_name
		FROM racing.courses c
		JOIN tmp_courses t ON racing.norm_text(c.course_name) = racing.norm_text(t.course_name)
	`
	rows, err := tx.Query(sel)
	if err != nil {
		return nil, fmt.Errorf("select course ids: %w", err)
	}
	defer rows.Close()

	out := map[string]int64{}
	for rows.Next() {
		var id int64
		var nm string
		if err := rows.Scan(&id, &nm); err != nil {
			return nil, err
		}
		out[strings.TrimSpace(nm)] = id
	}

	return out, rows.Err()
}

