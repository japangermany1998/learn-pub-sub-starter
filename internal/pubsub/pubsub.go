package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange, key string, val T) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return ch.PublishWithContext(context.Background(), exchange, key, false, false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		})
}

func SubscribeJSON[T any](
	conn *amqp.Connection,
	exchange, queueName, key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
	handler func(T) string) error {
	ch, _, err := DeclareAndBind(conn, exchange, queueName, key, simpleQueueType)
	if err != nil {
		return err
	}
	deliver, err := ch.Consume(queueName, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		defer ch.Close()
		for data := range deliver {
			var val T
			err = json.Unmarshal(data.Body, &val)
			if err != nil {
				fmt.Printf("could not unmarshal message: %v\n", err)
				continue
			}
			ackType := handler(val)
			switch ackType {
			case "Ack":
				fmt.Println("Ack occured")
				err = data.Ack(false)

				break
			case "NackRequeue":
				fmt.Println("NAckRequeue occured")
				err = data.Nack(false, true)
				break
			case "NackDiscard":
				fmt.Println("NAckDiscard occured")
				err = data.Nack(false, false)
				break
			}
		}
	}()

	return nil
}

func DeclareAndBind(
	conn *amqp.Connection,
	exchange,
	queueName,
	key string,
	simpleQueueType int, // an enum to represent "durable" or "transient"
) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		return nil, amqp.Queue{}, err
	}

	queue, err := ch.QueueDeclare(queueName, simpleQueueType == 0, simpleQueueType != 0, simpleQueueType != 0, false, amqp.Table{
		"x-dead-letter-exchange": "peril_dlx",
	})
	if err != nil {
		return nil, amqp.Queue{}, err
	}
	if err = ch.QueueBind(queueName, key, exchange, false, nil); err != nil {
		return nil, amqp.Queue{}, err
	}
	return ch, queue, nil
}
