package router

import (
	"giddyup/api/internal/database"
	"giddyup/api/internal/handlers"
	"giddyup/api/internal/middleware"
	"giddyup/api/internal/repository"

	"github.com/gin-gonic/gin"
)

func Setup(db *database.DB, corsOrigins []string) *gin.Engine {
	// Set Gin mode
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()

	// Global middleware
	r.Use(middleware.Logger())
	r.Use(middleware.ErrorHandler())
	r.Use(middleware.CORS(corsOrigins))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		if err := db.Health(); err != nil {
			c.JSON(500, gin.H{"status": "unhealthy", "error": err.Error()})
			return
		}
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// Initialize repositories
	searchRepo := repository.NewSearchRepository(db)
	profileRepo := repository.NewProfileRepository(db)
	raceRepo := repository.NewRaceRepository(db)
	marketRepo := repository.NewMarketRepository(db)
	biasRepo := repository.NewBiasRepository(db)
	angleRepo := repository.NewAngleRepository(db)

	// Initialize handlers
	searchHandler := handlers.NewSearchHandler(searchRepo)
	profileHandler := handlers.NewProfileHandler(profileRepo)
	raceHandler := handlers.NewRaceHandler(raceRepo)
	marketHandler := handlers.NewMarketHandler(marketRepo)
	biasHandler := handlers.NewBiasHandler(biasRepo)
	angleHandler := handlers.NewAngleHandler(angleRepo)
	adminHandler := handlers.NewAdminHandler(db.DB)

	// API v1 routes
	v1 := r.Group("/api/v1")
	v1.Use(middleware.ValidatePagination())
	v1.Use(middleware.ValidateDateParams())
	{
		// Search endpoints
		search := v1.Group("/search")
		{
			search.GET("", searchHandler.GlobalSearch)
			search.GET("/comments", searchHandler.SearchComments)
		}

		// Horse endpoints
		horses := v1.Group("/horses")
		{
			horses.GET("/:id/profile", profileHandler.GetHorseProfile)
		}

		// Trainer endpoints
		trainers := v1.Group("/trainers")
		{
			trainers.GET("/:id/profile", profileHandler.GetTrainerProfile)
		}

		// Jockey endpoints
		jockeys := v1.Group("/jockeys")
		{
			jockeys.GET("/:id/profile", profileHandler.GetJockeyProfile)
		}

		// Race endpoints
		races := v1.Group("/races")
		{
			races.GET("", raceHandler.GetRecentRaces)
			races.GET("/search", raceHandler.SearchRaces)
			races.GET("/:id", raceHandler.GetRace)
			races.GET("/:id/runners", raceHandler.GetRaceRunners)
		}

		// Course endpoints
		courses := v1.Group("/courses")
		{
			courses.GET("", raceHandler.GetCourses)
			courses.GET("/:id/meetings", raceHandler.GetCourseMeetings)
		}

		// Meetings endpoint - races grouped by venue
		v1.GET("/meetings", raceHandler.GetMeetings)

		// Market endpoints
		market := v1.Group("/market")
		{
			market.GET("/movers", marketHandler.GetMarketMovers)
			market.GET("/calibration/win", marketHandler.GetWinCalibration)
			market.GET("/calibration/place", marketHandler.GetPlaceCalibration)
			market.GET("/inplay-moves", marketHandler.GetInPlayMoves)
			market.GET("/book-vs-exchange", marketHandler.GetBookVsExchange)
		}

		// Bias endpoints
		bias := v1.Group("/bias")
		{
			bias.GET("/draw", biasHandler.GetDrawBias)
		}

		// Analysis endpoints
		analysis := v1.Group("/analysis")
		{
			analysis.GET("/recency", biasHandler.GetRecencyEffects)
			analysis.GET("/trainer-change", biasHandler.GetTrainerChanges)
		}

		// Angles endpoints (betting strategies)
		angles := v1.Group("/angles")
		{
			nearMiss := angles.Group("/near-miss-no-hike")
			{
				nearMiss.GET("/today", angleHandler.GetNearMissTodayQualifiers)
				nearMiss.GET("/past", angleHandler.GetNearMissPastCases)
			}
		}

		// Admin endpoints (data management)
		admin := v1.Group("/admin")
		{
			scrape := admin.Group("/scrape")
			{
				scrape.POST("/yesterday", adminHandler.ScrapeYesterday)
				scrape.POST("/date", adminHandler.ScrapeDate)
			}
			admin.GET("/status", adminHandler.GetUpdateStatus)
			admin.GET("/gaps", adminHandler.DetectGaps)
		}
	}

	// Handle 404 - return JSON instead of HTML
	r.NoRoute(func(c *gin.Context) {
		c.JSON(404, gin.H{
			"error": "endpoint not found",
			"path":  c.Request.URL.Path,
		})
	})

	return r
}
