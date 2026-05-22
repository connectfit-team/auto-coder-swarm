package bus

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

type MessageBus struct {
	nc  *nats.Conn
	js  jetstream.JetStream
}

func NewMessageBus(url string) (*MessageBus, error) {
	nc, err := nats.Connect(url, nats.Name("Swarm Engine"), nats.Timeout(10*time.Second))
	if err != nil {
		return nil, err
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, err
	}

	// Create a stream if it doesn't exist (Enterprise durability)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "SWARM_EVENTS",
		Subjects: []string{"swarm.>"},
		Storage:  jetstream.FileStorage, // Disk-based persistence
	})
	if err != nil {
		log.Printf("⚠️ [Bus] Stream creation warning (may already exist): %v", err)
	}

	return &MessageBus{nc: nc, js: js}, nil
}

func (b *MessageBus) Publish(ctx context.Context, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	log.Printf("📢 [Bus] Publishing to %s (Durable)", subject)
	_, err = b.js.Publish(ctx, subject, data)
	return err
}

func (b *MessageBus) Subscribe(ctx context.Context, subject, consumerName string, handler func([]byte)) error {
	log.Printf("📥 [Bus] Subscribed to %s with consumer %s", subject, consumerName)
	
	// Durable consumer ensures no messages are missed during downtime
	cons, err := b.js.CreateOrUpdateConsumer(ctx, "SWARM_EVENTS", jetstream.ConsumerConfig{
		Durable:   consumerName,
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		return err
	}

	iter, err := cons.Messages()
	if err != nil {
		return err
	}

	go func() {
		for {
			msg, err := iter.Next()
			if err != nil {
				log.Printf("⚠️ [Bus] Subscription error: %v", err)
				return
			}
			handler(msg.Data())
			msg.Ack() // Confirm processing
		}
	}()

	return nil
}

func (b *MessageBus) Close() {
	if b.nc != nil {
		b.nc.Close()
	}
}
