package handler

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"siphon-stream-api/internal/hub"
)

// APIHandler processes REST requests.
type APIHandler struct {
	Collection *mongo.Collection
	Hub        *hub.Hub
}

// HandleWS handles WebSocket connections.
func (h *APIHandler) HandleWS(c *gin.Context) {
	ws, err := hub.Upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade connection: %v", err)
		return
	}
	h.Hub.Register <- ws

	go func() {
		defer func() {
			h.Hub.Unregister <- ws
		}()
		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

// GetRecentRuns fetches recent test execution updates.
func (h *APIHandler) GetRecentRuns(c *gin.Context) {
	cursor, err := h.Collection.Find(c.Request.Context(), bson.M{}, options.Find().SetLimit(100).SetSort(bson.D{{Key: "timestamp", Value: -1}}))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(c.Request.Context())

	var results []bson.M
	if err := cursor.All(c.Request.Context(), &results); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetStats processes system stats calculations.
func (h *APIHandler) GetStats(c *gin.Context) {
	pipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$status"},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}},
		},
	}

	cursor, err := h.Collection.Aggregate(c.Request.Context(), pipeline)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cursor.Close(c.Request.Context())

	var stats []bson.M
	if err := cursor.All(c.Request.Context(), &stats); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	suitePipeline := mongo.Pipeline{
		bson.D{
			{Key: "$group", Value: bson.D{
				{Key: "_id", Value: "$execution_id"},
			}},
		},
		bson.D{
			{Key: "$count", Value: "total"},
		},
	}
	suiteCursor, err := h.Collection.Aggregate(c.Request.Context(), suitePipeline)
	var totalSuites int32 = 0
	if err == nil {
		defer suiteCursor.Close(c.Request.Context())
		if suiteCursor.Next(c.Request.Context()) {
			var res bson.M
			if err := suiteCursor.Decode(&res); err == nil {
				if val, ok := res["total"].(int32); ok {
					totalSuites = val
				} else if valFloat, ok := res["total"].(float64); ok {
					totalSuites = int32(valFloat)
				} else if valInt64, ok := res["total"].(int64); ok {
					totalSuites = int32(valInt64)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status_distribution": stats,
		"total_executions":    totalSuites,
	})
}
