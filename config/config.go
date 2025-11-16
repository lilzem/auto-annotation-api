package config

import "os"

// Config holds all configuration for the application
type Config struct {
	MongoURI          string
	DatabaseName      string
	Port              string
	GinMode           string
	Environment       string
	OllamaBaseURL     string
	OllamaModel       string
	UploadDir         string
	TTSOutputDir      string
	JWTSecret         string
	AWSAccessKeyID    string
	AWSSecretKey      string
	AWSRegion         string
	AWSS3BucketName   string
	AWSPollyVoiceID   string
	AWSPollyEngine    string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		MongoURI:          getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		DatabaseName:      getEnv("MONGODB_DATABASE", "auto_annotation_db"),
		Port:              getEnv("PORT", "8080"),
		GinMode:           getEnv("GIN_MODE", "debug"),
		Environment:       getEnv("ENVIRONMENT", "development"),
		OllamaBaseURL:     getEnv("OLLAMA_BASE_URL", "http://localhost:11434"),
		OllamaModel:       getEnv("OLLAMA_MODEL", "mistral"),
		UploadDir:         getEnv("UPLOAD_DIR", "uploads"),
		TTSOutputDir:      getEnv("TTS_OUTPUT_DIR", "uploads/audio"),
		JWTSecret:         getEnv("JWT_SECRET", "your-super-secret-jwt-key-change-this-in-production"),
		AWSAccessKeyID:    getEnv("AWS_ACCESS_KEY_ID", ""),
		AWSSecretKey:      getEnv("AWS_SECRET_ACCESS_KEY", ""),
		AWSRegion:         getEnv("AWS_REGION", "us-east-1"),
		AWSS3BucketName:   getEnv("AWS_S3_BUCKET_NAME", ""),
		AWSPollyVoiceID:   getEnv("AWS_POLLY_VOICE_ID", "Joanna"),
		AWSPollyEngine:    getEnv("AWS_POLLY_ENGINE", "neural"),
	}
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
