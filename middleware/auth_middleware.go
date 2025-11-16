package middleware

import (
	"auto-annotation-api/models"
	"auto-annotation-api/services"
	"auto-annotation-api/utils"
	"net/http"
	"slices"
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

// ContentCreatorMiddleware ensures only users with "content" role can access
func ContentCreatorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (should be set by AuthMiddleware)
		userInterface, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "User not authenticated",
			})
			c.Abort()
			return
		}

		user, ok := userInterface.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid user data",
			})
			c.Abort()
			return
		}

		// Check if user has content creator role
		if !user.IsContentCreator() {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Access denied. Content creator role required.",
				"user_role": user.Role,
			})
			c.Abort()
			return
		}

		// User has content creator role, continue
		c.Next()
	}
}

// RoleMiddleware creates a middleware that checks for specific roles
func RoleMiddleware(allowedRoles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user from context (should be set by AuthMiddleware)
		userInterface, exists := c.Get("user")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"message": "User not authenticated",
			})
			c.Abort()
			return
		}

		user, ok := userInterface.(*models.User)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Invalid user data",
			})
			c.Abort()
			return
		}

		// Check if user has any of the allowed roles
		hasRole := slices.ContainsFunc(allowedRoles, user.HasRole)

		if !hasRole {
			c.JSON(http.StatusForbidden, gin.H{
				"success": false,
				"message": "Access denied. Required role not found.",
				"user_role": user.Role,
				"allowed_roles": allowedRoles,
			})
			c.Abort()
			return
		}

		// User has required role, continue
		c.Next()
	}
}
