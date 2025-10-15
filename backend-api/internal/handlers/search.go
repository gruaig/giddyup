package handlers

import (
	"fmt"
	"net/http"

	"giddyup/api/internal/logger"
	"giddyup/api/internal/models"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	repo *repository.SearchRepository
}

func NewSearchHandler(repo *repository.SearchRepository) *SearchHandler {
	return &SearchHandler{repo: repo}
}

// GlobalSearch handles global search across all entities
// GET /api/v1/search?q=<query>&limit=<limit>
func (h *SearchHandler) GlobalSearch(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		logger.Warn("GlobalSearch: invalid query parameter: '%s'", query)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "query parameter 'q' is required",
		})
		return
	}

	limit := 10
	if l := c.Query("limit"); l != "" {
		if _, err := fmt.Sscanf(l, "%d", &limit); err != nil {
			limit = 10
		}
	}

	logger.Debug("GlobalSearch: query='%s', limit=%d", query, limit)

	results, err := h.repo.GlobalSearch(query, limit)
	if err != nil {
		logger.HandlerError("SearchHandler", "GlobalSearch", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to perform search",
		})
		return
	}

	logger.Debug("GlobalSearch: found %d total results", results.Total)
	c.JSON(http.StatusOK, results)
}

// SearchComments handles full-text search in runner comments
// GET /api/v1/search/comments?q=<query>&date_from=<date>&date_to=<date>&region=<region>
func (h *SearchHandler) SearchComments(c *gin.Context) {
	var params models.CommentSearchParams
	if err := c.ShouldBindQuery(&params); err != nil {
		logger.Warn("SearchComments: invalid params: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	if params.Limit <= 0 {
		params.Limit = 100
	}

	logger.Debug("SearchComments: query='%s', limit=%d", params.Query, params.Limit)

	results, err := h.repo.SearchComments(params)
	if err != nil {
		logger.HandlerError("SearchHandler", "SearchComments", err, 500)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to search comments",
		})
		return
	}

	logger.Debug("SearchComments: found %d results", len(results))
	c.JSON(http.StatusOK, results)
}
