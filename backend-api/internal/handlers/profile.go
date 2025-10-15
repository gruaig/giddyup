package handlers

import (
	"net/http"
	"strconv"

	"giddyup/api/internal/logger"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

type ProfileHandler struct {
	repo *repository.ProfileRepository
}

func NewProfileHandler(repo *repository.ProfileRepository) *ProfileHandler {
	return &ProfileHandler{repo: repo}
}

// GetHorseProfile returns complete horse profile
// GET /api/v1/horses/:id/profile
func (h *ProfileHandler) GetHorseProfile(c *gin.Context) {
	horseID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetHorseProfile: invalid horse ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid horse ID",
		})
		return
	}

	logger.Debug("GetHorseProfile: horse_id=%d", horseID)

	profile, err := h.repo.GetHorseProfile(horseID)
	if err != nil {
		logger.HandlerError("ProfileHandler", "GetHorseProfile", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get horse profile",
		})
		return
	}

	logger.Debug("GetHorseProfile: loaded profile for %s (%d runs)", profile.Horse.HorseName, profile.CareerSummary.Runs)
	c.JSON(http.StatusOK, profile)
}

// GetTrainerProfile returns complete trainer profile
// GET /api/v1/trainers/:id/profile
func (h *ProfileHandler) GetTrainerProfile(c *gin.Context) {
	trainerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetTrainerProfile: invalid trainer ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid trainer ID",
		})
		return
	}

	logger.Debug("GetTrainerProfile: trainer_id=%d", trainerID)

	profile, err := h.repo.GetTrainerProfile(trainerID)
	if err != nil {
		logger.HandlerError("ProfileHandler", "GetTrainerProfile", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get trainer profile",
		})
		return
	}

	logger.Debug("GetTrainerProfile: loaded profile for %s", profile.Trainer.TrainerName)
	c.JSON(http.StatusOK, profile)
}

// GetJockeyProfile returns complete jockey profile
// GET /api/v1/jockeys/:id/profile
func (h *ProfileHandler) GetJockeyProfile(c *gin.Context) {
	jockeyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		logger.Warn("GetJockeyProfile: invalid jockey ID: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid jockey ID",
		})
		return
	}

	logger.Debug("GetJockeyProfile: jockey_id=%d", jockeyID)

	profile, err := h.repo.GetJockeyProfile(jockeyID)
	if err != nil {
		logger.HandlerError("ProfileHandler", "GetJockeyProfile", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get jockey profile",
		})
		return
	}

	logger.Debug("GetJockeyProfile: loaded profile for %s", profile.Jockey.JockeyName)
	c.JSON(http.StatusOK, profile)
}
