package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rabbitmq/amqp091-go"
)

type Event struct {
	SourceService string          `json:"sourceService"`
	EventType     string          `json:"eventType"`
	Payload       json.RawMessage `json:"payload"`
}

func main() {
	rabbitURL := getenv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	databaseURL := getenv("DATABASE_URL", "postgres://postgres:postgres@postgres:5432/audit_db?sslmode=disable")
	queueName := getenv("AUDIT_QUEUE", "audit.events")

	pool, err := connectDB(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("failed to connect database: %v", err)
	}
	defer pool.Close()

	conn, err := amqp091.Dial(rabbitURL)
	if err != nil {
		log.Fatalf("failed to connect rabbitmq: %v", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq channel: %v", err)
	}
	defer channel.Close()

	if err := channel.ExchangeDeclare("team.activity", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("failed to declare team.activity exchange: %v", err)
	}
	if err := channel.ExchangeDeclare("asset.changes", "topic", true, false, false, false, nil); err != nil {
		log.Fatalf("failed to declare asset.changes exchange: %v", err)
	}

	queue, err := channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		log.Fatalf("failed to declare queue: %v", err)
	}

	bindings := []struct {
		exchange string
		key      string
	}{
		{exchange: "team.activity", key: "#"},
		{exchange: "asset.changes", key: "#"},
	}
	for _, binding := range bindings {
		if err := channel.QueueBind(queue.Name, binding.key, binding.exchange, false, nil); err != nil {
			log.Fatalf("failed to bind queue to %s: %v", binding.exchange, err)
		}
	}

	messages, err := channel.Consume(queue.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalf("failed to start consumer: %v", err)
	}

	log.Printf("audit consumer listening on rabbitmq queue %s", queue.Name)
	for message := range messages {
		var event Event
		if err := json.Unmarshal(message.Body, &event); err != nil {
			log.Printf("invalid event payload: %v", err)
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, execErr := pool.Exec(ctx, `
			INSERT INTO audit_log (source_service, event_type, routing_key, payload)
			VALUES ($1, $2, $3, $4)
		`, event.SourceService, event.EventType, message.RoutingKey, event.Payload)
		cancel()
		if execErr != nil {
			log.Printf("failed to write audit log: %v", execErr)
			continue
		}

		log.Printf("audited event %s from %s", event.EventType, event.SourceService)
	}
}

func connectDB(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, config)
}

func getenv(key, fallback string) string {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	return v
}
