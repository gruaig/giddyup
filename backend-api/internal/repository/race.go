package repository

import (
	"fmt"
	"strings"

	"giddyup/api/internal/database"
	"giddyup/api/internal/models"
)

type RaceRepository struct {
	db *database.DB
}

func NewRaceRepository(db *database.DB) *RaceRepository {
	return &RaceRepository{db: db}
}

// SearchRaces searches races with filters
func (r *RaceRepository) SearchRaces(filters models.RaceFilters) ([]models.Race, error) {
	if filters.Limit <= 0 {
		filters.Limit = 100
	}

	query := `
		SELECT 
			r.race_id, r.race_key, r.race_date, r.region, r.course_id,
			c.course_name, r.off_time::text, r.race_name, r.race_type, r.class,
			r.pattern, r.rating_band, r.age_band, r.sex_rest,
			r.dist_raw, r.dist_f, r.dist_m, r.going, r.surface, r.ran
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE 1=1
	`

	args := []interface{}{}
	argCount := 0

	if filters.DateFrom != nil {
		argCount++
		query += fmt.Sprintf(" AND r.race_date >= $%d", argCount)
		args = append(args, *filters.DateFrom)
	}

	if filters.DateTo != nil {
		argCount++
		query += fmt.Sprintf(" AND r.race_date <= $%d", argCount)
		args = append(args, *filters.DateTo)
	}

	if filters.Region != nil {
		argCount++
		query += fmt.Sprintf(" AND r.region = $%d", argCount)
		args = append(args, *filters.Region)
	}

	if filters.CourseID != nil {
		argCount++
		query += fmt.Sprintf(" AND r.course_id = $%d", argCount)
		args = append(args, *filters.CourseID)
	}

	if filters.Type != nil {
		argCount++
		query += fmt.Sprintf(" AND r.race_type = $%d", argCount)
		args = append(args, *filters.Type)
	}

	if filters.Class != nil {
		argCount++
		query += fmt.Sprintf(" AND r.class = $%d", argCount)
		args = append(args, *filters.Class)
	}

	if filters.Pattern != nil {
		argCount++
		query += fmt.Sprintf(" AND r.pattern = $%d", argCount)
		args = append(args, *filters.Pattern)
	}

	if filters.Handicap != nil {
		argCount++
		if *filters.Handicap {
			query += fmt.Sprintf(" AND r.race_name ILIKE $%d", argCount)
			args = append(args, "%handicap%")
		} else {
			query += fmt.Sprintf(" AND r.race_name NOT ILIKE $%d", argCount)
			args = append(args, "%handicap%")
		}
	}

	if filters.DistMin != nil {
		argCount++
		query += fmt.Sprintf(" AND r.dist_f >= $%d", argCount)
		args = append(args, *filters.DistMin)
	}

	if filters.DistMax != nil {
		argCount++
		query += fmt.Sprintf(" AND r.dist_f <= $%d", argCount)
		args = append(args, *filters.DistMax)
	}

	if filters.Going != nil {
		argCount++
		query += fmt.Sprintf(" AND r.going ILIKE $%d", argCount)
		args = append(args, "%"+*filters.Going+"%")
	}

	if filters.Surface != nil {
		argCount++
		query += fmt.Sprintf(" AND r.surface = $%d", argCount)
		args = append(args, *filters.Surface)
	}

	if filters.FieldMin != nil {
		argCount++
		query += fmt.Sprintf(" AND r.ran >= $%d", argCount)
		args = append(args, *filters.FieldMin)
	}

	if filters.FieldMax != nil {
		argCount++
		query += fmt.Sprintf(" AND r.ran <= $%d", argCount)
		args = append(args, *filters.FieldMax)
	}

	query += " ORDER BY r.race_date DESC, r.off_time"

	argCount++
	query += fmt.Sprintf(" LIMIT $%d", argCount)
	args = append(args, filters.Limit)

	if filters.Offset > 0 {
		argCount++
		query += fmt.Sprintf(" OFFSET $%d", argCount)
		args = append(args, filters.Offset)
	}

	var races []models.Race
	if err := r.db.Select(&races, query, args...); err != nil {
		return nil, fmt.Errorf("failed to search races: %w", err)
	}

	return races, nil
}

