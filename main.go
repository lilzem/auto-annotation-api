package main

import (
	"auto-annotation-api/config"
	"auto-annotation-api/database"
	"auto-annotation-api/handlers"
	"auto-annotation-api/middleware"
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

	// Initialize handlers
	authHandler := handlers.NewAuthHandler(db)

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

	// API routes (you can add more here)
	apiRoutes := router.Group("/api")
	apiRoutes.Use(middleware.OptionalAuthMiddleware(db)) // Optional auth for some endpoints
	{
		// Add your other API endpoints here
		// Example: apiRoutes.GET("/annotations", annotationHandler.GetAnnotations)
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