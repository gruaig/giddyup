package handlers

import (
	"net/http"

	"giddyup/api/internal/logger"
	"giddyup/api/internal/models"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

type AngleHandler struct {
	repo *repository.AngleRepository
}

func NewAngleHandler(repo *repository.AngleRepository) *AngleHandler {
	return &AngleHandler{repo: repo}
}

// GetNearMissTodayQualifiers returns today's qualifiers for near-miss-no-hike angle
// GET /api/v1/angles/near-miss-no-hike/today
func (h *AngleHandler) GetNearMissTodayQualifiers(c *gin.Context) {
	var params models.NearMissTodayParams
	if err := c.ShouldBindQuery(&params); err != nil {
		logger.Warn("NearMissToday: invalid params: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Set default for same_surface if not explicitly set
	if c.Query("same_surface") == "" {
		params.SameSurface = true
	}

	logger.Debug("NearMissToday: params=%+v", params)

	qualifiers, err := h.repo.GetNearMissTodayQualifiers(params)
	if err != nil {
		logger.HandlerError("AngleHandler", "GetNearMissTodayQualifiers", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get today's qualifiers",
		})
		return
	}

	logger.Debug("NearMissToday: found %d qualifiers", len(qualifiers))
	c.JSON(http.StatusOK, qualifiers)
}

// GetNearMissPastCases returns historical backtest for near-miss-no-hike angle
// GET /api/v1/angles/near-miss-no-hike/past
func (h *AngleHandler) GetNearMissPastCases(c *gin.Context) {
	var params models.NearMissPastParams
	if err := c.ShouldBindQuery(&params); err != nil {
		logger.Warn("NearMissPast: invalid params: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	// Set defaults
	if c.Query("same_surface") == "" {
		params.SameSurface = true
	}
	if c.Query("summary") == "" {
		params.Summary = true
	}

	logger.Debug("NearMissPast: params=%+v", params)

	result, err := h.repo.GetNearMissPastCases(params)
	if err != nil {
		logger.HandlerError("AngleHandler", "GetNearMissPastCases", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get past cases",
		})
		return
	}

	logger.Debug("NearMissPast: found %d cases", len(result.Cases))
	if result.Summary != nil {
		logger.Debug("NearMissPast: summary - N=%d, Wins=%d, WinRate=%.2f%%, ROI=%.2f%%",
			result.Summary.N, result.Summary.Wins, result.Summary.WinRate*100, *result.Summary.ROI*100)
	}

	c.JSON(http.StatusOK, result)
}

