package middleware

import (
	"auto-annotation-api/services"
	"auto-annotation-api/utils"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// AuthMiddleware validates JWT tokens and adds user to context
func AuthMiddleware(db *mongo.Database) gin.HandlerFunc {
	authService := services.NewAuthService(db)

	return func(c *gin.Context) {
		// Get token from Authorization header
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Authorization header required",
			})
			c.Abort()
			return
		}

		// Check if header starts with "Bearer "
		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid authorization header format. Use: Bearer <token>",
			})
			c.Abort()
			return
		}

		tokenString := tokenParts[1]

		// Validate token
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "Invalid or expired token",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// Get user from database
		user, err := authService.GetUserByID(c.Request.Context(), claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "User not found",
				"error":   err.Error(),
			})
			c.Abort()
			return
		}

		// Add user to context
		c.Set("user", user)
		c.Set("userID", user.ID)

		// Continue to next handler
		c.Next()
	}
}

// OptionalAuthMiddleware is like AuthMiddleware but doesn't abort if no token
func OptionalAuthMiddleware(db *mongo.Database) gin.HandlerFunc {
	authService := services.NewAuthService(db)

	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			// No token provided, continue without setting user
			c.Next()
			return
		}

		tokenParts := strings.Split(authHeader, " ")
		if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
			// Invalid format, continue without setting user
			c.Next()
			return
		}

		tokenString := tokenParts[1]
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			// Invalid token, continue without setting user
			c.Next()
			return
		}

		user, err := authService.GetUserByID(c.Request.Context(), claims.UserID)
		if err != nil {
			// User not found, continue without setting user
			c.Next()
			return
		}

		// Add user to context
		c.Set("user", user)
		c.Set("userID", user.ID)
		c.Next()
	}
}
