package services

import (
	"auto-annotation-api/models"
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// AnnotationService orchestrates the annotation creation process
type AnnotationService struct {
	collection    *mongo.Collection
	ollamaClient  *OllamaClient
	awsService    *AWSService
	uploadDir     string
}

// NewAnnotationService creates a new annotation service
func NewAnnotationService(db *mongo.Database, ollamaBaseURL, ollamaModel, uploadDir string, awsService *AWSService) *AnnotationService {
	if uploadDir == "" {
		uploadDir = "uploads"
	}

	// Create upload directory if it doesn't exist
	os.MkdirAll(uploadDir, 0755)
	os.MkdirAll(uploadDir+"/files", 0755)

	return &AnnotationService{
		collection:   db.Collection("annotations"),
		ollamaClient: NewOllamaClientWithConfig(ollamaBaseURL, ollamaModel),
		awsService:   awsService,
		uploadDir:    uploadDir,
	}
}

// CreateAnnotation creates a new annotation from uploaded file (synchronous)
func (s *AnnotationService) CreateAnnotation(ctx context.Context, userID, title, filePath, fileType string) (*models.Annotation, error) {
	// Create annotation record
	annotation := models.NewAnnotation(userID, title, filePath, fileType)

	// Step 1: Extract text from file
	log.Printf("Extracting text from %s file: %s", fileType, filePath)
	text, err := s.extractTextFromFile(filePath, fileType)
	if err != nil {
		return nil, fmt.Errorf("failed to extract text: %w", err)
	}
	annotation.TextContent = text
	log.Printf("Extracted %d characters of text from file", len(text))

	// Step 2: Generate annotation and genre using Ollama
	log.Printf("Generating annotation and genre using Ollama for: %s", title)
	result, err := s.ollamaClient.GenerateAnnotationWithGenre(text, title)
	if err != nil {
		annotation.Status = "failed"
		annotation.ErrorMessage = fmt.Sprintf("Annotation generation failed: %v", err)
		s.collection.InsertOne(ctx, annotation)
		return nil, fmt.Errorf("failed to generate annotation: %w", err)
	}
	annotation.Annotation = result.Annotation
	annotation.Genre = result.Genre
	log.Printf("Generated annotation of %d characters, genre: %s", len(result.Annotation), result.Genre)

	// Mark as completed (no TTS yet)
	annotation.Status = "completed"
	annotation.UpdatedAt = time.Now()

	// Insert into database
	_, err = s.collection.InsertOne(ctx, annotation)
	if err != nil {
		return nil, fmt.Errorf("failed to create annotation record: %w", err)
	}

	return annotation, nil
}

// GenerateTTSForAnnotation generates TTS for an existing annotation and uploads to S3
func (s *AnnotationService) GenerateTTSForAnnotation(ctx context.Context, annotationID string) (*models.Annotation, error) {
	// Get annotation
	annotation, err := s.GetAnnotationByID(ctx, annotationID)
	if err != nil {
		return nil, err
	}

	// Check if annotation text exists
	if annotation.Annotation == "" {
		return nil, fmt.Errorf("annotation text is empty")
	}

	// Check if AWS service is available
	if s.awsService == nil {
		return nil, fmt.Errorf("AWS service not configured")
	}

	log.Printf("Generating TTS for annotation ID: %s", annotationID)

	// Generate TTS and upload to S3
	ttsURL, err := s.awsService.GenerateAndUploadTTS(annotation.Annotation, annotationID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate TTS: %w", err)
	}

	log.Printf("TTS generated and uploaded to S3: %s", ttsURL)

	// Update annotation with TTS URL
	update := bson.M{
		"$set": bson.M{
			"tts_url":    ttsURL,
			"updated_at": time.Now(),
		},
	}

	_, err = s.collection.UpdateOne(
		ctx,
		bson.M{"_id": annotationID},
		update,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update annotation: %w", err)
	}

	// Return updated annotation
	return s.GetAnnotationByID(ctx, annotationID)
}

// UpdateAnnotation updates an annotation's fields
func (s *AnnotationService) UpdateAnnotation(ctx context.Context, annotationID, userID string, req *models.UpdateAnnotationRequest) (*models.Annotation, error) {
	// Get annotation first to check ownership
	annotation, err := s.GetAnnotationByID(ctx, annotationID)
	if err != nil {
		return nil, err
	}

	if annotation.UserID != userID {
		return nil, fmt.Errorf("unauthorized: annotation belongs to different user")
	}

	// Build update query
	updateFields := bson.M{
		"updated_at": time.Now(),
	}

	if req.Title != nil {
		updateFields["title"] = *req.Title
	}
	if req.Annotation != nil {
		updateFields["annotation"] = *req.Annotation
	}
	if req.Genre != nil {
		updateFields["genre"] = *req.Genre
	}

	update := bson.M{"$set": updateFields}

	// Update annotation
	_, err = s.collection.UpdateOne(
		ctx,
		bson.M{"_id": annotationID},
		update,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update annotation: %w", err)
	}

	// Return updated annotation
	return s.GetAnnotationByID(ctx, annotationID)
}

// extractTextFromFile extracts text content from uploaded file
func (s *AnnotationService) extractTextFromFile(filePath, fileType string) (string, error) {
	parser := GetParser(filePath)
	if parser == nil {
		return "", fmt.Errorf("unsupported file type: %s", fileType)
	}

	return parser.ExtractText(filePath)
}


// GetAnnotationByID retrieves an annotation by ID
func (s *AnnotationService) GetAnnotationByID(ctx context.Context, annotationID string) (*models.Annotation, error) {
	var annotation models.Annotation
	err := s.collection.FindOne(ctx, bson.M{"_id": annotationID}).Decode(&annotation)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("annotation not found")
		}
		return nil, err
	}
	return &annotation, nil
}