// GetRaceByID returns a single race with runners
func (r *RaceRepository) GetRaceByID(raceID int64) (*models.RaceWithRunners, error) {
	result := &models.RaceWithRunners{}

	// Get race details
	raceQuery := `
		SELECT 
			r.race_id, r.race_key, r.race_date, r.region, r.course_id,
			c.course_name, r.off_time::text, r.race_name, r.race_type, r.class,
			r.pattern, r.rating_band, r.age_band, r.sex_rest,
			r.dist_raw, r.dist_f, r.dist_m, r.going, r.surface, r.ran
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE r.race_id = $1
	`
	if err := r.db.Get(&result.Race, raceQuery, raceID); err != nil {
		return nil, fmt.Errorf("failed to get race: %w", err)
	}

	// Get runners
	runners, err := r.GetRaceRunners(raceID)
	if err != nil {
		return nil, err
	}
	result.Runners = runners

	return result, nil
}

// GetRaceRunners returns all runners for a race
func (r *RaceRepository) GetRaceRunners(raceID int64) ([]models.Runner, error) {
	query := `
		SELECT 
			ru.runner_id, ru.runner_key, ru.race_id, ru.race_date,
			ru.horse_id, h.horse_name,
			ru.trainer_id, t.trainer_name,
			ru.jockey_id, j.jockey_name,
			ru.owner_id, o.owner_name,
			ru.num, ru.pos_raw, ru.pos_num, ru.draw, ru.ovr_btn, ru.btn,
			ru.age, ru.sex, ru.lbs, ru.hg, ru.time_raw, ru.secs, ru.dec,
			ru.prize, ru."or", ru.rpr, ru.comment,
			ru.win_bsp, ru.win_ppwap, ru.win_morningwap, ru.win_ppmax, ru.win_ppmin,
			ru.win_ipmax, ru.win_ipmin, ru.win_morning_vol, ru.win_pre_vol, ru.win_ip_vol, ru.win_lose,
			ru.place_bsp, ru.place_ppwap, ru.place_morningwap, ru.place_ppmax, ru.place_ppmin,
			ru.place_ipmax, ru.place_ipmin, ru.place_morning_vol, ru.place_pre_vol, ru.place_ip_vol, ru.place_win_lose,
			bl.sire, bl.dam, bl.damsire,
			ru.win_flag,
			ru.price_updated_at
		FROM racing.runners ru
		LEFT JOIN racing.horses h ON h.horse_id = ru.horse_id
		LEFT JOIN racing.trainers t ON t.trainer_id = ru.trainer_id
		LEFT JOIN racing.jockeys j ON j.jockey_id = ru.jockey_id
		LEFT JOIN racing.owners o ON o.owner_id = ru.owner_id
		LEFT JOIN racing.bloodlines bl ON bl.blood_id = ru.blood_id
		WHERE ru.race_id = $1
		ORDER BY ru.pos_num NULLS LAST, ru.num
	`

	var runners []models.Runner
	if err := r.db.Select(&runners, query, raceID); err != nil {
		return nil, fmt.Errorf("failed to get runners: %w", err)
	}

	return runners, nil
}

// GetRecentRaces returns recent races
func (r *RaceRepository) GetRecentRaces(date string, limit int) ([]models.Race, error) {
	if limit <= 0 {
		limit = 50
	}

	query := `
		SELECT 
			r.race_id, r.race_key, r.race_date, r.region, r.course_id,
			c.course_name, r.off_time::text, r.race_name, r.race_type, r.class,
			r.pattern, r.dist_raw, r.dist_f, r.going, r.surface, r.ran
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE r.race_date = $1
		ORDER BY r.off_time
		LIMIT $2
	`

	var races []models.Race
	if err := r.db.Select(&races, query, date, limit); err != nil {
		return nil, fmt.Errorf("failed to get recent races: %w", err)
	}

	return races, nil
}

