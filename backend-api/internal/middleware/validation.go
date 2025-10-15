package middleware

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ValidatePagination caps limit to max 1000 and ensures minimum of 1
func ValidatePagination() gin.HandlerFunc {
	return func(c *gin.Context) {
		if limitStr := c.Query("limit"); limitStr != "" {
			limit, err := strconv.Atoi(limitStr)
			if err != nil || limit < 1 {
				limit = 50
			}
			if limit > 1000 {
				limit = 1000
			}
			c.Set("validated_limit", limit)
		}
		c.Next()
	}
}

// ValidateDateParams validates date query parameters format (YYYY-MM-DD)
func ValidateDateParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		dateParams := []string{"date", "date_from", "date_to", "on"}

		for _, param := range dateParams {
			if dateStr := c.Query(param); dateStr != "" && dateStr != "today" {
				if _, err := time.Parse("2006-01-02", dateStr); err != nil {
					c.JSON(400, gin.H{
						"error": fmt.Sprintf("invalid %s format, use YYYY-MM-DD", param),
						"field": param,
					})
					c.Abort()
					return
				}
			}
		}
		c.Next()
	}
}
