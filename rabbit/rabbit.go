package rabbit_handler

import (
	"context"
	"fmt"

	"github.com/geffersonFerraz/frigate-events/config"
	amqp "github.com/rabbitmq/amqp091-go"
)

func Run() {
	fmt.Println("RabbitMQ handler started")
}

type Rabbit interface {
	Close() error
	DeclareQueue(name string, durable bool) error
	DeclareExchange(name string, durable bool) error
	DeclareBinding(queue, exchange, routingKey string) error
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
	Consume(queue string, consumer string) (<-chan amqp.Delivery, error)
}

type rabbit struct {
	conn *amqp.Connection
	ch   *amqp.Channel
}

func NewRabbit(cfg *config.Config) (Rabbit, error) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, fmt.Errorf("error dialing RabbitMQ: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("error creating channel: %w", err)
	}
	return &rabbit{
		conn: conn,
		ch:   ch,
	}, nil
}

func (r *rabbit) Close() error {
	return r.conn.Close()
}

func (r *rabbit) DeclareQueue(name string, durable bool) error {
	_, err := r.ch.QueueDeclare(name, durable, false, false, false, nil)
	return err
}

func (r *rabbit) DeclareExchange(name string, durable bool) error {
	return r.ch.ExchangeDeclare(name, "direct", durable, false, false, false, nil)
}

func (r *rabbit) DeclareBinding(queue, exchange, routingKey string) error {
	return r.ch.QueueBind(queue, routingKey, exchange, false, nil)
}

func (r *rabbit) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return r.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func (r *rabbit) Consume(queue string, consumer string) (<-chan amqp.Delivery, error) {
	return r.ch.Consume(queue, consumer, false, false, false, false, nil)
}
