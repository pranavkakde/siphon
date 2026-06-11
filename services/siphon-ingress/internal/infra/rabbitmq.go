package infra

import (
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// RabbitClient encapsulates connections to RabbitMQ server.
type RabbitClient struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
}

// NewRabbitClient connects to RabbitMQ and prepares the default queue exchange.
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
		log.Printf("Connecting to RabbitMQ (attempt %d/10): %v", i+1, err)
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

	// Declare DLX Exchange
	err = ch.ExchangeDeclare(
		"siphon-dlx-exchange", // name
		"direct",              // type
		true,                  // durable
		false,                 // auto-deleted
		false,                 // internal
		false,                 // no-wait
		nil,                   // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Declare DLQ (Dead Letter Queue)
	_, err = ch.QueueDeclare(
		"siphon-dlq", // name
		true,         // durable
		false,        // delete when unused
		false,        // exclusive
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Bind DLQ to DLX exchange
	err = ch.QueueBind(
		"siphon-dlq",
		"siphon-routing-dlkey",
		"siphon-dlx-exchange",
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	// Declare primary buffer queue with x-dead-letter-exchange routing attributes
	args := amqp.Table{
		"x-dead-letter-exchange":    "siphon-dlx-exchange",
		"x-dead-letter-routing-key": "siphon-routing-dlkey",
	}

	_, err = ch.QueueDeclare(
		"siphon-buffer-queue", // name
		true,                  // durable
		false,                 // delete when unused
		false,                 // exclusive
		false,                 // no-wait
		args,                  // arguments binding DLX
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitClient{
		Conn:    conn,
		Channel: ch,
	}, nil
}

// Close releases RabbitMQ socket connections.
func (r *RabbitClient) Close() {
	if r.Channel != nil {
		r.Channel.Close()
	}
	if r.Conn != nil {
		r.Conn.Close()
	}
}
