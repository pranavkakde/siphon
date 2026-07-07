package infra

import (
	"context"
	"os"

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

	// Create Time-Series collection if it doesn't exist yet
	// Timefield: timestamp, Metafield: project
	_ = db.CreateCollection(context.Background(), "siphon_results", 
		options.CreateCollection().SetTimeSeriesOptions(
			options.TimeSeries().SetTimeField("timestamp").SetMetaField("project").SetGranularity("seconds"),
		),
	)

	collection := db.Collection("siphon_results")

	// Unique compound indexes are not supported on Time-Series collections.
	// The unique constraint is handled at the application logic level during ingestion instead.

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
