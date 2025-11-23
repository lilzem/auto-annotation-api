package services

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/polly"
	pollyTypes "github.com/aws/aws-sdk-go-v2/service/polly/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// AWSService handles AWS operations (S3 and Polly)
type AWSService struct {
	s3Client     *s3.Client
	pollyClient  *polly.Client
	bucketName   string
	pollyVoiceID string
	pollyEngine  string
}

// NewAWSService creates a new AWS service
func NewAWSService(accessKeyID, secretKey, region, bucketName, voiceID, engine string) (*AWSService, error) {
	// Create AWS config
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			accessKeyID,
			secretKey,
			"",
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	// Set defaults
	if voiceID == "" {
		voiceID = "Joanna"
	}
	if engine == "" {
		engine = "neural"
	}

	return &AWSService{
		s3Client:     s3.NewFromConfig(cfg),
		pollyClient:  polly.NewFromConfig(cfg),
		bucketName:   bucketName,
		pollyVoiceID: voiceID,
		pollyEngine:  engine,
	}, nil
}

// GenerateTTS generates TTS audio using AWS Polly and returns audio data
func (a *AWSService) GenerateTTS(text string) ([]byte, error) {
	// Determine engine type
	var engineType pollyTypes.Engine
	if a.pollyEngine == "neural" {
		engineType = pollyTypes.EngineNeural
	} else {
		engineType = pollyTypes.EngineStandard
	}

	// Create Polly input
	input := &polly.SynthesizeSpeechInput{
		Text:         aws.String(text),
		OutputFormat: pollyTypes.OutputFormatMp3,
		VoiceId:      pollyTypes.VoiceId(a.pollyVoiceID),
		Engine:       engineType,
		TextType:     pollyTypes.TextTypeText,
	}

	// Call Polly API
	result, err := a.pollyClient.SynthesizeSpeech(context.TODO(), input)
	if err != nil {
		return nil, fmt.Errorf("failed to synthesize speech: %w", err)
	}
	defer result.AudioStream.Close()

	// Read audio data
	audioData, err := io.ReadAll(result.AudioStream)
	if err != nil {
		return nil, fmt.Errorf("failed to read audio stream: %w", err)
	}

	return audioData, nil
}

// UploadToS3 uploads data to S3 and returns the public URL
func (a *AWSService) UploadToS3(key string, data []byte, contentType string) (string, error) {
	// Upload to S3 (public access controlled by bucket policy, not ACL)
	_, err := a.s3Client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket:      aws.String(a.bucketName),
		Key:         aws.String(key),
		Body:        bytes.NewReader(data),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload to S3: %w", err)
	}

	// Generate the S3 URL
	url := fmt.Sprintf("https://%s.s3.amazonaws.com/%s", a.bucketName, key)
	return url, nil
}

// GenerateAndUploadTTS generates TTS and uploads to S3, returning the URL
func (a *AWSService) GenerateAndUploadTTS(text, annotationID string) (string, error) {
	// Generate TTS
	audioData, err := a.GenerateTTS(text)
	if err != nil {
		return "", err
	}

	// Create S3 key with timestamp to ensure uniqueness
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("tts/%s_%d.mp3", annotationID, timestamp)

	// Upload to S3
	url, err := a.UploadToS3(key, audioData, "audio/mpeg")
	if err != nil {
		return "", err
	}

	return url, nil
}

// UploadImageToS3 uploads an image to S3 and returns the URL
func (a *AWSService) UploadImageToS3(imageData []byte, annotationID, contentType string) (string, error) {
	// Determine file extension from content type
	ext := ".jpg"
	switch contentType {
	case "image/png":
		ext = ".png"
	case "image/jpeg", "image/jpg":
		ext = ".jpg"
	case "image/gif":
		ext = ".gif"
	case "image/webp":
		ext = ".webp"
	}

	// Create S3 key with timestamp to ensure uniqueness
	timestamp := time.Now().Unix()
	key := fmt.Sprintf("images/%s_%d%s", annotationID, timestamp, ext)

	// Upload to S3
	url, err := a.UploadToS3(key, imageData, contentType)
	if err != nil {
		return "", err
	}

	return url, nil
}

// DeleteFromS3 deletes a file from S3
func (a *AWSService) DeleteFromS3(key string) error {
	_, err := a.s3Client.DeleteObject(context.TODO(), &s3.DeleteObjectInput{
		Bucket: aws.String(a.bucketName),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete from S3: %w", err)
	}
	return nil
}

// TestConnection tests AWS connectivity
func (a *AWSService) TestConnection() error {
	// Test S3 by listing buckets
	_, err := a.s3Client.HeadBucket(context.TODO(), &s3.HeadBucketInput{
		Bucket: aws.String(a.bucketName),
	})
	if err != nil {
		return fmt.Errorf("S3 bucket not accessible: %w", err)
	}

	// Test Polly by describing voices
	_, err = a.pollyClient.DescribeVoices(context.TODO(), &polly.DescribeVoicesInput{})
	if err != nil {
		return fmt.Errorf("polly not accessible: %w", err)
	}

	return nil
}

