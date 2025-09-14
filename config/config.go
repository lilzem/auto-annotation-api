package config

import "os"

// Config holds all configuration for the application
type Config struct {
	MongoURI     string
	DatabaseName string
	Port         string
	GinMode      string
	Environment  string
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		MongoURI:     getEnv("MONGODB_URI", "mongodb://localhost:27017"),
		DatabaseName: getEnv("MONGODB_DATABASE", "auto_annotation_db"),
		Port:         getEnv("PORT", "8080"),
		GinMode:      getEnv("GIN_MODE", "debug"),
		Environment:  getEnv("ENVIRONMENT", "development"),
	}
}

// getEnv gets an environment variable with a fallback default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
