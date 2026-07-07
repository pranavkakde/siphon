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

type AIAnalysis struct {
	Category     string    `bson:"category"`
	Confidence   float64   `bson:"confidence"`
	RootCause    string    `bson:"root_cause"`
	SuggestedFix string    `bson:"suggested_fix"`
	AnalyzedAt   time.Time `bson:"analyzed_at"`
	Model        string    `bson:"model"`
	Provider     string    `bson:"provider"`
}

type TestRun struct {
	ExecutionID   string     `bson:"execution_id"`
	TestCaseID    string     `bson:"test_case_id"`
	SuiteID       string     `bson:"suite_id"`
	SuiteName     string     `bson:"suite_name"`
	Environment   string     `bson:"environment"`
	Project       string     `bson:"project"`
	Release       string     `bson:"release"`
	Sprint        string     `bson:"sprint"`
	Timestamp     time.Time  `bson:"timestamp"`
	TestCaseName  string     `bson:"test_case_name"`
	Status        string     `bson:"status"`
	DurationMs    int64      `bson:"duration_ms"`
	ErrorMessage  string     `bson:"error_message,omitempty"`
	ScreenshotURL string     `bson:"screenshot_url,omitempty"`
	Steps         []TestStep `bson:"steps"`
	// Rich contextual artifact fields
	DOMSnapshot string `bson:"dom_snapshot,omitempty"`
	HARData     string `bson:"har_data,omitempty"`
	ErrorTrace  string `bson:"error_trace,omitempty"`
	// AI analysis fields
	AIStatus   string      `bson:"ai_status,omitempty"`
	AIAnalysis *AIAnalysis `bson:"ai_analysis,omitempty"`
	UpdatedAt  time.Time   `bson:"updated_at"`
}

// Synthetic DOM snapshots for demo purposes
var syntheticDOMSnapshots = []string{
	`<div id="checkout-form"><button id="submit-payment-btn" class="btn btn-primary" disabled>Pay Now</button><span class="error-msg">Payment gateway timeout</span></div>`,
	`<div class="login-container"><input id="username-field" type="text" /><button id="login-btn" class="btn" aria-disabled="true">Sign In</button></div>`,
	`<table id="data-grid"><tbody><tr class="row-stale" data-version="2023-01-01"><!-- stale cached data --></tr></tbody></table>`,
	`<div id="api-response-panel"><pre class="error">{"error":"503 Service Unavailable","message":"Upstream API unreachable"}</pre></div>`,
	`<form id="notification-form"><select id="recipient-select" disabled></select><p class="hint">Recipients list failed to load</p></form>`,
}

// Synthetic HAR data (failed requests only) for demo purposes
var syntheticHARData = []string{
	`{"log":{"entries":[{"request":{"method":"POST","url":"https://api.stripe.com/v1/payment_intents"},"response":{"status":408,"statusText":"Request Timeout"},"timings":{"wait":29847}}]}}`,
	`{"log":{"entries":[{"request":{"method":"GET","url":"/api/v2/users/me"},"response":{"status":401,"statusText":"Unauthorized"},"timings":{"wait":45}}]}}`,
	`{"log":{"entries":[{"request":{"method":"GET","url":"/api/data/records?page=1"},"response":{"status":200,"statusText":"OK","content":{"mimeType":"application/json","text":"{\"data\":[]}"}},"timings":{"wait":12}}]}}`,
	`{"log":{"entries":[{"request":{"method":"POST","url":"https://notifications.internal/send"},"response":{"status":503,"statusText":"Service Unavailable"},"timings":{"wait":5001}}]}}`,
	`{"log":{"entries":[{"request":{"method":"PUT","url":"/api/settings/cache"},"response":{"status":500,"statusText":"Internal Server Error"},"timings":{"wait":3012}}]}}`,
}

// Synthetic error traces for demo purposes
var syntheticErrorTraces = []string{
	"TimeoutError: Waiting for element '#submit-payment-btn' to be enabled\n  at Object.<anonymous> (tests/checkout.spec.ts:47:5)\n  at processTicksAndRejections (node:internal/process/task_queues:95:5)\nCaused by: net::ERR_CONNECTION_TIMED_OUT",
	"AssertionError: expected 200 but received 401\n  at Context.<anonymous> (tests/auth.spec.ts:23:12)\n  Error: Request to GET /api/v2/users/me returned 401 Unauthorized",
	"Error: Selector '#data-grid tr' returned 0 rows, expected at least 1\n  at DataGrid.waitForRows (lib/helpers.ts:88:3)\n  at Context.<anonymous> (tests/data.spec.ts:61:5)",
	"Error: upstream dependency 'notification-service' unavailable\n  at NotificationTest.send (tests/notifications.spec.ts:34:9)\n  503 Service Unavailable — retry limit (3) exceeded",
	"ReferenceError: Cannot read properties of undefined (reading 'token')\n  at CacheManager.flush (lib/cache.ts:112:17)\n  at Context.<anonymous> (tests/cache.spec.ts:19:5)",
}

