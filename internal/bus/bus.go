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
	if err != nil { return nil, err }

	js, err := jetstream.New(nc)
	if err != nil { return nil, err }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err = js.CreateStream(ctx, jetstream.StreamConfig{
		Name:     "SWARM_EVENTS",
		Subjects: []string{"swarm.>"},
		Storage:  jetstream.FileStorage,
	})
	if err != nil { log.Printf("⚠️ [Bus] Stream creation warning: %v", err) }

	return &MessageBus{nc: nc, js: js}, nil
}

func (b *MessageBus) Publish(ctx context.Context, subject string, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil { return err }
	log.Printf("📢 [Bus] Publishing to %s", subject)
	_, err = b.js.Publish(ctx, subject, data)
	return err
}

// SubscribeOnce uses standard NATS subscription for lightweight signaling
func (b *MessageBus) SubscribeOnce(ctx context.Context, subject string) (chan []byte, error) {
	ch := make(chan []byte, 1)
	sub, err := b.nc.Subscribe(subject, func(m *nats.Msg) {
		ch <- m.Data
	})
	if err != nil { return nil, err }

	go func() {
		select {
		case <-ctx.Done():
		case <-ch: // Signal received
		}
		sub.Unsubscribe()
	}()

	return ch, nil
}

func (b *MessageBus) Close() {
	if b.nc != nil { b.nc.Close() }
}
