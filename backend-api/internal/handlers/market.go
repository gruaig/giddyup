package handlers

import (
	"net/http"
	"strconv"
	"time"

	"giddyup/api/internal/logger"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

type MarketHandler struct {
	repo *repository.MarketRepository
}

func NewMarketHandler(repo *repository.MarketRepository) *MarketHandler {
	return &MarketHandler{repo: repo}
}

// GetMarketMovers returns steamers and drifters
// GET /api/v1/market/movers?date=2024-01-13&min_move=20&type=steamer
func (h *MarketHandler) GetMarketMovers(c *gin.Context) {
	date := c.DefaultQuery("date", time.Now().Format("2006-01-02"))
	minMove := 20.0
	if m := c.Query("min_move"); m != "" {
		if parsed, err := strconv.ParseFloat(m, 64); err == nil {
			minMove = parsed
		}
	}
	moveType := c.DefaultQuery("type", "all")

	logger.Debug("GetMarketMovers: date=%s, minMove=%.1f, type=%s", date, minMove, moveType)

	movers, err := h.repo.GetMarketMovers(date, minMove, moveType)
	if err != nil {
		logger.HandlerError("MarketHandler", "GetMarketMovers", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get market movers",
		})
		return
	}

	logger.Debug("GetMarketMovers: found %d movers", len(movers))
	c.JSON(http.StatusOK, movers)
}

// GetWinCalibration returns win market calibration
// GET /api/v1/market/calibration/win?date_from=2024-01-01&date_to=2024-12-31
func (h *MarketHandler) GetWinCalibration(c *gin.Context) {
	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))

	calibration, err := h.repo.GetWinCalibration(dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get calibration data",
		})
		return
	}

	c.JSON(http.StatusOK, calibration)
}

// GetPlaceCalibration returns place market calibration
// GET /api/v1/market/calibration/place?date_from=2024-01-01&date_to=2024-12-31
func (h *MarketHandler) GetPlaceCalibration(c *gin.Context) {
	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(-1, 0, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))

	calibration, err := h.repo.GetPlaceCalibration(dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get place calibration data",
		})
		return
	}

	c.JSON(http.StatusOK, calibration)
}

// GetInPlayMoves returns in-play price movements
// GET /api/v1/market/inplay-moves?date_from=2024-01-01&date_to=2024-12-31&min_move=20
func (h *MarketHandler) GetInPlayMoves(c *gin.Context) {
	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))
	minMove := 20.0
	if m := c.Query("min_move"); m != "" {
		if parsed, err := strconv.ParseFloat(m, 64); err == nil {
			minMove = parsed
		}
	}

	moves, err := h.repo.GetInPlayMoves(dateFrom, dateTo, minMove)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get in-play moves",
		})
		return
	}

	c.JSON(http.StatusOK, moves)
}

// GetBookVsExchange compares SP vs BSP
// GET /api/v1/market/book-vs-exchange?date_from=2024-01-01&date_to=2024-12-31
func (h *MarketHandler) GetBookVsExchange(c *gin.Context) {
	dateFrom := c.DefaultQuery("date_from", time.Now().AddDate(0, -1, 0).Format("2006-01-02"))
	dateTo := c.DefaultQuery("date_to", time.Now().Format("2006-01-02"))

	comparison, err := h.repo.GetBookVsExchange(dateFrom, dateTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get book vs exchange comparison",
		})
		return
	}

	c.JSON(http.StatusOK, comparison)
}