// Synthetic AI analyses for seeded FAIL records
var syntheticAIAnalyses = []AIAnalysis{
	{
		Category:     "API_Failure",
		Confidence:   0.92,
		RootCause:    "The Stripe payment intent API returned a 408 timeout after 29 seconds, exceeding the test runner's 30-second limit. The external payment gateway appears to be throttling connections from the CI environment's IP range.",
		SuggestedFix: "// Increase timeout and add retry with backoff\nawait page.waitForResponse(res => res.url().includes('stripe.com'), { timeout: 60000 });\n// Or mock the Stripe endpoint in CI:\nnock('https://api.stripe.com').post('/v1/payment_intents').reply(200, mockPaymentIntent);",
	},
	{
		Category:     "Environment_Issue",
		Confidence:   0.88,
		RootCause:    "The API returned a 401 Unauthorized because the test JWT token expired between setup and assertion — the token TTL is 30 seconds but the test setup takes 35 seconds in CI. This is an environment timing issue, not a code bug.",
		SuggestedFix: "// Refresh the token immediately before the assertion step\nconst token = await authHelper.refreshToken();\nrequest.setHeader('Authorization', `Bearer ${token}`);\n// Or set token TTL to 10 minutes in the test environment config.",
	},
	{
		Category:     "Data_Stale",
		Confidence:   0.85,
		RootCause:    "The data grid returned 0 rows because the test database was seeded with records from 2023, which are filtered out by the default 'last 30 days' date range filter applied in the query. The test data is stale and does not match the active filter.",
		SuggestedFix: "// Seed with a relative timestamp\nconst record = await db.insert({ ...testData, created_at: new Date() });\n// Or override the default filter in the test:\nawait page.selectOption('#date-filter', 'all-time');",
	},
	{
		Category:     "Environment_Issue",
		Confidence:   0.91,
		RootCause:    "The notification service returned 503 after 3 retries because the dependency is not deployed in the QA-Integration environment — it is only available in Staging-US-East. The test should be tagged to skip in unsupported environments.",
		SuggestedFix: "// Add environment guard at test start\nif (process.env.TEST_ENV !== 'Staging-US-East') {\n  test.skip('Notification service not available in this environment');\n}",
	},
	{
		Category:     "Locator_Changed",
		Confidence:   0.79,
		RootCause:    "The CacheManager references a 'token' property that was renamed to 'accessToken' in a recent API contract change. The selector path is still pointing to the old field name, causing a ReferenceError at runtime.",
		SuggestedFix: "// Update the property reference in the test helper\n// Before: const token = cache.token;\n// After:\nconst token = cache.accessToken ?? cache.token; // backward-compat shim\n// Then update the test fixture to use the new field name.",
	},
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
		var domSnapshot string
		var harData string
		var errorTrace string
		var aiStatus string
		var aiAnalysis *AIAnalysis

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
			// Attach synthetic rich artifact data
			artifactIdx := random.Intn(len(syntheticDOMSnapshots))
			domSnapshot = syntheticDOMSnapshots[artifactIdx]
			harData = syntheticHARData[artifactIdx]
			errorTrace = syntheticErrorTraces[artifactIdx]
			// Attach pre-computed AI analysis for demo (70% have analysis, 30% still pending)
			if random.Float32() < 0.70 {
				analysisIdx := random.Intn(len(syntheticAIAnalyses))
				a := syntheticAIAnalyses[analysisIdx]
				a.AnalyzedAt = time.Now().Add(-time.Duration(random.Intn(3600)) * time.Second)
				a.Model = "gpt-4o-mini"
				a.Provider = "openai"
				aiAnalysis = &a
				aiStatus = "done"
			} else {
				aiStatus = "pending"
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
			DOMSnapshot:   domSnapshot,
			HARData:       harData,
			ErrorTrace:    errorTrace,
			AIStatus:      aiStatus,
			AIAnalysis:    aiAnalysis,
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
