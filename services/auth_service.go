package services

import (
	"auto-annotation-api/models"
	"auto-annotation-api/utils"
	"context"
	"errors"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	collection *mongo.Collection
}

// NewAuthService creates a new auth service
func NewAuthService(db *mongo.Database) *AuthService {
	return &AuthService{
		collection: db.Collection("users"),
	}
}

// Register creates a new user account
func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	// Check if user already exists
	var existingUser models.User
	err := s.collection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&existingUser)
	if err == nil {
		return nil, errors.New("user with this email already exists")
	}
	if err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Hash the password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, errors.New("failed to hash password")
	}

	// Create new user
	user := models.NewUser(req.Email, string(hashedPassword), req.Name)

	// Insert user into database
	_, err = s.collection.InsertOne(ctx, user)
	if err != nil {
		return nil, errors.New("failed to create user")
	}

	// Generate JWT token
	token, err := utils.GenerateToken(user)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &models.AuthResponse{
		User:  user.ToUserResponse(),
		Token: token,
	}, nil
}

// Login authenticates a user and returns a token
func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	// Find user by email
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"email": req.Email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("invalid email or password")
		}
		return nil, err
	}

	// Verify password
	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password))
	if err != nil {
		return nil, errors.New("invalid email or password")
	}

	// Generate JWT token
	token, err := utils.GenerateToken(&user)
	if err != nil {
		return nil, errors.New("failed to generate token")
	}

	return &models.AuthResponse{
		User:  user.ToUserResponse(),
		Token: token,
	}, nil
}

// GetUserByID retrieves a user by ID
func (s *AuthService) GetUserByID(ctx context.Context, userID string) (*models.User, error) {
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"_id": userID}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *AuthService) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := s.collection.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("user not found")
		}
		return nil, err
	}
	return &user, nil
}
