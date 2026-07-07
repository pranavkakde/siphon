package infra

import (
	"context"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient wraps the analyzer's MongoDB connection.
type MongoClient struct {
	Client     *mongo.Client
	DB         *mongo.Database
	Collection *mongo.Collection
}

// NewMongoClient connects to MongoDB and returns handles for both the
// results collection and the parent database (for the settings collection).
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

	return &MongoClient{
		Client:     client,
		DB:         db,
		Collection: collection,
	}, nil
}

// Close disconnects the Mongo client.
func (m *MongoClient) Close() {
	if m.Client != nil {
		m.Client.Disconnect(context.Background())
	}
}