// GetUserAnnotations retrieves all annotations for a user
func (s *AnnotationService) GetUserAnnotations(ctx context.Context, userID string, limit, offset int64) ([]*models.Annotation, error) {
	opts := options.Find()
	if limit > 0 {
		opts.SetLimit(limit)
	}
	if offset > 0 {
		opts.SetSkip(offset)
	}
	opts.SetSort(bson.D{{Key: "created_at", Value: -1}})

	cursor, err := s.collection.Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var annotations []*models.Annotation
	if err = cursor.All(ctx, &annotations); err != nil {
		return nil, err
	}

	return annotations, nil
}

// DeleteAnnotation deletes an annotation and its associated files
func (s *AnnotationService) DeleteAnnotation(ctx context.Context, annotationID, userID string) error {
	// Get annotation first to check ownership and get file paths
	annotation, err := s.GetAnnotationByID(ctx, annotationID)
	if err != nil {
		return err
	}

	if annotation.UserID != userID {
		return fmt.Errorf("unauthorized: annotation belongs to different user")
	}

	// Delete from database
	result, err := s.collection.DeleteOne(ctx, bson.M{"_id": annotationID})
	if err != nil {
		return err
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("annotation not found")
	}

	// Clean up files asynchronously
	go s.cleanupFiles(annotation.SourceFile, annotation.TTSURL)

	return nil
}

// cleanupFiles removes associated files
func (s *AnnotationService) cleanupFiles(sourceFile, ttsURL string) {
	// Remove source file
	if sourceFile != "" {
		if err := os.Remove(sourceFile); err != nil {
			log.Printf("Failed to remove source file %s: %v", sourceFile, err)
		}
	}

	// Note: TTS files are in S3, we could delete them but keeping them for now
	// If you want to delete from S3, extract the key from URL and call s.awsService.DeleteFromS3(key)
}

// GetAnnotationStats returns statistics about annotations
func (s *AnnotationService) GetAnnotationStats(ctx context.Context, userID string) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"user_id": userID}},
		{"$group": bson.M{
			"_id": "$status",
			"count": bson.M{"$sum": 1},
		}},
	}

	cursor, err := s.collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stats := map[string]interface{}{
		"total":      0,
		"processing": 0,
		"completed":  0,
		"failed":     0,
	}

	for cursor.Next(ctx) {
		var result struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cursor.Decode(&result); err != nil {
			continue
		}

		stats[result.ID] = result.Count
		stats["total"] = stats["total"].(int) + result.Count
	}

	return stats, nil
}

// CheckServices verifies that required services are available
func (s *AnnotationService) CheckServices() map[string]interface{} {
	status := make(map[string]interface{})

	// Check Ollama
	if err := s.ollamaClient.TestConnection(); err != nil {
		status["ollama"] = map[string]interface{}{
			"status": "Error",
			"error":  err.Error(),
		}
	} else {
		// Get available models
		models, err := s.ollamaClient.GetAvailableModels()
		if err != nil {
			status["ollama"] = map[string]interface{}{
				"status": "Connected",
				"models": "Error getting models: " + err.Error(),
			}
		} else {
			status["ollama"] = map[string]interface{}{
				"status": "OK",
				"models": models,
			}
		}
	}

	// Check AWS (S3 and Polly)
	if s.awsService != nil {
		if err := s.awsService.TestConnection(); err != nil {
			status["aws"] = map[string]interface{}{
				"status": "Error",
				"error":  err.Error(),
			}
		} else {
			status["aws"] = map[string]interface{}{
				"status": "OK",
				"services": "S3 and Polly",
			}
		}
	} else {
		status["aws"] = map[string]interface{}{
			"status": "Not Configured",
		}
	}

	return status
}
