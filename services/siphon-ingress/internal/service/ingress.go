package service

import (
	"encoding/json"
	"io"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"

	pb "shared/proto/siphon"
)

// IngressService implements pb.IngressServiceServer
type IngressService struct {
	pb.UnimplementedIngressServiceServer
	RabbitChan *amqp.Channel
	QueueName  string
}

// StreamTestResults receives gRPC payload items and forwards them to RabbitMQ buffer queue
func (s *IngressService) StreamTestResults(stream pb.IngressService_StreamTestResultsServer) error {
	ctx := stream.Context()
	tr := otel.Tracer("siphon-ingress")

	for {
		req, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.TestResultStreamResponse{
				Success: true,
				Message: "Stream completed successfully",
			})
		}
		if err != nil {
			log.Printf("Ingress stream receiver error: %v", err)
			return err
		}

		_, span := tr.Start(ctx, "IngestTestResult")

		payload, err := json.Marshal(req)
		if err != nil {
			log.Printf("Failed to marshal test result stream req: %v", err)
			span.RecordError(err)
			span.End()
			continue
		}

		err = s.RabbitChan.PublishWithContext(
			ctx,
			"",
			s.QueueName,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        payload,
				Timestamp:   time.Now(),
			},
		)
		if err != nil {
			log.Printf("Failed to publish message: %v", err)
			span.RecordError(err)
			span.End()
			return err
		}

		span.End()
	}
}
