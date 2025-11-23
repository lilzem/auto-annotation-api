package handlers

import (
	"auto-annotation-api/models"
	"auto-annotation-api/services"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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

	// Get title from form
	title := c.PostForm("title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Title is required",
		})
		return
	}
	
	// Handle optional image - can be URL or file upload
	var imageURL string
	
	// Check if image file was uploaded
	imageFile, err := c.FormFile("image")
	if err == nil {
		// Image file provided - validate and upload to S3
		ext := strings.ToLower(filepath.Ext(imageFile.Filename))
		validExts := map[string]bool{
			".jpg":  true,
			".jpeg": true,
			".png":  true,
			".gif":  true,
			".webp": true,
		}

		if !validExts[ext] {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Only image files are supported (jpg, png, gif, webp)",
			})
			return
		}

		// Open and read image file
		imgFile, err := imageFile.Open()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to open uploaded image",
				"error":   err.Error(),
			})
			return
		}
		defer imgFile.Close()

		imageData := make([]byte, imageFile.Size)
		_, err = imgFile.Read(imageData)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to read uploaded image",
				"error":   err.Error(),
			})
			return
		}

		// Determine content type
		contentType := "image/jpeg"
		switch ext {
		case ".png":
			contentType = "image/png"
		case ".gif":
			contentType = "image/gif"
		case ".webp":
			contentType = "image/webp"
		}

		// We'll pass the image data to the service to upload after annotation is created
		// For now, generate a temporary ID to use for the S3 key
		tempID := fmt.Sprintf("temp_%d", time.Now().UnixNano())
		uploadedURL, err := h.service.UploadImageForAnnotationUpdate(c.Request.Context(), tempID, imageData, contentType)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to upload image",
				"error":   err.Error(),
			})
			return
		}
		imageURL = uploadedURL
	} else {
		// No image file - check if image URL was provided as text
		imageURL = c.PostForm("image_url")
	}

	// Handle PDF file upload
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
		imageURL,
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

// UpdateAnnotation handles PATCH /annotations/:id (accepts FormData)
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

	req := &models.UpdateAnnotationRequest{}
	
	// Check Content-Type to determine how to parse the request
	contentType := c.GetHeader("Content-Type")
	isMultipart := strings.HasPrefix(contentType, "multipart/form-data")
	
	if isMultipart {
		// Parse as FormData
		title := c.PostForm("title")
		annotation := c.PostForm("annotation")
		genre := c.PostForm("genre")
		
		log.Printf("PATCH /annotations/%s - FormData: title='%s', annotation_len=%d, genre='%s'", 
			annotationID, title, len(annotation), genre)
		
		if title != "" {
			req.Title = &title
		}
		if annotation != "" {
			req.Annotation = &annotation
		}
		if genre != "" {
			req.Genre = &genre
		}
		
		// Handle optional image upload
		imageFile, err := c.FormFile("image")
		if err == nil {
			// Image file provided - validate and upload to S3
			ext := strings.ToLower(filepath.Ext(imageFile.Filename))
			validExts := map[string]bool{
				".jpg":  true,
				".jpeg": true,
				".png":  true,
				".gif":  true,
				".webp": true,
			}

			if !validExts[ext] {
				c.JSON(http.StatusBadRequest, gin.H{
					"success": false,
					"message": "Only image files are supported (jpg, png, gif, webp)",
				})
				return
			}

			// Open and read image file
			file, err := imageFile.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to open uploaded image",
					"error":   err.Error(),
				})
				return
			}
			defer file.Close()

			imageData := make([]byte, imageFile.Size)
			_, err = file.Read(imageData)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to read uploaded image",
					"error":   err.Error(),
				})
				return
			}

			// Determine content type
			imageContentType := "image/jpeg"
			switch ext {
			case ".png":
				imageContentType = "image/png"
			case ".gif":
				imageContentType = "image/gif"
			case ".webp":
				imageContentType = "image/webp"
			}

			// Upload to S3 and get URL
			imageURL, err := h.service.UploadImageForAnnotationUpdate(c.Request.Context(), annotationID, imageData, imageContentType)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to upload image",
					"error":   err.Error(),
				})
				return
			}

			req.Image = &imageURL
		}
	} else {
		// Parse as JSON
		var jsonReq models.UpdateAnnotationRequest
		if err := c.ShouldBindJSON(&jsonReq); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid request body",
				"error":   err.Error(),
			})
			return
		}
		
		log.Printf("PATCH /annotations/%s - JSON: title=%v, annotation=%v, genre=%v, image=%v", 
			annotationID, 
			jsonReq.Title != nil,
			jsonReq.Annotation != nil,
			jsonReq.Genre != nil,
			jsonReq.Image != nil)
		
		req = &jsonReq
	}

	// Update annotation
	var updatedAnnotation *models.Annotation
	var err error
	updatedAnnotation, err = h.service.UpdateAnnotation(c.Request.Context(), annotationID, user.ID, req)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if strings.Contains(err.Error(), "not found") {
			statusCode = http.StatusNotFound
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
		"data":    updatedAnnotation.ToResponse(),
	})
}
