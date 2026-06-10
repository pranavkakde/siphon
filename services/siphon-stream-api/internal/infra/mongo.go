package infra

import (
	"context"
	"os"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MongoClient wraps database connectivity for siphon-stream-api.
type MongoClient struct {
	Client     *mongo.Client
	Collection *mongo.Collection
}

// NewMongoClient establishes database connectivity.
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
		Collection: collection,
	}, nil
}

// Close disconnects the Mongo instance.
func (m *MongoClient) Close() {
	if m.Client != nil {
		m.Client.Disconnect(context.Background())
	}
}
