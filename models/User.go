package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        string    `json:"id" bson:"_id"`
	Email     string    `json:"email" bson:"email"`
	Password  string    `json:"-" bson:"password"` // "-" means this field won't be included in JSON responses
	Name      string    `json:"name" bson:"name"`
	Role      string    `json:"role" bson:"role"` // "content", "basic", or empty
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// NewUser creates a new user with a generated UUID
func NewUser(email, password, name string) *User {
	now := time.Now()
	return &User{
		ID:        uuid.New().String(),
		Email:     email,
		Password:  password, // This should be hashed before saving
		Name:      name,
		Role:      "basic",  // Default role is "basic"
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewUserWithRole creates a new user with a specific role
func NewUserWithRole(email, password, name, role string) *User {
	now := time.Now()
	return &User{
		ID:        uuid.New().String(),
		Email:     email,
		Password:  password, // This should be hashed before saving
		Name:      name,
		Role:      role,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// IsContentCreator checks if user has content creator role
func (u *User) IsContentCreator() bool {
	return u.Role == "content"
}

// HasRole checks if user has a specific role
func (u *User) HasRole(role string) bool {
	return u.Role == role
}