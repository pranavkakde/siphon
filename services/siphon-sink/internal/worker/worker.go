package worker

import (
	"context"
	"encoding/json"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.opentelemetry.io/otel"

	pb "shared/proto/siphon"
)

// ResultWorker consumes from RabbitMQ and updates MongoDB documents.
type ResultWorker struct {
	Collection *mongo.Collection
}

// ProcessMessages processes incoming AMQP messages.
func (rw *ResultWorker) ProcessMessages(workerID int, msgs <-chan amqp.Delivery) {
	tr := otel.Tracer("siphon-sink")

	for d := range msgs {
		ctx, span := tr.Start(context.Background(), "SinkProcessMessage")

		var streamReq pb.TestResultStreamRequest
		if err := json.Unmarshal(d.Body, &streamReq); err != nil {
			log.Printf("Worker %d: Malformed json payload: %v", workerID, err)
			d.Ack(false)
			span.RecordError(err)
			span.End()
			continue
		}

		if streamReq.TestSuite == nil || streamReq.TestCase == nil {
			log.Printf("Worker %d: Missing test_suite or test_case parameters", workerID)
			d.Ack(false)
			span.End()
			continue
		}

		execID := streamReq.TestSuite.ExecutionId
		testCaseID := streamReq.TestCase.Id

		flatDoc := bson.M{
			"execution_id":    execID,
			"test_case_id":    testCaseID,
			"suite_id":        streamReq.TestSuite.Id,
			"suite_name":      streamReq.TestSuite.Name,
			"environment":     streamReq.TestSuite.Environment,
			"timestamp":       time.Unix(streamReq.TestSuite.Timestamp.GetSeconds(), int64(streamReq.TestSuite.Timestamp.GetNanos())),
			"test_case_name":  streamReq.TestCase.Name,
			"status":          streamReq.TestCase.Status.String(),
			"duration_ms":     streamReq.TestCase.DurationMs,
			"error_message":   streamReq.TestCase.ErrorMessage,
			"screenshot_url":  streamReq.TestCase.ScreenshotUrl,
			"steps":           streamReq.TestSteps,
			"updated_at":      time.Now(),
		}

		filter := bson.M{
			"execution_id": execID,
			"test_case_id": testCaseID,
		}

		update := bson.M{
			"$set": flatDoc,
		}

		opts := options.Update().SetUpsert(true)
		_, err := rw.Collection.UpdateOne(ctx, filter, update, opts)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				log.Printf("Worker %d: Duplicate key ignored for ExecutionID: %s, TestCaseID: %s", workerID, execID, testCaseID)
				d.Ack(false)
			} else {
				log.Printf("Worker %d: MongoDB update failed: %v", workerID, err)
				d.Nack(false, true)
			}
		} else {
			d.Ack(false)
		}

		span.End()
	}
}
