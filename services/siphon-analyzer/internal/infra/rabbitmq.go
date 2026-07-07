package infra

import (
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient wraps a RabbitMQ connection and channel for the analyzer consumer.
type RabbitClient struct {
	conn    *amqp.Connection
	Channel *amqp.Channel
}

// NewRabbitClient dials RabbitMQ with retries and declares the analysis queue.
func NewRabbitClient() (*RabbitClient, error) {
	rabbitURL := os.Getenv("RABBITMQ_URL")
	if rabbitURL == "" {
		rabbitURL = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	var err error
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		log.Printf("Analyzer: Connecting to RabbitMQ (attempt %d/10): %v", i+1, err)
		time.Sleep(3 * time.Second)
	}
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	// Declare the queue (idempotent — safe to call even if already declared by siphon-sink)
	_, err = ch.QueueDeclare(
		"siphon-analysis-queue",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitClient{conn: conn, Channel: ch}, nil
}

// Close releases the RabbitMQ channel and connection.
func (r *RabbitClient) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.conn != nil {
		r.conn.Close()
	}
}
