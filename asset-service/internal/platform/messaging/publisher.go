package messaging

import (
	"context"
	"encoding/json"

	"github.com/rabbitmq/amqp091-go"
)

type Event struct {
	SourceService string `json:"sourceService"`
	EventType     string `json:"eventType"`
	Payload       any    `json:"payload"`
}

type Publisher struct {
	conn     *amqp091.Connection
	exchange string
}

func NewPublisher(url, exchange string) (*Publisher, error) {
	conn, err := amqp091.Dial(url)
	if err != nil {
		return nil, err
	}
	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := channel.ExchangeDeclare(exchange, "topic", true, false, false, false, nil); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}
	_ = channel.Close()
	return &Publisher{conn: conn, exchange: exchange}, nil
}

func (p *Publisher) Close() error {
	if p.conn != nil {
		return p.conn.Close()
	}
	return nil
}

func (p *Publisher) Publish(ctx context.Context, routingKey string, event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return err
	}
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()
	return channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp091.Publishing{ContentType: "application/json", Body: body})
}
