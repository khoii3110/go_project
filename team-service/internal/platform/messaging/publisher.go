package messaging

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rabbitmq/amqp091-go"
)

type Event struct {
	SourceService string      `json:"sourceService"`
	EventType     string      `json:"eventType"`
	Payload       any         `json:"payload"`
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

	if err := channel.ExchangeDeclare(p.exchange, "topic", true, false, false, false, nil); err != nil {
		return err
	}

	return channel.PublishWithContext(ctx, p.exchange, routingKey, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (p *Publisher) EnsureQueue(queueName, bindingKey string) error {
	channel, err := p.conn.Channel()
	if err != nil {
		return err
	}
	defer channel.Close()

	queue, err := channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		return err
	}
	if err := channel.QueueBind(queue.Name, bindingKey, p.exchange, false, nil); err != nil {
		return err
	}
	return nil
}

func (p *Publisher) Consume(queueName string) (<-chan amqp091.Delivery, error) {
	channel, err := p.conn.Channel()
	if err != nil {
		return nil, err
	}
	queue, err := channel.QueueDeclare(queueName, true, false, false, false, nil)
	if err != nil {
		_ = channel.Close()
		return nil, err
	}
	msgs, err := channel.Consume(queue.Name, "", true, false, false, false, nil)
	if err != nil {
		_ = channel.Close()
		return nil, fmt.Errorf("consume queue %s: %w", queue.Name, err)
	}
	return msgs, nil
}
