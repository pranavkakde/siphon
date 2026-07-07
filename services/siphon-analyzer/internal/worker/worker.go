package worker

import (
	"context"
	"encoding/json"
	"log"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"siphon-analyzer/internal/analyzer"
	"siphon-analyzer/internal/settings"
)

// AnalysisJob matches the JSON published by siphon-sink's fan-out.
type AnalysisJob struct {
	ExecutionID  string `json:"execution_id"`
	TestCaseID   string `json:"test_case_id"`
	TestCaseName string `json:"test_case_name"`
	ErrorMessage string `json:"error_message"`
	DOMSnapshot  string `json:"dom_snapshot"`
	HARData      string `json:"har_data"`
	ErrorTrace   string `json:"error_trace"`
}

// AnalysisWorker consumes from the analysis queue, calls the LLM, and writes
// the result back to the MongoDB document.
type AnalysisWorker struct {
	Collection    *mongo.Collection
	Analyzer      *analyzer.Analyzer
	SettingsStore *settings.Store
}

// ProcessMessages runs the analysis pipeline for every incoming job.
func (w *AnalysisWorker) ProcessMessages(workerID int, msgs <-chan amqp.Delivery) {
	for d := range msgs {
		ctx := context.Background()

		var job AnalysisJob
		if err := json.Unmarshal(d.Body, &job); err != nil {
			log.Printf("Analyzer Worker %d: malformed job JSON: %v — routing to DLQ", workerID, err)
			d.Nack(false, false)
			continue
		}

		log.Printf("Analyzer Worker %d: processing %s/%s", workerID, job.ExecutionID, job.TestCaseID)

		// Load current LLM config (cached, re-reads MongoDB every 30s)
		cfg, err := w.SettingsStore.Load(ctx)
		if err != nil || cfg == nil || cfg.APIKey == "" {
			log.Printf("Analyzer Worker %d: LLM not configured (set via siphon-glass Settings). Skipping.", workerID)
			// Re-queue so the job retries when config is eventually set
			d.Nack(false, true)
			continue
		}

		// Run LLM analysis
		result, analysisErr := w.Analyzer.Analyze(ctx, *cfg, analyzer.PromptData{
			TestCaseName: job.TestCaseName,
			ErrorMessage: job.ErrorMessage,
			DOMSnapshot:  job.DOMSnapshot,
			HARData:      job.HARData,
			ErrorTrace:   job.ErrorTrace,
		})

		filter := bson.M{
			"execution_id": job.ExecutionID,
			"test_case_id": job.TestCaseID,
		}

		if analysisErr != nil {
			log.Printf("Analyzer Worker %d: LLM error for %s/%s: %v", workerID, job.ExecutionID, job.TestCaseID, analysisErr)
			// Mark as error state so the UI shows a graceful fallback
			_, _ = w.Collection.UpdateOne(ctx, filter, bson.M{
				"$set": bson.M{"ai_status": "error", "ai_error": analysisErr.Error()},
			})
			d.Ack(false) // Don't re-queue — it will keep failing with the same input
			continue
		}

		// Write analysis back to the existing MongoDB document
		_, updateErr := w.Collection.UpdateOne(ctx, filter, bson.M{
			"$set": bson.M{
				"ai_analysis": result,
				"ai_status":   "done",
			},
		})
		if updateErr != nil {
			log.Printf("Analyzer Worker %d: MongoDB write-back failed: %v", workerID, updateErr)
			d.Nack(false, true)
			continue
		}

		log.Printf("Analyzer Worker %d: ✓ %s — category=%s confidence=%.2f",
			workerID, job.TestCaseName, result.Category, result.Confidence)
		d.Ack(false)
	}
}
