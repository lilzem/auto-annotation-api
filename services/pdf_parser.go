package services

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFParser handles PDF text extraction
type PDFParser struct{}

// NewPDFParser creates a new PDF parser
func NewPDFParser() *PDFParser {
	return &PDFParser{}
}

// ExtractTextFromReader extracts text content from a PDF reader
func (p *PDFParser) ExtractTextFromReader(reader io.Reader, size int64) (string, error) {
	// Read all data into memory
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("failed to read PDF data: %w", err)
	}

	// Create reader from bytes
	r, err := pdf.NewReader(bytes.NewReader(data), size)
	if err != nil {
		return "", fmt.Errorf("failed to parse PDF: %w", err)
	}

	var textBuilder strings.Builder
	totalPages := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		page := r.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}

		textContent, err := page.GetPlainText(nil)
		if err != nil {
			// Skip this page if we can't extract text
			continue
		}

		// Add page separator for better structure
		if pageIndex > 1 {
			textBuilder.WriteString("\n\n--- Page " + fmt.Sprintf("%d", pageIndex) + " ---\n\n")
		}
		
		textBuilder.WriteString(textContent)
	}

	extractedText := textBuilder.String()
	
	// Clean up the text
	extractedText = cleanExtractedText(extractedText)
	
	if extractedText == "" {
		return "", fmt.Errorf("no text content found in PDF")
	}

	return extractedText, nil
}

// ExtractText extracts text content from a PDF file (kept for backward compatibility if needed)
func (p *PDFParser) ExtractText(filePath string) (string, error) {
	f, r, err := pdf.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	var textBuilder strings.Builder
	totalPages := r.NumPage()

	for pageIndex := 1; pageIndex <= totalPages; pageIndex++ {
		page := r.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}

		textContent, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}

		if pageIndex > 1 {
			textBuilder.WriteString("\n\n--- Page " + fmt.Sprintf("%d", pageIndex) + " ---\n\n")
		}
		
		textBuilder.WriteString(textContent)
	}

	extractedText := cleanExtractedText(textBuilder.String())
	if extractedText == "" {
		return "", fmt.Errorf("no text content found in PDF")
	}

	return extractedText, nil
}

// cleanExtractedText cleans and normalizes extracted text
func cleanExtractedText(text string) string {
	// Replace multiple whitespaces with single space
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	
	// Remove excessive line breaks
	lines := strings.Split(text, "\n")
	var cleanLines []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanLines = append(cleanLines, line)
		}
	}
	
	// Join with proper spacing
	result := strings.Join(cleanLines, "\n")
	
	// Remove excessive spaces
	for strings.Contains(result, "  ") {
		result = strings.ReplaceAll(result, "  ", " ")
	}
	
	return strings.TrimSpace(result)
}

// FileParser interface for unified file parsing
type FileParser interface {
	ExtractText(filePath string) (string, error)
	ExtractTextFromReader(reader io.Reader, size int64) (string, error)
}

// GetParser returns appropriate parser based on file type
func GetParser(fileType string) FileParser {
	switch strings.ToLower(fileType) {
	case "pdf", ".pdf":
		return NewPDFParser()
	default:
		return nil
	}
}
