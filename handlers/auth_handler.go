package handlers

import (
	"auto-annotation-api/models"
	"auto-annotation-api/services"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type AuthHandler struct {
	authService *services.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(db *mongo.Database) *AuthHandler {
	return &AuthHandler{
		authService: services.NewAuthService(db),
	}
}

// Register handles POST /auth/register
func (h *AuthHandler) Register(c *gin.Context) {
	var req models.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
		return
	}

	authResponse, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "user with this email already exists" {
			statusCode = http.StatusConflict
		}
		
		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Registration failed",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "User registered successfully",
		"data":    authResponse,
	})
}

// Login handles POST /auth/login
func (h *AuthHandler) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request payload",
			"error":   err.Error(),
		})
		return
	}

	authResponse, err := h.authService.Login(c.Request.Context(), req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err.Error() == "invalid email or password" {
			statusCode = http.StatusUnauthorized
		}

		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Login failed",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Login successful",
		"data":    authResponse,
	})
}

// GetProfile handles GET /auth/profile (protected route)
func (h *AuthHandler) GetProfile(c *gin.Context) {
	// Get user from context (set by JWT middleware)
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not found in context",
		})
		return
	}

	user, ok := userInterface.(*models.User)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Invalid user data",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Profile retrieved successfully",
		"data":    user.ToUserResponse(),
	})
}
