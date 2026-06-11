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

// GetRecentRuns fetches recent test execution updates. Supports search queries and metadata filtering.
func (h *APIHandler) GetRecentRuns(c *gin.Context) {
	filter := bson.M{}
	
	if project := c.Query("project"); project != "" {
		filter["project"] = project
	}
	if release := c.Query("release"); release != "" {
		filter["release"] = release
	}
	if sprint := c.Query("sprint"); sprint != "" {
		filter["sprint"] = sprint
	}
	if search := c.Query("search"); search != "" {
		filter["$or"] = []bson.M{
			{"test_case_name": bson.M{"$regex": search, "$options": "i"}},
			{"error_message": bson.M{"$regex": search, "$options": "i"}},
			{"suite_name": bson.M{"$regex": search, "$options": "i"}},
			{"execution_id": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	cursor, err := h.Collection.Find(c.Request.Context(), filter, options.Find().SetLimit(100).SetSort(bson.D{{Key: "timestamp", Value: -1}}))
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

// GetStats processes system stats calculations and aggregations based on selected filters.
func (h *APIHandler) GetStats(c *gin.Context) {
	match := bson.M{}
	if project := c.Query("project"); project != "" {
		match["project"] = project
	}
	if release := c.Query("release"); release != "" {
		match["release"] = release
	}
	if sprint := c.Query("sprint"); sprint != "" {
		match["sprint"] = sprint
	}
	if search := c.Query("search"); search != "" {
		match["$or"] = []bson.M{
			{"test_case_name": bson.M{"$regex": search, "$options": "i"}},
			{"error_message": bson.M{"$regex": search, "$options": "i"}},
			{"suite_name": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	// Status Distribution Aggregate
	statusPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$status"},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}}},
	}

	cursor, err := h.Collection.Aggregate(c.Request.Context(), statusPipeline)
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

	// Unique Suite Executions Count
	suitePipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$execution_id"},
		}}},
		bson.D{{Key: "$count", Value: "total"}},
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

	// Aggregate Project Metric Distributions for Advanced Analytics Charts (exclude project filter to keep all projects visible in sidebar)
	projectMatch := bson.M{}
	if release := c.Query("release"); release != "" {
		projectMatch["release"] = release
	}
	if sprint := c.Query("sprint"); sprint != "" {
		projectMatch["sprint"] = sprint
	}
	if search := c.Query("search"); search != "" {
		projectMatch["$or"] = []bson.M{
			{"test_case_name": bson.M{"$regex": search, "$options": "i"}},
			{"error_message": bson.M{"$regex": search, "$options": "i"}},
			{"suite_name": bson.M{"$regex": search, "$options": "i"}},
		}
	}

	projectAggPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: projectMatch}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$project"},
			{Key: "avg_duration", Value: bson.D{{Key: "$avg", Value: "$duration_ms"}}},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "passed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$status", "PASS"}}}, 1, 0}},
			}}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}}, // Sort to prevent list flickering and reordering
	}
	projCursor, err := h.Collection.Aggregate(c.Request.Context(), projectAggPipeline)
	var projectMetrics []bson.M
	if err == nil {
		defer projCursor.Close(c.Request.Context())
		_ = projCursor.All(c.Request.Context(), &projectMetrics)
	}

	// Aggregate Sprint Trends Distributions for Advanced Analytics
	sprintAggPipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: match}},
		bson.D{{Key: "$group", Value: bson.D{
			{Key: "_id", Value: "$sprint"},
			{Key: "avg_duration", Value: bson.D{{Key: "$avg", Value: "$duration_ms"}}},
			{Key: "total", Value: bson.D{{Key: "$sum", Value: 1}}},
			{Key: "passed", Value: bson.D{{Key: "$sum", Value: bson.D{
				{Key: "$cond", Value: bson.A{bson.D{{Key: "$eq", Value: bson.A{"$status", "PASS"}}}, 1, 0}},
			}}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}}, // Sort to stabilize line chart time series ordering
	}
	sprintCursor, err := h.Collection.Aggregate(c.Request.Context(), sprintAggPipeline)
	var sprintMetrics []bson.M
	if err == nil {
		defer sprintCursor.Close(c.Request.Context())
		_ = sprintCursor.All(c.Request.Context(), &sprintMetrics)
	}

	// Collect list of unique Projects, Releases, Sprints for filter dropdown UI elements
	projects, _ := h.Collection.Distinct(c.Request.Context(), "project", bson.M{})
	releases, _ := h.Collection.Distinct(c.Request.Context(), "release", bson.M{})
	sprints, _ := h.Collection.Distinct(c.Request.Context(), "sprint", bson.M{})

	c.JSON(http.StatusOK, gin.H{
		"status_distribution": stats,
		"total_executions":    totalSuites,
		"project_metrics":     projectMetrics,
		"sprint_metrics":      sprintMetrics,
		"filter_options": gin.H{
			"projects": projects,
			"releases": releases,
			"sprints":  sprints,
		},
	})
}
