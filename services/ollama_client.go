package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OllamaClient handles communication with local Ollama instance
type OllamaClient struct {
	baseURL string
	model   string
	client  *http.Client
}

// OllamaRequest represents the request to Ollama API
type OllamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

// OllamaResponse represents the response from Ollama API
type OllamaResponse struct {
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// NewOllamaClient creates a new Ollama client
func NewOllamaClient() *OllamaClient {
	baseURL := os.Getenv("OLLAMA_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Default Ollama URL
	}

	model := os.Getenv("OLLAMA_MODEL")
	if model == "" {
		model = "mistral" // Default to mistral model
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 300 * time.Second, // 5 minute timeout for LLM requests
		},
	}
}

// NewOllamaClientWithConfig creates a new Ollama client with provided config
func NewOllamaClientWithConfig(baseURL, model string) *OllamaClient {
	if baseURL == "" {
		baseURL = "http://localhost:11434" // Default Ollama URL
	}

	if model == "" {
		model = "mistral" // Default to mistral model
	}

	return &OllamaClient{
		baseURL: baseURL,
		model:   model,
		client: &http.Client{
			Timeout: 300 * time.Second, // 5 minute timeout for LLM requests
		},
	}
}

// AnnotationWithGenre holds annotation text and detected genre
type AnnotationWithGenre struct {
	Annotation string
	Genre      string
}

// GenerateAnnotation generates an annotation for the given text using Ollama
func (o *OllamaClient) GenerateAnnotation(text, title string) (string, error) {
	result, err := o.GenerateAnnotationWithGenre(text, title)
	if err != nil {
		return "", err
	}
	return result.Annotation, nil
}

// GenerateAnnotationWithGenre generates an annotation and detects genre for the given text
func (o *OllamaClient) GenerateAnnotationWithGenre(text, title string) (*AnnotationWithGenre, error) {
	prompt := o.createAnnotationPrompt(text, title)

	request := OllamaRequest{
		Model:  o.model,
		Prompt: prompt,
		Stream: false,
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Make request to Ollama
	resp, err := o.client.Post(o.baseURL+"/api/generate", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to make request to Ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Ollama API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Read response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var ollamaResp OllamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	responseText := strings.TrimSpace(ollamaResp.Response)
	if responseText == "" {
		return nil, fmt.Errorf("received empty response from Ollama")
	}

	// Parse the response to extract genre and annotation
	result := o.parseAnnotationResponse(responseText)
	
	return result, nil
}

// createAnnotationPrompt creates a comprehensive prompt for annotation generation
func (o *OllamaClient) createAnnotationPrompt(text, title string) string {
	prompt := fmt.Sprintf(`You are creating educational study notes. Write directly about the concepts and ideas, not about the document itself.

Title: %s

Source Material:
%s

INSTRUCTIONS:
1. Start with: GENRE: [pick one: Fiction, Non-Fiction, Academic, Educational, or Other]

2. Then write your educational notes/annotation.

CRITICAL RULES - YOU MUST FOLLOW THESE:
- NEVER start sentences with: "This paper", "This document", "This case study", "This content", "The author", "The research"
- NEVER use phrases like: "discusses", "presents", "explores", "examines" when referring to the document
- Write DIRECTLY about the subject matter itself
- Act as if YOU are teaching the topic, not describing someone else's work

WRONG (DO NOT DO THIS):
"This case study presents the Software as a Service lifecycle..."
"The paper discusses cloud computing concepts..."
"This research examines the relationship between..."

CORRECT (DO THIS):
"The Software as a Service (SaaS) lifecycle encompasses multiple phases..."
"Cloud computing relies on distributed infrastructure..."
"Modern software sourcing involves strategic vendor selection..."

Start your response with "GENRE:" followed by your direct educational content. Begin now:`, title, text)

	return prompt
}

// parseAnnotationResponse parses the Ollama response to extract genre and annotation
func (o *OllamaClient) parseAnnotationResponse(response string) *AnnotationWithGenre {
	result := &AnnotationWithGenre{
		Genre:      "Other", // Default genre
		Annotation: response,
	}

	// Look for "GENRE: " at the start
	lines := strings.Split(response, "\n")
	if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "GENRE:") {
		// Extract genre
		genreLine := strings.TrimSpace(lines[0])
		genre := strings.TrimSpace(strings.TrimPrefix(genreLine, "GENRE:"))
		result.Genre = genre

		// Remove the genre line from annotation
		if len(lines) > 1 {
			result.Annotation = strings.TrimSpace(strings.Join(lines[1:], "\n"))
		} else {
			result.Annotation = ""
		}
	}

	// If annotation is empty, use the full response
	if result.Annotation == "" {
		result.Annotation = response
	}

	return result
}

// TestConnection tests if Ollama is accessible
func (o *OllamaClient) TestConnection() error {
	resp, err := o.client.Get(o.baseURL + "/api/tags")
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", o.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama not responding correctly (status %d)", resp.StatusCode)
	}

	return nil
}

// GetAvailableModels returns list of available models in Ollama
func (o *OllamaClient) GetAvailableModels() ([]string, error) {
	resp, err := o.client.Get(o.baseURL + "/api/tags")
	if err != nil {
		return nil, fmt.Errorf("failed to get models: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result struct {
		Models []struct {
			Name string `json:"name"`
		} `json:"models"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var models []string
	for _, model := range result.Models {
		models = append(models, model.Name)
	}

	return models, nil
}
