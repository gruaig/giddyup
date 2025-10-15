package middleware

import (
	"net/http"
	"time"

	"giddyup/api/internal/logger"

	"github.com/gin-gonic/gin"
)

// ErrorHandler middleware for handling panics and errors
func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("Panic recovered: %v", err)
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// Logger middleware for request logging
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method
		ip := c.ClientIP()

		// Process request
		c.Next()

		// Log after processing
		duration := time.Since(start)
		status := c.Writer.Status()

		logger.Request(method, path, ip, duration, status)
	}
}
