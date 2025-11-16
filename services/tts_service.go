package services

import (
	"crypto/md5"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// TTSService handles text-to-speech conversion
type TTSService struct {
	outputDir string
}

// NewTTSService creates a new TTS service
func NewTTSService() *TTSService {
	outputDir := os.Getenv("TTS_OUTPUT_DIR")
	if outputDir == "" {
		outputDir = "uploads/audio" // Default audio output directory
	}

	// Create output directory if it doesn't exist
	os.MkdirAll(outputDir, 0755)

	return &TTSService{
		outputDir: outputDir,
	}
}

// ConvertToSpeech converts text to speech and returns the audio file path
func (t *TTSService) ConvertToSpeech(text, annotationID string) (string, error) {
	// Generate unique filename
	hash := fmt.Sprintf("%x", md5.Sum([]byte(annotationID+text)))
	fileName := fmt.Sprintf("annotation_%s_%s.wav", annotationID, hash[:8])
	filePath := filepath.Join(t.outputDir, fileName)

	// Check if file already exists
	if _, err := os.Stat(filePath); err == nil {
		return fileName, nil // Return existing file
	}

	// Choose TTS method based on OS
	var err error
	switch runtime.GOOS {
	case "darwin": // macOS
		err = t.convertWithSay(text, filePath)
	case "linux":
		err = t.convertWithEspeak(text, filePath)
	case "windows":
		err = t.convertWithPowerShell(text, filePath)
	default:
		return "", fmt.Errorf("TTS not supported on %s", runtime.GOOS)
	}

	if err != nil {
		return "", fmt.Errorf("TTS conversion failed: %w", err)
	}

	return fileName, nil
}

// convertWithSay uses macOS 'say' command for TTS
func (t *TTSService) convertWithSay(text, outputPath string) error {
	// Clean text for better speech synthesis
	cleanText := t.cleanTextForTTS(text)
	
	cmd := exec.Command("say", "-o", outputPath, cleanText)
	return cmd.Run()
}

// convertWithEspeak uses Linux 'espeak' for TTS
func (t *TTSService) convertWithEspeak(text, outputPath string) error {
	cleanText := t.cleanTextForTTS(text)
	
	cmd := exec.Command("espeak", "-w", outputPath, cleanText)
	return cmd.Run()
}

// convertWithPowerShell uses Windows PowerShell for TTS
func (t *TTSService) convertWithPowerShell(text, outputPath string) error {
	cleanText := t.cleanTextForTTS(text)
	
	// PowerShell script for TTS
	script := fmt.Sprintf(`
		Add-Type -AssemblyName System.Speech
		$synth = New-Object System.Speech.Synthesis.SpeechSynthesizer
		$synth.SetOutputToWaveFile('%s')
		$synth.Speak('%s')
		$synth.Dispose()
	`, outputPath, strings.ReplaceAll(cleanText, "'", "''"))

	cmd := exec.Command("powershell", "-Command", script)
	return cmd.Run()
}

// cleanTextForTTS cleans text to improve TTS output
func (t *TTSService) cleanTextForTTS(text string) string {
	// Remove excessive whitespace
	text = strings.TrimSpace(text)
	
	// Replace multiple spaces with single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	
	// Replace multiple line breaks with period and space for better speech flow
	text = strings.ReplaceAll(text, "\n\n", ". ")
	text = strings.ReplaceAll(text, "\n", " ")
	
	// Ensure sentences end with proper punctuation for natural pauses
	sentences := strings.Split(text, ".")
	var cleanSentences []string
	
	for _, sentence := range sentences {
		sentence = strings.TrimSpace(sentence)
		if sentence != "" {
			if !strings.HasSuffix(sentence, ".") && !strings.HasSuffix(sentence, "!") && !strings.HasSuffix(sentence, "?") {
				sentence += "."
			}
			cleanSentences = append(cleanSentences, sentence)
		}
	}
	
	return strings.Join(cleanSentences, " ")
}

// GetAudioFilePath returns the full path to an audio file
func (t *TTSService) GetAudioFilePath(fileName string) string {
	return filepath.Join(t.outputDir, fileName)
}

// DeleteAudioFile removes an audio file
func (t *TTSService) DeleteAudioFile(fileName string) error {
	filePath := filepath.Join(t.outputDir, fileName)
	return os.Remove(filePath)
}

// CheckTTSAvailability checks if TTS is available on the system
func (t *TTSService) CheckTTSAvailability() error {
	switch runtime.GOOS {
	case "darwin":
		// Check if 'say' command is available
		_, err := exec.LookPath("say")
		if err != nil {
			return fmt.Errorf("'say' command not found on macOS")
		}
	case "linux":
		// Check if 'espeak' is available
		_, err := exec.LookPath("espeak")
		if err != nil {
			return fmt.Errorf("'espeak' not found. Install with: sudo apt-get install espeak")
		}
	case "windows":
		// Check if PowerShell is available (should be on most Windows systems)
		_, err := exec.LookPath("powershell")
		if err != nil {
			return fmt.Errorf("PowerShell not found on Windows")
		}
	default:
		return fmt.Errorf("TTS not supported on %s", runtime.GOOS)
	}
	
	return nil
}
