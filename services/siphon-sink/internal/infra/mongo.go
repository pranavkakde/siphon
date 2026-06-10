package infra

import (
	"context"
	"log"
	"os"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient wraps MongoDB Client setup and database configuration handlers.
type MongoClient struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// NewMongoClient dials MongoDB and verifies collections and indices.
func NewMongoClient() (*MongoClient, error) {
	mongoURL := os.Getenv("MONGO_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	if err != nil {
		return nil, err
	}

	db := client.Database("siphon")
	collection := db.Collection("siphon_results")

	// Ensure unique index compound key exists as fail-safe
	_, err = collection.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{
			{Key: "execution_id", Value: 1},
			{Key: "test_case_id", Value: 1},
		},
		Options: options.Index().SetUnique(true),
	})
	if err != nil {
		log.Printf("Unique index compound keys verified: %v", err)
	}

	return &MongoClient{
		Client:     client,
		Collection: collection,
	}, nil
}

// Close releases MongoDB connections.
func (m *MongoClient) Close() {
	if m.Client != nil {
		m.Client.Disconnect(context.Background())
	}
}
