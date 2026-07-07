package main

import (
	"context"
	"fmt"
	"log"
	"runtime"

	"siphon-analyzer/internal/analyzer"
	"siphon-analyzer/internal/infra"
	"siphon-analyzer/internal/settings"
	"siphon-analyzer/internal/worker"
	"shared/telemetry"
)

func main() {
	tp, err := telemetry.InitTracer("siphon-analyzer")
	if err != nil {
		log.Printf("Warning: Failed to init OpenTelemetry: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
	}

	// Connect to MongoDB
	mongoClient, err := infra.NewMongoClient()
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Close()

	// Connect to RabbitMQ
	rabbit, err := infra.NewRabbitClient()
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}
	defer rabbit.Close()

	// Set prefetch to 1 per worker — LLM calls are slow, no point pre-fetching
	if err := rabbit.Channel.Qos(1, 0, false); err != nil {
		log.Fatalf("Failed to set QoS: %v", err)
	}

	msgs, err := rabbit.Channel.Consume(
		"siphon-analysis-queue",
		"",
		false, // manual ack
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register analyzer consumer: %v", err)
	}

	// Build shared components
	llmAnalyzer, err := analyzer.New()
	if err != nil {
		log.Fatalf("Failed to initialise LLM analyzer: %v", err)
	}

	settingsStore := settings.New(mongoClient.DB)

	numWorkers := runtime.NumCPU()
	fmt.Printf("siphon-analyzer starting with %d workers...\n", numWorkers)
	fmt.Println("Configure your LLM provider via the siphon-glass Settings panel.")

	analysisWorker := &worker.AnalysisWorker{
		Collection:    mongoClient.Collection,
		Analyzer:      llmAnalyzer,
		SettingsStore: settingsStore,
	}

	for w := 1; w <= numWorkers; w++ {
		go analysisWorker.ProcessMessages(w, msgs)
	}

	select {}
}
