package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc"

	"siphon-ingress/internal/infra"
	"siphon-ingress/internal/service"
	pb "shared/proto/siphon"
	"shared/telemetry"
)

func main() {
	tp, err := telemetry.InitTracer("siphon-ingress")
	if err != nil {
		log.Printf("Warning: Failed to init OpenTelemetry: %v", err)
	} else {
		defer tp.Shutdown(context.Background())
	}

	rabbit, err := infra.NewRabbitClient()
	if err != nil {
		log.Fatalf("Could not connect to RabbitMQ: %v", err)
	}
	defer rabbit.Close()

	port := os.Getenv("PORT")
	if port == "" {
		port = "50051"
	}
	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("Failed to listen on port %s: %v", port, err)
	}

	grpcServer := grpc.NewServer(
		grpc.StatsHandler(otelgrpc.NewServerHandler()),
	)

	ingressSvc := &service.IngressService{
		RabbitChan: rabbit.Channel,
		QueueName:  "siphon-buffer-queue",
	}
	pb.RegisterIngressServiceServer(grpcServer, ingressSvc)

	fmt.Printf("siphon-ingress listening on port %s...\n", port)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("Failed to run gRPC Ingress server: %v", err)
	}
}
