package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TestStep struct {
	Name       string `bson:"name"`
	Status     string `bson:"status"`
	DurationMs int64  `bson:"duration_ms"`
}

type TestRun struct {
	ExecutionID  string     `bson:"execution_id"`
	TestCaseID   string     `bson:"test_case_id"`
	SuiteID      string     `bson:"suite_id"`
	SuiteName    string     `bson:"suite_name"`
	Environment  string     `bson:"environment"`
	Project      string     `bson:"project"`
	Release      string     `bson:"release"`
	Sprint       string     `bson:"sprint"`
	Timestamp    time.Time  `bson:"timestamp"`
	TestCaseName string     `bson:"test_case_name"`
	Status       string     `bson:"status"`
	DurationMs   int64      `bson:"duration_ms"`
	ErrorMessage string     `bson:"error_message,omitempty"`
	ScreenshotURL string     `bson:"screenshot_url,omitempty"`
	Steps        []TestStep `bson:"steps"`
	UpdatedAt    time.Time  `bson:"updated_at"`
}

func main() {
	log.Println("Starting database seeding process (1000s of test records)...")

	mongoURL := os.Getenv("MONGO_URL")
	if mongoURL == "" {
		mongoURL = "mongodb://localhost:27017"
	}

	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(mongoURL))
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer client.Disconnect(context.Background())

	db := client.Database("siphon")
	
	// Drop the legacy non-timeseries collection if it exists
	_ = db.Collection("siphon_results").Drop(context.Background())

	// Create Time-Series collection
	err = db.CreateCollection(context.Background(), "siphon_results", 
		options.CreateCollection().SetTimeSeriesOptions(
			options.TimeSeries().SetTimeField("timestamp").SetMetaField("project").SetGranularity("seconds"),
		),
	)
	if err != nil {
		log.Printf("Collection creation log info: %v", err)
	}

	collection := db.Collection("siphon_results")

	projects := []string{"siphon-core", "payment-gateway", "auth-service", "notifications-engine"}
	releases := map[string][]string{
		"siphon-core":          {"v1.0.0", "v1.1.0", "v1.2.0-rc1"},
		"payment-gateway":      {"v2.4.0", "v2.5.0"},
		"auth-service":         {"v0.9.0", "v1.0.0"},
		"notifications-engine": {"v1.0.1", "v1.1.0"},
	}
	sprints := []string{"Sprint 23", "Sprint 24", "Sprint 25", "Sprint 26"}
	environments := []string{"Staging-US-East", "Production-US-West", "QA-Integration"}

	testNames := []string{
		"Validate JWT Signature Verification Flow",
		"Verify Stripe Payment Charge Capturing API",
		"Download PDF Invoice Report Stream",
		"User Authentication login form validation",
		"Flush Cache Redis Connection Pool State",
		"Render Admin Dashboard settings grids",
		"Dispatch Transactional Email Notifications",
		"Verify OAuth2 Single-Sign-On Callback Payload",
		"Process Bulk CSV Data Import parsing",
		"Check Health Check endpoints network status",
	}

	random := rand.New(rand.NewSource(time.Now().UnixNano()))
	batchSize := 1250
	operations := make([]mongo.WriteModel, batchSize)

	log.Printf("Generating %d records across projects and sprints...", batchSize)

	for i := 0; i < batchSize; i++ {
		project := projects[random.Intn(len(projects))]
		projReleases := releases[project]
		release := projReleases[random.Intn(len(projReleases))]
		sprint := sprints[random.Intn(len(sprints))]
		env := environments[random.Intn(len(environments))]

		execID := fmt.Sprintf("exec-%d", 10000+random.Intn(90000))
		suiteID := fmt.Sprintf("suite-%d", 100+random.Intn(900))
		testCaseID := fmt.Sprintf("tc-%d", 1000+random.Intn(9000))

		statusVal := random.Float32()
		status := "PASS"
		var duration int64 = int64(100 + random.Intn(800))
		var errMsg string
		var screenshot string
		var steps []TestStep

		if statusVal < 0.15 { // 15% FAIL rate
			status = "FAIL"
			duration = int64(800 + random.Intn(2000))
			errMsg = "Stripe webhook payment authorization failure: connection timeout"
			screenshot = fmt.Sprintf("http://localhost:9000/siphon-screenshots/failure-%d.png", random.Int63n(10000000))
			steps = []TestStep{
				{Name: "Initialize request context", Status: "PASS", DurationMs: 15},
				{Name: "Check local credit limits", Status: "PASS", DurationMs: 30},
				{Name: "Call external payment token handler", Status: "FAIL", DurationMs: duration - 45},
			}
		} else if statusVal < 0.23 { // 8% SKIPPED rate
			status = "SKIPPED"
			duration = 0
			steps = []TestStep{}
		} else { // 77% PASS rate
			steps = []TestStep{
				{Name: "Initialize request context", Status: "PASS", DurationMs: 10},
				{Name: "Perform system DB validations", Status: "PASS", DurationMs: duration - 10},
			}
		}

		// Distribute runs across the last 15 days
		daysAgo := random.Intn(15)
		timestamp := time.Now().AddDate(0, 0, -daysAgo).Add(time.Duration(random.Intn(24)) * time.Hour)

		run := TestRun{
			ExecutionID:   execID,
			TestCaseID:    testCaseID,
			SuiteID:       suiteID,
			SuiteName:     "Functional E2E Suite - " + project,
			Environment:   env,
			Project:       project,
			Release:       release,
			Sprint:        sprint,
			Timestamp:     timestamp,
			TestCaseName:  testNames[random.Intn(len(testNames))],
			Status:        status,
			DurationMs:    duration,
			ErrorMessage:  errMsg,
			ScreenshotURL: screenshot,
			Steps:         steps,
			UpdatedAt:     time.Now(),
		}

		model := mongo.NewInsertOneModel().SetDocument(run)
		operations[i] = model
	}

	opts := options.BulkWrite().SetOrdered(false)
	result, err := collection.BulkWrite(context.Background(), operations, opts)
	if err != nil {
		log.Fatalf("Failed to execute bulk write seeder: %v", err)
	}

	log.Printf("Seeding complete! Inserted %d records successfully into time-series collection.", result.InsertedCount)
}
