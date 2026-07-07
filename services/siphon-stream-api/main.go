package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/gin-gonic/gin"

	"siphon-stream-api/internal/handler"
	"siphon-stream-api/internal/hub"
	"siphon-stream-api/internal/infra"
	"shared/telemetry"
)

func main() {
	tp, err := telemetry.InitTracer("siphon-stream-api")
	if err != nil {
		log.Printf("Warning: Failed to init OpenTelemetry: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
	}

	mongoClient, err := infra.NewMongoClient()
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Close()

	hubInstance := hub.NewHub()
	go hubInstance.Run()

	// Watch DB Updates using change streams fallback pollers
	go hubInstance.TailMongoChangeStream(context.Background(), mongoClient.Collection)

	apiHandler := &handler.APIHandler{
		Collection: mongoClient.Collection,
		Hub:        hubInstance,
		DB:         mongoClient.DB,
	}

	r := gin.Default()

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// Core endpoints
	r.GET("/ws", apiHandler.HandleWS)
	r.GET("/api/runs", apiHandler.GetRecentRuns)
	r.GET("/api/stats", apiHandler.GetStats)

	// AI Analysis endpoint
	r.GET("/api/runs/:execution_id/:test_case_id/analysis", apiHandler.GetTestAnalysis)

	// LLM Settings endpoints
	r.GET("/api/settings", apiHandler.GetSettings)
	r.POST("/api/settings", apiHandler.SaveSettings)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("siphon-stream-api listening on port %s...\n", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to run HTTP stream api: %v", err)
	}
}
