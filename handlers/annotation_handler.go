package handlers

import (
	"auto-annotation-api/models"
	"auto-annotation-api/services"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

type AnnotationHandler struct {
	service   *services.AnnotationService
	uploadDir string
}

// NewAnnotationHandler creates a new annotation handler
func NewAnnotationHandler(db *mongo.Database, ollamaBaseURL, ollamaModel, uploadDir string, awsService *services.AWSService) *AnnotationHandler {
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	return &AnnotationHandler{
		service:   services.NewAnnotationService(db, ollamaBaseURL, ollamaModel, uploadDir, awsService),
		uploadDir: uploadDir,
	}
}

// UploadAndCreateAnnotation handles POST /annotations/upload
func (h *AnnotationHandler) UploadAndCreateAnnotation(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
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

	// Get title and image from form
	title := c.PostForm("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Title is required",
		})
		return
	}
	
	image := c.PostForm("image") // Optional image URL

	// Handle file upload
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "File is required",
			"error":   err.Error(),
		})
		return
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if ext != ".pdf" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Only PDF files are supported",
		})
		return
	}

	// Open file for reading (no saving to disk!)
	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to open uploaded file",
			"error":   err.Error(),
		})
		return
	}
	defer file.Close()

	// Create annotation from stream
	fileType := strings.TrimPrefix(ext, ".")
	annotation, err := h.service.CreateAnnotationFromStream(
		c.Request.Context(),
		user.ID,
		title,
		image,
		file,
		fileHeader.Size,
		fileType,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create annotation",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Annotation created successfully",
		"data":    annotation.ToResponse(),
	})
}

// GetAnnotation handles GET /annotations/:id (any authenticated user can view)
func (h *AnnotationHandler) GetAnnotation(c *gin.Context) {
	annotationID := c.Param("id")
	
	annotation, err := h.service.GetAnnotationByID(c.Request.Context(), annotationID)
	if err != nil {
		statusCode := http.StatusNotFound
		if err.Error() != "annotation not found" {
			statusCode = http.StatusInternalServerError
		}
		
		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Failed to get annotation",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Annotation retrieved successfully",
		"data":    annotation.ToResponse(),
	})
}

// GetAllAnnotations handles GET /annotations (all annotations for any authenticated user)
func (h *AnnotationHandler) GetAllAnnotations(c *gin.Context) {
	// Parse query parameters
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.ParseInt(limitStr, 10, 64)
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset, err := strconv.ParseInt(offsetStr, 10, 64)
	if err != nil || offset < 0 {
		offset = 0
	}

	// Get all annotations (no user filter)
	annotations, err := h.service.GetAllAnnotations(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get annotations",
			"error":   err.Error(),
		})
		return
	}

	// Convert to response format
	responses := make([]models.AnnotationResponse, len(annotations))
	for i, annotation := range annotations {
		responses[i] = annotation.ToResponse()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Annotations retrieved successfully",
		"data": gin.H{
			"annotations": responses,
			"pagination": gin.H{
				"limit":  limit,
				"offset": offset,
				"count":  len(responses),
			},
		},
	})
}

// DeleteAnnotation handles DELETE /annotations/:id
func (h *AnnotationHandler) DeleteAnnotation(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
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

	annotationID := c.Param("id")
	
	err := h.service.DeleteAnnotation(c.Request.Context(), annotationID, user.ID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
		}
		
		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Failed to delete annotation",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Annotation deleted successfully",
	})
}

// GetAnnotationStats handles GET /annotations/stats
func (h *AnnotationHandler) GetAnnotationStats(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
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

	stats, err := h.service.GetAnnotationStats(c.Request.Context(), user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get annotation statistics",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Statistics retrieved successfully",
		"data":    stats,
	})
}

// DownloadAudio handles GET /annotations/:id/audio (Deprecated - redirects to S3)
func (h *AnnotationHandler) DownloadAudio(c *gin.Context) {
	annotationID := c.Param("id")
	
	annotation, err := h.service.GetAnnotationByID(c.Request.Context(), annotationID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Annotation not found",
		})
		return
	}

	// TTS files are now stored on S3, redirect to S3 URL
	if annotation.TTSURL == "" {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "TTS audio not available. Use POST /annotations/:id/tts to generate it.",
		})
		return
	}

	// Redirect to S3 URL
	c.Redirect(http.StatusFound, annotation.TTSURL)
}

// CheckServices handles GET /annotations/services/status
func (h *AnnotationHandler) CheckServices(c *gin.Context) {
	status := h.service.CheckServices()
	
	allOK := true
	for _, s := range status {
		if serviceStatus, ok := s.(map[string]interface{}); ok {
			if serviceStatus["status"] != "OK" {
				allOK = false
				break
			}
		}
	}

	statusCode := http.StatusOK
	if !allOK {
		statusCode = http.StatusServiceUnavailable
	}

	c.JSON(statusCode, gin.H{
		"success": allOK,
		"message": "Service status check completed",
		"data":    status,
	})
}

// GenerateTTSForAnnotation handles POST /annotations/:id/tts
func (h *AnnotationHandler) GenerateTTSForAnnotation(c *gin.Context) {
	annotationID := c.Param("id")

	annotation, err := h.service.GenerateTTSForAnnotation(c.Request.Context(), annotationID)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "not configured") {
			statusCode = http.StatusServiceUnavailable
		}

		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Failed to generate TTS",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "TTS generated successfully",
		"data":    annotation.ToResponse(),
	})
}

// UpdateAnnotation handles PATCH /annotations/:id
func (h *AnnotationHandler) UpdateAnnotation(c *gin.Context) {
	// Get user from context
	userInterface, exists := c.Get("user")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "User not authenticated",
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

	annotationID := c.Param("id")

	// Parse request body
	var req models.UpdateAnnotationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
		return
	}

	// Update annotation
	annotation, err := h.service.UpdateAnnotation(c.Request.Context(), annotationID, user.ID, &req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
		} else if strings.Contains(err.Error(), "unauthorized") {
			statusCode = http.StatusForbidden
		}

		c.JSON(statusCode, gin.H{
			"success": false,
			"message": "Failed to update annotation",
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Annotation updated successfully",
		"data":    annotation.ToResponse(),
	})
}
