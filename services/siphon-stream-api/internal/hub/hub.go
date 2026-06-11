package hub

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"
)

var Upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Hub coordinates connection channels and streams.
type Hub struct {
	Clients    sync.Map
	Broadcast  chan []byte
	Register   chan *websocket.Conn
	Unregister chan *websocket.Conn
}

// NewHub creates a new communication hub instance.
func NewHub() *Hub {
	return &Hub{
		Broadcast:  make(chan []byte),
		Register:   make(chan *websocket.Conn),
		Unregister: make(chan *websocket.Conn),
	}
}

// Run listens for client lifecycle events and broadcasts updates.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.Register:
			h.Clients.Store(client, true)
			log.Println("New WebSocket client connected")
		case client := <-h.Unregister:
			if _, ok := h.Clients.Load(client); ok {
				h.Clients.Delete(client)
				client.Close()
				log.Println("WebSocket client disconnected")
			}
		case message := <-h.Broadcast:
			h.Clients.Range(func(key, value interface{}) bool {
				client := key.(*websocket.Conn)
				err := client.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					log.Printf("WS write error: %v", err)
					client.Close()
					h.Clients.Delete(client)
				}
				return true
			})
		}
	}
}

// TailMongoChangeStream listens to DB changes and streams updates.
func (h *Hub) TailMongoChangeStream(ctx context.Context, collection *mongo.Collection) {
	tr := otel.Tracer("siphon-stream-api")

	// Fallback to polling if replica set watch fails
	stream, err := collection.Watch(ctx, mongo.Pipeline{}, options.ChangeStream().SetFullDocument(options.UpdateLookup))
	if err == nil {
		defer stream.Close(ctx)
		for stream.Next(ctx) {
			_, span := tr.Start(ctx, "BroadcastChangeStreamEvent")
			var changeEvent bson.M
			if err := stream.Decode(&changeEvent); err == nil {
				fullDoc, ok := changeEvent["fullDocument"]
				if ok {
					payload, err := json.Marshal(fullDoc)
					if err == nil {
						h.Broadcast <- payload
					}
				}
			}
			span.End()
		}
		return
	}

	log.Printf("MongoDB Change Stream watch failed: %v. Falling back to active database polling loop.", err)
	
	var lastSeen = time.Now().Add(-10 * time.Second)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(1 * time.Second):
			_, span := tr.Start(ctx, "PollDatabaseFallback")
			filter := bson.M{
				"updated_at": bson.M{"$gt": lastSeen},
			}
			cursor, err := collection.Find(ctx, filter)
			if err == nil {
				var items []bson.M
				if err := cursor.All(ctx, &items); err == nil && len(items) > 0 {
					for _, item := range items {
						payload, err := json.Marshal(item)
						if err == nil {
							h.Broadcast <- payload
						}
						
						if t, ok := item["updated_at"].(time.Time); ok {
							if t.After(lastSeen) {
								lastSeen = t
							}
						}
					}
				}
				cursor.Close(ctx)
			}
			span.End()
		}
	}
}
