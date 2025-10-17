package handlers

import (
	"net/http"
	"strconv"
	"time"

	"giddyup/api/internal/logger"
	"giddyup/api/internal/models"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

type RaceHandler struct {
	repo *repository.RaceRepository
}

func NewRaceHandler(repo *repository.RaceRepository) *RaceHandler {
	return &RaceHandler{repo: repo}
}

// SearchRaces handles race search with filters
// GET /api/v1/races/search
func (h *RaceHandler) SearchRaces(c *gin.Context) {
	start := time.Now()
	logger.Info("→ SearchRaces request from %s | Query: %s", c.ClientIP(), c.Request.URL.RawQuery)

	var filters models.RaceFilters
	if err := c.ShouldBindQuery(&filters); err != nil {
		logger.Warn("SearchRaces: invalid filters: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Support single 'date' parameter (sets both date_from and date_to)
	if date := c.Query("date"); date != "" {
		if filters.DateFrom == nil {
			filters.DateFrom = &date
		}
		if filters.DateTo == nil {
			filters.DateTo = &date
		}
	}

	if filters.Limit <= 0 {
		filters.Limit = 100
	}
	// Cap limit to reasonable maximum to prevent performance issues
	if filters.Limit > 1000 {
		filters.Limit = 1000
	}

	logger.Debug("SearchRaces: filters=%+v", filters)

	races, err := h.repo.SearchRaces(filters)
	if err != nil {
		logger.Error("SearchRaces: repository error: %v | Filters: %+v", err, filters)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to search races",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← SearchRaces: %d races | %v", len(races), duration)
	c.JSON(http.StatusOK, races)
}

// GetRace returns a single race with runners
// GET /api/v1/races/:id
func (h *RaceHandler) GetRace(c *gin.Context) {
	start := time.Now()
	raceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetRace: invalid race ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid race ID",
		})
		return
	}

	logger.Info("→ GetRace: race_id=%d | IP: %s", raceID, c.ClientIP())

	race, err := h.repo.GetRaceByID(raceID)
	if err != nil {
		logger.Error("GetRace: repository error for race_id=%d: %v", raceID, err)
		c.JSON(http.StatusNotFound, gin.H{
			"error": "race not found",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetRace: race_id=%d, %d runners | %v", raceID, len(race.Runners), duration)
	c.JSON(http.StatusOK, race)
}

// GetRaceRunners returns runners for a race
// GET /api/v1/races/:id/runners
func (h *RaceHandler) GetRaceRunners(c *gin.Context) {
	start := time.Now()
	raceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetRaceRunners: invalid race ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid race ID",
		})
		return
	}

	logger.Info("→ GetRaceRunners: race_id=%d | IP: %s", raceID, c.ClientIP())

	runners, err := h.repo.GetRaceRunners(raceID)
	if err != nil {
		logger.Error("GetRaceRunners: repository error for race_id=%d: %v", raceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get runners",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetRaceRunners: race_id=%d, %d runners | %v", raceID, len(runners), duration)
	c.JSON(http.StatusOK, runners)
}

// GetRecentRaces returns races for a specific date
// GET /api/v1/races?date=2024-01-13&limit=50
func (h *RaceHandler) GetRecentRaces(c *gin.Context) {
	start := time.Now()
	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsedLimit, err := strconv.Atoi(l); err == nil {
			limit = parsedLimit
		}
	}

	logger.Info("→ GetRecentRaces: date=%s, limit=%d | IP: %s", date, limit, c.ClientIP())

	races, err := h.repo.GetRecentRaces(date, limit)
	if err != nil {
		logger.Error("GetRecentRaces: repository error for date=%s: %v", date, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get races",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetRecentRaces: %d races on %s | %v", len(races), date, duration)
	c.JSON(http.StatusOK, races)
}

// GetCourseMeetings returns meetings at a course
// GET /api/v1/courses/:id/meetings?date_from=2024-01-01&date_to=2024-12-31
func (h *RaceHandler) GetCourseMeetings(c *gin.Context) {
	start := time.Now()
	courseID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetCourseMeetings: invalid course ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid course ID",
		})
		return
	}

	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))

	logger.Info("→ GetCourseMeetings: course_id=%d, from=%s, to=%s | IP: %s", courseID, dateFrom, dateTo, c.ClientIP())

	meetings, err := h.repo.GetCourseMeetings(courseID, dateFrom, dateTo)
	if err != nil {
		logger.Error("GetCourseMeetings: repository error for course_id=%d: %v", courseID, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get meetings",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetCourseMeetings: %d meetings | %v", len(meetings), duration)
	c.JSON(http.StatusOK, meetings)
}

// GetMeetings returns races grouped by meetings (course + date)
// GET /api/v1/meetings
func (h *RaceHandler) GetMeetings(c *gin.Context) {
	start := time.Now()
	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))

	logger.Info("→ GetMeetings: date=%s | IP: %s", date, c.ClientIP())

	meetings, err := h.repo.GetRacesByMeetings(date)
	if err != nil {
		logger.Error("GetMeetings: repository error: %v | Date: %s", err, date)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get meetings",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetMeetings: %d meetings, %d total races | %v",
		len(meetings),
		func() int {
			count := 0
			for _, m := range meetings {
				count += len(m.Races)
			}
			return count
		}(),
		duration)

	c.JSON(http.StatusOK, meetings)
}

// GetCourses returns all courses
// GET /api/v1/courses
func (h *RaceHandler) GetCourses(c *gin.Context) {
	start := time.Now()
	logger.Info("→ GetCourses | IP: %s", c.ClientIP())

	courses, err := h.repo.GetCourses()
	if err != nil {
		logger.Error("GetCourses: repository error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get courses",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetCourses: %d courses | %v", len(courses), duration)

	logger.Debug("GetCourses: found %d courses", len(courses))
	c.JSON(http.StatusOK, courses)
}

// GetTodayMeetings returns today's meetings (convenience endpoint)
func (h *RaceHandler) GetTodayMeetings(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	
	// Set date param and call existing GetMeetings logic
	c.Request.URL.RawQuery = "date=" + today
	c.Params = append(c.Params, gin.Param{Key: "date", Value: today})
	
	// Reuse existing logic
	h.GetMeetings(c)
}

// GetTomorrowMeetings returns tomorrow's meetings (convenience endpoint)
func (h *RaceHandler) GetTomorrowMeetings(c *gin.Context) {
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	
	// Set date param and call existing GetMeetings logic
	c.Request.URL.RawQuery = "date=" + tomorrow
	c.Params = append(c.Params, gin.Param{Key: "date", Value: tomorrow})
	
	// Reuse existing logic
	h.GetMeetings(c)
}
