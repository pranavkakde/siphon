package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"siphon-sink/internal/infra"
	"siphon-sink/internal/worker"
	"shared/telemetry"
)

func main() {
	tp, err := telemetry.InitTracer("siphon-sink")
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

	// RabbitMQ connection
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("Connecting to RabbitMQ (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	// Ingestion consumer channel
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open RabbitMQ channel: %v", err)
	}
	defer ch.Close()

	// Separate channel for publishing to analysis queue (prevents head-of-line blocking)
	analysisCh, err := conn.Channel()
	if err != nil {
		log.Printf("Warning: Failed to open analysis channel, AI analysis disabled: %v", err)
		analysisCh = nil
	} else {
		defer analysisCh.Close()
	}

	// Declare ingestion queue
	qName := "siphon-buffer-queue"
	_, err = ch.QueueDeclare(
		qName,
		true,
		false,
		false,
		false,
		amqp.Table{
			"x-dead-letter-exchange":    "siphon-dlx-exchange",
			"x-dead-letter-routing-key": "siphon-routing-dlkey",
		},
	)
	if err != nil {
		log.Fatalf("Failed to declare queue: %v", err)
	}

	// Declare analysis queue (durable, so jobs survive restarts)
	analysisQName := "siphon-analysis-queue"
	if analysisCh != nil {
		_, err = analysisCh.QueueDeclare(
			analysisQName,
			true,
			false,
			false,
			false,
			nil,
		)
		if err != nil {
			log.Printf("Warning: Failed to declare analysis queue, AI analysis disabled: %v", err)
			analysisCh = nil
		}
	}

	err = ch.Qos(100, 0, false)
	if err != nil {
		log.Fatalf("Failed to set Qos: %v", err)
	}

	msgs, err := ch.Consume(
		qName,
		"",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register consumer: %v", err)
	}

	numCPUs := runtime.NumCPU()
	fmt.Printf("Starting worker pool subscriber. CPU workers running: %d\n", numCPUs)

	resultWorker := &worker.ResultWorker{
		Collection:    mongoClient.Collection,
		AnalysisChan:  analysisCh,
		AnalysisQueue: analysisQName,
	}

	for w := 1; w <= numCPUs; w++ {
		go resultWorker.ProcessMessages(w, msgs)
	}

	select {}
}
