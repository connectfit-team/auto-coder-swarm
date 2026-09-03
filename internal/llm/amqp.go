package llm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"iter"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/adk/model"
	"google.golang.org/genai"
)

type RabbitMQModel struct {
	name     string
	amqpURL  string
	exchange string
	routing  string
	conn     *amqp.Connection
	mu       sync.Mutex
}

func NewRabbitMQModel(name, url string) model.LLM {
	client := &RabbitMQModel{
		name:     name,
		amqpURL:  url,
		exchange: "llm_exchange",
		routing:  "task.text",
	}
	client.getConnection()
	return client
}

func (m *RabbitMQModel) Name() string { return m.name }

func (m *RabbitMQModel) getConnection() (*amqp.Connection, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn == nil || m.conn.IsClosed() {
		log.Println("🔄 [RabbitMQModel] Connecting to RabbitMQ...")
		var err error
		m.conn, err = amqp.Dial(m.amqpURL)
		if err != nil {
			return nil, fmt.Errorf("failed to connect: %v", err)
		}
	}
	return m.conn, nil
}

func (m *RabbitMQModel) GenerateContent(ctx context.Context, req *model.LLMRequest, stream bool) iter.Seq2[*model.LLMResponse, error] {
	return func(yield func(*model.LLMResponse, error) bool) {
		promptText := BuildPrompt(req)

		conn, err := m.getConnection()
		if err != nil {
			yield(nil, fmt.Errorf("RabbitMQ connection unavailable: %v", err))
			return
		}

		ch, err := conn.Channel()
		if err != nil {
			yield(nil, fmt.Errorf("failed to open channel: %v", err))
			return
		}
		defer ch.Close()

		replyQueue, err := ch.QueueDeclare("", false, true, true, false, nil)
		if err != nil {
			yield(nil, fmt.Errorf("failed to declare reply queue: %v", err))
			return
		}

		msgs, err := ch.Consume(replyQueue.Name, "", true, false, false, false, nil)
		if err != nil {
			yield(nil, fmt.Errorf("failed to consume: %v", err))
			return
		}

		b := make([]byte, 16)
		rand.Read(b)
		corrID := hex.EncodeToString(b)

		payload := map[string]interface{}{
			"prompt": promptText,
		}
		body, _ := json.Marshal(payload)

		err = ch.PublishWithContext(ctx, m.exchange, m.routing, false, false, amqp.Publishing{
			ContentType:   "application/json",
			CorrelationId: corrID,
			ReplyTo:       replyQueue.Name,
			Priority:      5,
			Body:          body,
		})
		if err != nil {
			yield(nil, fmt.Errorf("publish error: %v", err))
			return
		}

		var rawContent string
	WaitLoop:
		for {
			select {
			case d := <-msgs:
				if d.CorrelationId == corrID {
					var parsed map[string]interface{}
					if err := json.Unmarshal(d.Body, &parsed); err == nil {
						if respText, ok := parsed["response"].(string); ok {
							rawContent = respText
							break WaitLoop
						}
					}
					rawContent = string(d.Body)
					break WaitLoop
				}
			case <-ctx.Done():
				yield(nil, ctx.Err())
				return
			case <-time.After(5 * time.Minute):
				yield(nil, fmt.Errorf("timeout waiting for LLM response from RabbitMQ"))
				return
			}
		}

		// Yield the full string once as stream response (ADK compatible)
		if rawContent != "" {
			if !yield(&model.LLMResponse{Content: &genai.Content{Role: "model", Parts: []*genai.Part{{Text: rawContent}}}}, nil) {
				return
			}
		}

		// Parse tool calls
		re := regexp.MustCompile("(?s)<\\|tool_call\\>call:([^\\{]+)(\\{.*?\\})<\\|tool_call\\>")
		matches := re.FindAllStringSubmatch(rawContent, -1)
		if len(matches) > 0 {
			res := &model.LLMResponse{Content: &genai.Content{Role: "model"}, FinishReason: "FINISH_REASON_FUNCTION_CALL"}
			for _, match := range matches {
				name := strings.TrimSpace(match[1])
				argsStr := strings.ReplaceAll(match[2], "<|\"|>", "\"")
				var args map[string]any
				if err := json.Unmarshal([]byte(argsStr), &args); err == nil {
					res.Content.Parts = append(res.Content.Parts, &genai.Part{FunctionCall: &genai.FunctionCall{Name: name, Args: args}})
				}
			}
			yield(res, nil)
		}
	}
}
