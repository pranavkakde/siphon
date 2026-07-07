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
	"google.golang.org/protobuf/proto"

	pb "shared/proto/siphon"
)

// AnalysisJob is the lightweight message published to the analysis queue.
// It contains only what the LLM needs — no heavy protobuf framing.
type AnalysisJob struct {
	ExecutionID  string `json:"execution_id"`
	TestCaseID   string `json:"test_case_id"`
	TestCaseName string `json:"test_case_name,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	DOMSnapshot  string `json:"dom_snapshot,omitempty"`
	HARData      string `json:"har_data,omitempty"`
	ErrorTrace   string `json:"error_trace,omitempty"`
}

// ResultWorker consumes from RabbitMQ and updates MongoDB documents.
type ResultWorker struct {
	Collection      *mongo.Collection
	AnalysisChan    *amqp.Channel
	AnalysisQueue   string
}

// ProcessMessages processes incoming AMQP messages.
func (rw *ResultWorker) ProcessMessages(workerID int, msgs <-chan amqp.Delivery) {
	tr := otel.Tracer("siphon-sink")

	for d := range msgs {
		ctx, span := tr.Start(context.Background(), "SinkProcessMessage")

		var streamReq pb.TestResultStreamRequest
		if err := proto.Unmarshal(d.Body, &streamReq); err != nil {
			log.Printf("Worker %d: Malformed protobuf binary payload: %v. Routing to DLQ.", workerID, err)
			d.Nack(false, false) // Reject and route to DLQ instead of dropping or infinite loops
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
			"project":         streamReq.TestSuite.Project,
			"release":         streamReq.TestSuite.Release,
			"sprint":          streamReq.TestSuite.Sprint,
			"timestamp":       time.Unix(streamReq.TestSuite.Timestamp.GetSeconds(), int64(streamReq.TestSuite.Timestamp.GetNanos())),
			"test_case_name":  streamReq.TestCase.Name,
			"status":          streamReq.TestCase.Status.String(),
			"duration_ms":     streamReq.TestCase.DurationMs,
			"error_message":   streamReq.TestCase.ErrorMessage,
			"screenshot_url":  streamReq.TestCase.ScreenshotUrl,
			"steps":           streamReq.TestSteps,
			// Rich contextual artifact fields
			"dom_snapshot":    streamReq.TestCase.DomSnapshot,
			"har_data":        streamReq.TestCase.HarData,
			"error_trace":     streamReq.TestCase.ErrorTrace,
			"updated_at":      time.Now(),
		}

		// Pre-mark analysis status so the UI can show "Analyzing…" immediately
		if streamReq.TestCase.Status == pb.TestCase_FAIL {
			flatDoc["ai_status"] = "pending"
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
			span.End()
			continue
		}

		d.Ack(false)

		// Fan-out: publish lightweight analysis job to siphon-analysis-queue for any FAIL
		if streamReq.TestCase.Status == pb.TestCase_FAIL && rw.AnalysisChan != nil {
			job := AnalysisJob{
				ExecutionID:  execID,
				TestCaseID:   testCaseID,
				TestCaseName: streamReq.TestCase.Name,
				ErrorMessage: streamReq.TestCase.ErrorMessage,
				DOMSnapshot:  streamReq.TestCase.DomSnapshot,
				HARData:      streamReq.TestCase.HarData,
				ErrorTrace:   streamReq.TestCase.ErrorTrace,
			}
			jobBytes, jsonErr := json.Marshal(job)
			if jsonErr != nil {
				log.Printf("Worker %d: Failed to marshal analysis job: %v", workerID, jsonErr)
			} else {
				pubErr := rw.AnalysisChan.PublishWithContext(
					ctx,
					"",
					rw.AnalysisQueue,
					false,
					false,
					amqp.Publishing{
						ContentType: "application/json",
						Body:        jobBytes,
						Timestamp:   time.Now(),
					},
				)
				if pubErr != nil {
					log.Printf("Worker %d: Failed to publish analysis job: %v", workerID, pubErr)
				} else {
					log.Printf("Worker %d: Analysis job queued for %s/%s", workerID, execID, testCaseID)
				}
			}
		}

		span.End()
	}
}
