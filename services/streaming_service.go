package services

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// StreamingService handles Server-Sent Events for real-time updates
type StreamingService struct {
	clients map[string]map[chan StreamEvent]bool // annotationID -> clients
	mutex   sync.RWMutex
}

// StreamEvent represents an event sent to clients
type StreamEvent struct {
	Type      string      `json:"type"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// ProgressUpdate represents progress information
type ProgressUpdate struct {
	AnnotationID string `json:"annotation_id"`
	Status       string `json:"status"`
	Step         string `json:"step"`
	Progress     int    `json:"progress"` // 0-100
	Message      string `json:"message"`
	Error        string `json:"error,omitempty"`
}

// NewStreamingService creates a new streaming service
func NewStreamingService() *StreamingService {
	return &StreamingService{
		clients: make(map[string]map[chan StreamEvent]bool),
	}
}

// AddClient adds a client for streaming updates for a specific annotation
func (s *StreamingService) AddClient(annotationID string, clientChan chan StreamEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.clients[annotationID] == nil {
		s.clients[annotationID] = make(map[chan StreamEvent]bool)
	}
	s.clients[annotationID][clientChan] = true
}

// RemoveClient removes a client from streaming
func (s *StreamingService) RemoveClient(annotationID string, clientChan chan StreamEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if clients, exists := s.clients[annotationID]; exists {
		delete(clients, clientChan)
		if len(clients) == 0 {
			delete(s.clients, annotationID)
		}
	}
	close(clientChan)
}

// SendProgress sends a progress update to all clients listening to an annotation
func (s *StreamingService) SendProgress(annotationID, status, step, message string, progress int) {
	update := ProgressUpdate{
		AnnotationID: annotationID,
		Status:       status,
		Step:         step,
		Progress:     progress,
		Message:      message,
	}

	event := StreamEvent{
		Type:      "progress",
		Data:      update,
		Timestamp: time.Now(),
	}

	s.sendEvent(annotationID, event)
}

// SendError sends an error update to all clients
func (s *StreamingService) SendError(annotationID, step, errorMsg string) {
	update := ProgressUpdate{
		AnnotationID: annotationID,
		Status:       "failed",
		Step:         step,
		Progress:     0,
		Message:      "Processing failed",
		Error:        errorMsg,
	}

	event := StreamEvent{
		Type:      "error",
		Data:      update,
		Timestamp: time.Now(),
	}

	s.sendEvent(annotationID, event)
}

// SendComplete sends completion notification to all clients
func (s *StreamingService) SendComplete(annotationID string, annotation interface{}) {
	event := StreamEvent{
		Type:      "complete",
		Data:      annotation,
		Timestamp: time.Now(),
	}

	s.sendEvent(annotationID, event)
}

// sendEvent sends an event to all clients listening to a specific annotation
func (s *StreamingService) sendEvent(annotationID string, event StreamEvent) {
	s.mutex.RLock()
	clients, exists := s.clients[annotationID]
	s.mutex.RUnlock()

	if !exists {
		return
	}

	// Send to all clients (non-blocking)
	for clientChan := range clients {
		select {
		case clientChan <- event:
		default:
			// Client channel is full or closed, remove it
			go s.RemoveClient(annotationID, clientChan)
		}
	}
}

// StreamHandler handles SSE connections for annotation updates
func (s *StreamingService) StreamHandler(c *gin.Context) {
	annotationID := c.Param("id")
	if annotationID == "" {
		c.JSON(400, gin.H{"error": "annotation ID required"})
		return
	}

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("Access-Control-Allow-Headers", "Cache-Control")

	// Create client channel
	clientChan := make(chan StreamEvent, 10)
	s.AddClient(annotationID, clientChan)

	// Send initial connection event
	initialEvent := StreamEvent{
		Type: "connected",
		Data: map[string]string{
			"annotation_id": annotationID,
			"message":       "Connected to annotation stream",
		},
		Timestamp: time.Now(),
	}

	// Send initial event
	s.writeSSEEvent(c, initialEvent)
	c.Writer.Flush()

	// Listen for events and client disconnect
	clientGone := c.Request.Context().Done()
	for {
		select {
		case event := <-clientChan:
			s.writeSSEEvent(c, event)
			c.Writer.Flush()
		case <-clientGone:
			s.RemoveClient(annotationID, clientChan)
			return
		}
	}
}

// writeSSEEvent writes an SSE event to the response
func (s *StreamingService) writeSSEEvent(c *gin.Context, event StreamEvent) {
	data, _ := json.Marshal(event)
	fmt.Fprintf(c.Writer, "data: %s\n\n", data)
}

// Global streaming service instance
var GlobalStreamingService = NewStreamingService()
