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

type BiasHandler struct {
	repo *repository.BiasRepository
}

func NewBiasHandler(repo *repository.BiasRepository) *BiasHandler {
	return &BiasHandler{repo: repo}
}

// GetDrawBias returns draw bias statistics
// GET /api/v1/bias/draw?course_id=1&dist_min=5&dist_max=7&going=Good&min_runners=10
func (h *BiasHandler) GetDrawBias(c *gin.Context) {
	var params models.DrawBiasParams
	if err := c.ShouldBindQuery(&params); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if params.MinRunners <= 0 {
		params.MinRunners = 10
	}

	results, err := h.repo.GetDrawBias(params)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get draw bias data",
		})
		return
	}

	c.JSON(http.StatusOK, results)
}

// GetRecencyEffects returns days-since-run statistics
// GET /api/v1/analysis/recency?date_from=2024-01-01&date_to=2024-12-31
func (h *BiasHandler) GetRecencyEffects(c *gin.Context) {
	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))

	effects, err := h.repo.GetRecencyEffects(dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get recency effects",
		})
		return
	}

	c.JSON(http.StatusOK, effects)
}

// GetTrainerChanges returns trainer change impact analysis
// GET /api/v1/analysis/trainer-change?min_runs=5
func (h *BiasHandler) GetTrainerChanges(c *gin.Context) {
	start := time.Now()
	minRuns := 5
	if m := c.Query("min_runs"); m != "" {
		if parsed, err := strconv.Atoi(m); err == nil {
			minRuns = parsed
		}
	}

	logger.Info("→ GetTrainerChanges: min_runs=%d | IP: %s", minRuns, c.ClientIP())

	changes, err := h.repo.GetTrainerChanges(minRuns)
	if err != nil {
		logger.Error("GetTrainerChanges: repository error: %v | min_runs=%d", err, minRuns)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get trainer change data",
		})
		return
	}

	duration := time.Since(start)
	logger.Info("← GetTrainerChanges: %d changes | %v", len(changes), duration)
	c.JSON(http.StatusOK, changes)
}
