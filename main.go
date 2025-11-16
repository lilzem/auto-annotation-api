package main

import (
	"auto-annotation-api/config"
	"auto-annotation-api/database"
	"auto-annotation-api/handlers"
	"auto-annotation-api/middleware"
	"auto-annotation-api/services"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Initialize configuration
	cfg := config.Load()

	// Initialize database connection
	db, err := database.Connect(cfg.MongoURI, cfg.DatabaseName)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer database.Disconnect()

	log.Println("MongoDB connected successfully!")
	log.Printf("Database: %s", cfg.DatabaseName)

	// Set Gin mode
	gin.SetMode(cfg.GinMode)

	// Initialize router
	router := gin.Default()

	// Initialize AWS service (if configured)
	var awsService *services.AWSService
	if cfg.AWSAccessKeyID != "" && cfg.AWSSecretKey != "" && cfg.AWSS3BucketName != "" {
		var err error
		awsService, err = services.NewAWSService(
			cfg.AWSAccessKeyID,
			cfg.AWSSecretKey,
			cfg.AWSRegion,
			cfg.AWSS3BucketName,
			cfg.AWSPollyVoiceID,
			cfg.AWSPollyEngine,
		)
		if err != nil {
			log.Printf("Warning: Failed to initialize AWS service: %v", err)
			log.Println("TTS functionality will not be available")
		} else {
			log.Println("AWS service initialized successfully (S3 + Polly)")
		}
	} else {
		log.Println("AWS credentials not configured. TTS functionality will not be available")
	}

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db)
	annotationHandler := handlers.NewAnnotationHandler(db, cfg.OllamaBaseURL, cfg.OllamaModel, cfg.UploadDir, awsService)

	// Basic route
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Auto Annotation API",
			"status":  "connected to MongoDB",
			"database": cfg.DatabaseName,
		})
	})

	// Auth routes (public)
	authRoutes := router.Group("/auth")
	{
		authRoutes.POST("/register", authHandler.Register)
		authRoutes.POST("/login", authHandler.Login)
	}

	// Protected routes (require authentication)
	protectedRoutes := router.Group("/auth")
	protectedRoutes.Use(middleware.AuthMiddleware(db))
	{
		protectedRoutes.GET("/profile", authHandler.GetProfile)
	}

	// Annotation routes - viewing is available to all authenticated users
	annotationRoutes := router.Group("/annotations")
	annotationRoutes.Use(middleware.AuthMiddleware(db))
	{
		// Public viewing (any authenticated user)
		annotationRoutes.GET("", annotationHandler.GetAllAnnotations)
		annotationRoutes.GET("/:id", annotationHandler.GetAnnotation)
		annotationRoutes.GET("/:id/audio", annotationHandler.DownloadAudio) // Deprecated - kept for backward compatibility
	}

	// Annotation creation/modification routes (content creators only)
	annotationCreatorRoutes := router.Group("/annotations")
	annotationCreatorRoutes.Use(middleware.AuthMiddleware(db))
	annotationCreatorRoutes.Use(middleware.ContentCreatorMiddleware())
	{
		annotationCreatorRoutes.POST("/upload", annotationHandler.UploadAndCreateAnnotation)
		annotationCreatorRoutes.GET("/stats", annotationHandler.GetAnnotationStats)
		annotationCreatorRoutes.PATCH("/:id", annotationHandler.UpdateAnnotation)
		annotationCreatorRoutes.DELETE("/:id", annotationHandler.DeleteAnnotation)
		annotationCreatorRoutes.POST("/:id/tts", annotationHandler.GenerateTTSForAnnotation)
	}

	// System routes
	systemRoutes := router.Group("/system")
	{
		systemRoutes.GET("/services/status", annotationHandler.CheckServices)
	}


	// Start server
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	log.Printf("Visit http://localhost:%s to test the connection", port)
	
	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}