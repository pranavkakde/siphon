package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math/rand"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "shared/proto/siphon"
)

func generateDummyImage() []byte {
	// Create a simple red/orange image representing a test failure
	width, height := 320, 240
	upLeft := image.Point{0, 0}
	lowRight := image.Point{width, height}
	img := image.NewRGBA(image.Rectangle{upLeft, lowRight})
	
	// Fill with failure color
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			img.Set(x, y, color.RGBA{239, 68, 68, 255}) // Crimson failure
		}
	}

	buf := new(bytes.Buffer)
	err := png.Encode(buf, img)
	if err != nil {
		log.Printf("Failed to encode dummy PNG: %v", err)
	}
	return buf.Bytes()
}

func uploadScreenshotToMinIO() (string, error) {
	endpoint := "localhost:9000"
	accessKeyID := "minioadmin"
	secretAccessKey := "minioadmin"
	useSSL := false

	// Initialize MinIO client
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return "", fmt.Errorf("minio connection error: %v", err)
	}

	bucketName := "siphon-screenshots"
	objectName := fmt.Sprintf("failure-%d.png", time.Now().UnixNano())

	imgBytes := generateDummyImage()
	reader := bytes.NewReader(imgBytes)

	_, err = minioClient.PutObject(context.Background(), bucketName, objectName, reader, int64(len(imgBytes)), minio.PutObjectOptions{
		ContentType: "image/png",
	})
	if err != nil {
		return "", fmt.Errorf("failed upload: %v", err)
	}

	// Construct local public endpoint URL
	url := fmt.Sprintf("http://localhost:9000/%s/%s", bucketName, objectName)
	return url, nil
}

func main() {
	log.Println("Starting mock test execution simulation...")
	
	// 1. Upload failure screenshot
	screenshotURL, err := uploadScreenshotToMinIO()
	if err != nil {
		log.Printf("Warning: Failed to upload screenshot to MinIO: %v", err)
		screenshotURL = "http://localhost:9000/siphon-screenshots/placeholder.png"
	} else {
		log.Printf("MinIO upload complete. Screenshot URL: %s", screenshotURL)
	}

	// 2. Connect to gRPC Ingress API
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("gRPC connection failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewIngressServiceClient(conn)
	stream, err := client.StreamTestResults(context.Background())
	if err != nil {
		log.Fatalf("Failed to open StreamTestResults channel: %v", err)
	}

	executionID := fmt.Sprintf("exec-%d", rand.Intn(100000))
	suiteID := fmt.Sprintf("suite-%d", rand.Intn(1000))
	
	suite := &pb.TestSuite{
		Id:          suiteID,
		Name:        "E2E Checkout Workflow",
		Environment: "Staging-US-East",
		ExecutionId: executionID,
		Timestamp:   timestamppb.Now(),
	}

	// Define test cases: 2 PASS, 1 FAIL, 1 SKIPPED
	testCases := []*pb.TestCase{
		{
			Id:          "test-1",
			SuiteId:     suiteID,
			Name:        "User Authentication Login Session",
			Status:      pb.TestCase_PASS,
			DurationMs:  450,
		},
		{
			Id:          "test-2",
			SuiteId:     suiteID,
			Name:        "Add Item to Cart and Check Inventory",
			Status:      pb.TestCase_PASS,
			DurationMs:  230,
		},
		{
			Id:            "test-3",
			SuiteId:       suiteID,
			Name:          "Submit Stripe Payment Gateway Validation",
			Status:        pb.TestCase_FAIL,
			DurationMs:    1280,
			ErrorMessage:  "Stripe token registration timeout. Status code 504.",
			ScreenshotUrl: screenshotURL,
		},
		{
			Id:          "test-4",
			SuiteId:     suiteID,
			Name:        "Download Invoice PDF",
			Status:      pb.TestCase_SKIPPED,
			DurationMs:  0,
		},
	}

	for _, tc := range testCases {
		// Mock steps
		steps := []*pb.TestStep{
			{Name: "Initialize request context", Status: pb.TestStep_PASS, DurationMs: 10},
			{Name: "Evaluate database constraint rules", Status: pb.TestStep_PASS, DurationMs: 50},
		}
		if tc.Status == pb.TestCase_FAIL {
			steps = append(steps, &pb.TestStep{
				Name:       "Call Stripe Charge Intent Endpoint",
				Status:     pb.TestStep_FAIL,
				DurationMs: 1220,
			})
		}

		req := &pb.TestResultStreamRequest{
			TestSuite: suite,
			TestCase:  tc,
			TestSteps: steps,
		}

		if err := stream.Send(req); err != nil {
			log.Fatalf("Failed to push test result packet: %v", err)
		}
		log.Printf("Sent test result: %s [%s]", tc.Name, tc.Status)
		time.Sleep(300 * time.Millisecond)
	}

	res, err := stream.CloseAndRecv()
	if err != nil {
		log.Fatalf("Failed to close and receive gRPC stream: %v", err)
	}

	log.Printf("Server Response: Success=%t, Message=%s", res.Success, res.Message)
	log.Println("Mock test suite execution run complete.")
}
