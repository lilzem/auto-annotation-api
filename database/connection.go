package database

import (
	"context"
	"crypto/tls"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
	client   *mongo.Client
	database *mongo.Database
)

// Connect establishes a connection to MongoDB
func Connect(mongoURI, databaseName string) (*mongo.Database, error) {
	// Set client options
	clientOptions := options.Client().ApplyURI(mongoURI)

	// Configure TLS for MongoDB Atlas
	tlsConfig := &tls.Config{
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS12,
	}
	clientOptions.SetTLSConfig(tlsConfig)

	// Set timeout - increase for Atlas
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Connect to MongoDB
	var err error
	client, err = mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, err
	}

	// Ping the database to verify connection
	err = client.Ping(ctx, nil)
	if err != nil {
		return nil, err
	}

	log.Printf("Successfully connected to MongoDB database: %s", databaseName)

	// Get database instance
	database = client.Database(databaseName)

	return database, nil
}

// GetDatabase returns the database instance
func GetDatabase() *mongo.Database {
	return database
}

// GetClient returns the MongoDB client
func GetClient() *mongo.Client {
	return client
}

// Disconnect closes the MongoDB connection
func Disconnect() error {
	if client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		err := client.Disconnect(ctx)
		if err != nil {
			log.Printf("Error disconnecting from MongoDB: %v", err)
			return err
		}

		log.Println("Disconnected from MongoDB")
	}
	return nil
}
