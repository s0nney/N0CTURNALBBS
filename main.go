package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"N0CTURNALBBS/internal/config"
	database "N0CTURNALBBS/internal/db"
	"N0CTURNALBBS/internal/handlers"
	"N0CTURNALBBS/internal/middleware"
	"N0CTURNALBBS/internal/templates"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	csrf "github.com/utrack/gin-csrf"
)

func main() {
	// Set to release mode when running in production
	// gin.SetMode(gin.ReleaseMode)

	// Load db config and connect
	dbConfig, err := config.LoadDBConfig(filepath.Join("config", "db.yaml"))
	if err != nil {
		log.Fatalf("Failed to load database configuration: %v", err)
	}

	db, err := database.InitDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB(db)

	// Load board config and generate
	cfg, err := config.LoadConfig(filepath.Join("config", "boards.yaml"))
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := config.InitializeBoards(cfg, db); err != nil {
		log.Fatalf("Failed to initialize boards: %v", err)
	}

	// Site descriptions
	siteConfig, err := config.LoadSiteConfig(filepath.Join("config", "site.yaml"))
	if err != nil {
		log.Fatalf("Failed to load site config: %v", err)
	}

	// Load moderator config
	modConfig, err := config.LoadModConfig(filepath.Join("config", "mod.yaml"))
	if err != nil {
		log.Fatalf("Failed to load moderator configuration: %v", err)
	}

	// Initialize moderators in database
	if err := config.InitializeModerators(modConfig, db); err != nil {
		log.Fatalf("Failed to initialize moderators: %v", err)
	}

	// Important session keys
	sessionConfig, err := config.LoadSessionConfig(filepath.Join("config", "session_security.yaml"))
	if err != nil {
		log.Fatalf("Failed to load session config: %v", err)
	}

	// Load rate limit config
	rateConf, err := config.LoadRateLimitConfig(filepath.Join("config", "rate_limit.yaml"))
	if err != nil {
		log.Fatalf("Failed to load rate limit config: %v", err)
	}

	// Initialize router
	router := gin.Default()
	store := cookie.NewStore([]byte(sessionConfig.SecretKey))
	store.Options(sessions.Options{
		Path:     "/",
		MaxAge:   sessionConfig.MaxAge,
		Secure:   sessionConfig.Secure,
		HttpOnly: sessionConfig.HTTPOnly,
		SameSite: sessionConfig.SameSiteMode(),
	})

	router.Use(sessions.Sessions(sessionConfig.CookieName, store))

	router.Use(csrf.Middleware(csrf.Options{
		Secret: sessionConfig.CSRF.Secret,
		ErrorFunc: func(c *gin.Context) {
			if strings.Contains(c.GetHeader("Accept"), "application/json") {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   "invalid_csrf_token",
					"message": sessionConfig.CSRF.ErrorMsg,
				})
			} else {
				c.HTML(http.StatusForbidden, "error.html", gin.H{
					"Error":       sessionConfig.CSRF.ErrorMsg,
					"Site":        siteConfig,
					"CurrentYear": time.Now().Year(),
				})
			}
			c.Abort()
		},
	}))

	router.Use(middleware.Logger())

	// Serve static files
	router.Static("/static", "./static")
	router.StaticFile("/styles.css", "./static/css/styles.css")
	router.StaticFile("/font.css", "./static/css/font.css")
	router.StaticFile("/favicon.ico", "./favicon.ico")
	router.StaticFile("/robots.txt", "./robots.txt")

	router.StaticFile("/ChakraPetch-Regular.ttf", "./static/fonts/ChakraPetch-Regular.ttf")
	router.StaticFile("/ChakraPetch-Italic.ttf", "./static/fonts/ChakraPetch-Italic.ttf")
	router.StaticFile("/ChakraPetch-Light.ttf", "./static/fonts/ChakraPetch-Light.ttf")
	router.StaticFile("/ChakraPetch-LightItalic.ttf", "./static/fonts/ChakraPetch-LightItalic.ttf")
	router.StaticFile("/ChakraPetch-Medium.ttf", "./static/fonts/ChakraPetch-Medium.ttf")
	router.StaticFile("/ChakraPetch-MediumItalic.ttf", "./static/fonts/ChakraPetch-MediumItalic.ttf")
	router.StaticFile("/ChakraPetch-SemiBold.ttf", "./static/fonts/ChakraPetch-SemiBold.ttf")
	router.StaticFile("/ChakraPetch-SemiBoldItalic.ttf", "./static/fonts/ChakraPetch-SemiBoldItalic.ttf")
	router.StaticFile("/ChakraPetch-Bold.ttf", "./static/fonts/ChakraPetch-Bold.ttf")
	router.StaticFile("/ChakraPetch-BoldItalic.ttf", "./static/fonts/ChakraPetch-BoldItalic.ttf")

	router.HTMLRender = templates.CreateRenderer("internal/templates/*.html")

	// Moderator routes group
	modRoutes := router.Group("/mod")
	{
		modRoutes.GET("/login", handlers.ModLoginHandler(db, modConfig, siteConfig))
		modRoutes.POST("/login", middleware.RateLimiter(rateConf, siteConfig), middleware.CaptchaRequired(siteConfig), handlers.ModLoginHandler(db, modConfig, siteConfig))

		authorized := modRoutes.Group("")
		authorized.Use(middleware.AuthRequired(db, modConfig))
		{
			// Logout
			authorized.GET("/logout", handlers.ModLogoutHandler(db, modConfig))

			// Dashboard
			authorized.GET("/dashboard", handlers.ModDashboardHandler(db, siteConfig))

			// Thread management
			authorized.GET("/threads", handlers.ModThreadsHandler(db, siteConfig))
			authorized.GET("/threads/:id/posts", handlers.ModPostsHandler(db, siteConfig))
			authorized.POST("/threads/:id/delete", handlers.DeleteThreadHandler(db))
			authorized.POST("/threads/:id/lock", handlers.LockThreadHandler(db))

			//filters
			authorized.GET("/filters", handlers.ModWordFiltersHandler(db, siteConfig))
			authorized.POST("/filters/add", handlers.ModAddBannedWordHandler(db, siteConfig))
			authorized.POST("/filters/delete/:id", handlers.ModDeleteBannedWordHandler(db, siteConfig))

			// Post management
			authorized.POST("/posts/:id/delete", handlers.DeletePostHandler(db))

			// Action history
			authorized.GET("/actions", handlers.ModActionsHandler(db, siteConfig))
		}
	}

	router.GET("/", handlers.HomeHandler(db, siteConfig))
	router.GET("/board/:slug", handlers.BoardHandler(db, siteConfig))
	router.GET("/thread/:id", handlers.ThreadHandler(db, siteConfig))
	router.GET("/captcha", handlers.GenerateCaptcha)

	// Thread and reply creation
	router.POST("/thread/new/:board_id", middleware.RateLimiter(rateConf, siteConfig), middleware.CaptchaRequired(siteConfig), handlers.NewThreadHandler(db, siteConfig))
	router.POST("/post/new/:thread_id", middleware.RateLimiter(rateConf, siteConfig), middleware.CaptchaRequired(siteConfig), handlers.NewPostHandler(db, siteConfig))

	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Session security configured with: %+v", sessionConfig)

	// Start up the engine
	log.Printf("Starting server on :%s", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