// GetCourseMeetings returns meetings at a course
func (r *RaceRepository) GetCourseMeetings(courseID int64, dateFrom, dateTo string) ([]models.Meeting, error) {
	query := `
		SELECT 
			r.race_date,
			md5(r.race_date::text || '|' || r.region || '|' || c.course_name) AS meeting_key,
			COUNT(*) AS race_count,
			MIN(r.off_time)::text AS first_race,
			MAX(r.off_time)::text AS last_race,
			SUM(r.ran) AS total_runners
		FROM racing.races r
		LEFT JOIN racing.courses c ON c.course_id = r.course_id
		WHERE r.course_id = $1
			AND r.race_date BETWEEN $2 AND $3
		GROUP BY r.race_date, r.region, c.course_name
		ORDER BY r.race_date DESC
	`

	var meetings []models.Meeting
	if err := r.db.Select(&meetings, query, courseID, dateFrom, dateTo); err != nil {
		return nil, fmt.Errorf("failed to get meetings: %w", err)
	}

	return meetings, nil
}

// GetCourses returns all courses
func (r *RaceRepository) GetCourses() ([]models.Course, error) {
	query := `SELECT course_id, course_name, region FROM racing.courses ORDER BY course_name`

	var courses []models.Course
	if err := r.db.Select(&courses, query); err != nil {
		return nil, fmt.Errorf("failed to get courses: %w", err)
	}

	return courses, nil
}

// GetRacesByMeetings returns races grouped by meetings (course + date)
func (r *RaceRepository) GetRacesByMeetings(date string) ([]models.MeetingWithRaces, error) {
	// Get all races for the date
	races, err := r.GetRecentRaces(date, 1000) // Get all races for this date
	if err != nil {
		return nil, err
	}

	// Group races by course
	meetingsMap := make(map[int64]*models.MeetingWithRaces)

	for _, race := range races {
		// Create a key for this meeting (course_id)
		var courseKey int64
		if race.CourseID != nil {
			courseKey = *race.CourseID
		} else {
			courseKey = -1 // Unknown course
		}

		// Create meeting if it doesn't exist
		if _, exists := meetingsMap[courseKey]; !exists {
			meetingsMap[courseKey] = &models.MeetingWithRaces{
				RaceDate:   race.RaceDate,
				Region:     race.Region,
				CourseID:   race.CourseID,
				CourseName: race.CourseName,
				Races:      []models.Race{},
			}
		}

		// Add race to meeting
		meeting := meetingsMap[courseKey]
		
		// Fetch runners for this race
		runners, err := r.GetRaceRunners(race.RaceID)
		if err == nil {
			race.Runners = runners
		}
		
		meeting.Races = append(meeting.Races, race)
	}

	// Convert map to slice and calculate meeting stats
	meetings := make([]models.MeetingWithRaces, 0, len(meetingsMap))
	for _, meeting := range meetingsMap {
		meeting.RaceCount = len(meeting.Races)

		// Look up course name from courses table if missing
		if meeting.CourseName == nil && meeting.CourseID != nil {
			var courseName string
			err := r.db.Get(&courseName, `
				SELECT course_name FROM racing.courses WHERE course_id = $1
			`, *meeting.CourseID)
			if err == nil {
				meeting.CourseName = &courseName
			}
		}

		// Find first and last race times
		var firstTime, lastTime string
		raceTypes := make(map[string]bool)

		for i, race := range meeting.Races {
			if race.OffTime != nil && *race.OffTime != "" {
				if i == 0 || *race.OffTime < firstTime {
					firstTime = *race.OffTime
				}
				if *race.OffTime > lastTime {
					lastTime = *race.OffTime
				}
			}
			raceTypes[race.RaceType] = true
		}

		if firstTime != "" {
			meeting.FirstRaceTime = &firstTime
			meeting.LastRaceTime = &lastTime
		}

		// Build race types string
		types := make([]string, 0, len(raceTypes))
		for t := range raceTypes {
			types = append(types, t)
		}
		if len(types) > 0 {
			typesStr := strings.Join(types, ", ")
			meeting.RaceTypes = &typesStr
		}

		meetings = append(meetings, *meeting)
	}

	// Sort meetings by first race time
	// Sort by course name for now (simpler)
	return meetings, nil
}

// buildWhereClause is a helper to build dynamic WHERE clauses
func buildWhereClause(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(conditions, " AND ")
}
