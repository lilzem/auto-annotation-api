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
		CreatedAt: now,
		UpdatedAt: now,
	}
}