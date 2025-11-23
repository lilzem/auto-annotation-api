package models

import (
	"time"

	"github.com/google/uuid"
)

// Annotation represents a generated annotation
type Annotation struct {
	ID           string    `json:"id" bson:"_id"`
	UserID       string    `json:"user_id" bson:"user_id"`
	Title        string    `json:"title" bson:"title"`
	Image        string    `json:"image,omitempty" bson:"image,omitempty"` // Image URL/path
	SourceFile   string    `json:"source_file" bson:"source_file"`
	SourceType   string    `json:"source_type" bson:"source_type"` // "pdf" only now
	TextContent  string    `json:"text_content" bson:"text_content"`
	Annotation   string    `json:"annotation" bson:"annotation"`
	Genre        string    `json:"genre" bson:"genre"`
	TTSURL       string    `json:"tts_url,omitempty" bson:"tts_url,omitempty"`
	Status       string    `json:"status" bson:"status"` // "processing", "completed", "failed"
	ErrorMessage string    `json:"error_message,omitempty" bson:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" bson:"updated_at"`
}

// CreateAnnotationRequest represents the request to create an annotation
type CreateAnnotationRequest struct {
	Title string `form:"title" binding:"required"`
	Image string `form:"image"` // Optional image URL
}

// AnnotationResponse represents the annotation response
type AnnotationResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Image       string    `json:"image,omitempty"`
	SourceFile  string    `json:"source_file"`
	SourceType  string    `json:"source_type"`
	Annotation  string    `json:"annotation"`
	Genre       string    `json:"genre"`
	TTSURL      string    `json:"tts_url,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// NewAnnotation creates a new annotation
func NewAnnotation(userID, title, sourceFile, sourceType string) *Annotation {
	now := time.Now()
	return &Annotation{
		ID:         uuid.New().String(),
		UserID:     userID,
		Title:      title,
		SourceFile: sourceFile,
		SourceType: sourceType,
		Status:     "processing",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
}

// ToResponse converts Annotation to AnnotationResponse
func (a *Annotation) ToResponse() AnnotationResponse {
	return AnnotationResponse{
		ID:         a.ID,
		Title:      a.Title,
		Image:      a.Image,
		SourceFile: a.SourceFile,
		SourceType: a.SourceType,
		Annotation: a.Annotation,
		Genre:      a.Genre,
		TTSURL:     a.TTSURL,
		Status:     a.Status,
		CreatedAt:  a.CreatedAt,
		UpdatedAt:  a.UpdatedAt,
	}
}

// UpdateAnnotationRequest represents the request to update an annotation
type UpdateAnnotationRequest struct {
	Title      *string `json:"title,omitempty"`
	Image      *string `json:"image,omitempty"`
	Annotation *string `json:"annotation,omitempty"`
	Genre      *string `json:"genre,omitempty"`
}
