package settings

import (
	"context"
	"log"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"siphon-analyzer/internal/analyzer"
)

// Store manages LLM provider configuration, caching settings from MongoDB.
// The analyzer polls this store so hot config changes (made via the UI) take
// effect within one cache TTL without restarting the service.
type Store struct {
	collection *mongo.Collection
	mu         sync.RWMutex
	cached     *analyzer.ProviderConfig
	cachedAt   time.Time
	ttl        time.Duration
}

// New creates a new Store backed by the siphon_settings MongoDB collection.
func New(db *mongo.Database) *Store {
	col := db.Collection("siphon_settings")
	// Ensure unique singleton index
	_, _ = col.Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys:    bson.D{{Key: "singleton", Value: 1}},
		Options: options.Index().SetUnique(true),
	})
	return &Store{collection: col, ttl: 30 * time.Second}
}

// Load returns the current provider config, using the cache if fresh.
func (s *Store) Load(ctx context.Context) (*analyzer.ProviderConfig, error) {
	s.mu.RLock()
	if s.cached != nil && time.Since(s.cachedAt) < s.ttl {
		cfg := *s.cached
		s.mu.RUnlock()
		return &cfg, nil
	}
	s.mu.RUnlock()

	// Cache miss — fetch from MongoDB
	var doc struct {
		Provider string `bson:"provider"`
		APIKey   string `bson:"api_key"`
		Model    string `bson:"model"`
		BaseURL  string `bson:"base_url"`
	}
	err := s.collection.FindOne(ctx, bson.M{"singleton": "llm_config"}).Decode(&doc)
	if err == mongo.ErrNoDocuments {
		log.Println("Settings: No LLM config found in MongoDB. Configure via the siphon-glass Settings panel.")
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	cfg := &analyzer.ProviderConfig{
		Provider: doc.Provider,
		APIKey:   doc.APIKey,
		Model:    doc.Model,
		BaseURL:  doc.BaseURL,
	}

	s.mu.Lock()
	s.cached = cfg
	s.cachedAt = time.Now()
	s.mu.Unlock()

	return cfg, nil
}

// Invalidate clears the in-memory cache, forcing the next Load to re-read MongoDB.
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.cached = nil
	s.mu.Unlock()
}
