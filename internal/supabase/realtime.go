package supabase

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"sycronizafhir/internal/models"
)

type RealtimeClient struct {
	realtimeURL string
	apiKey      string
	channel     string
	schema      string
	table       string
}

type realtimeEnvelope struct {
	Event   string `json:"event"`
	Topic   string `json:"topic"`
	Payload struct {
		EventType string          `json:"eventType"`
		New       json.RawMessage `json:"new"`
	} `json:"payload"`
	Ref string `json:"ref"`
}

func NewRealtimeClient(realtimeURL, apiKey, channel, schema, table string) *RealtimeClient {
	return &RealtimeClient{
		realtimeURL: realtimeURL,
		apiKey:      apiKey,
		channel:     channel,
		schema:      schema,
		table:       table,
	}
}

func (c *RealtimeClient) ListenPedidos(ctx context.Context, onPedido func(models.Pedido) error) error {
	rawURL := strings.TrimRight(c.realtimeURL, "/")
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse realtime url: %w", err)
	}

	header := http.Header{}
	header.Set("apikey", c.apiKey)
	header.Set("Authorization", "Bearer "+c.apiKey)

	conn, _, err := websocket.DefaultDialer.Dial(parsed.String(), header)
	if err != nil {
		return fmt.Errorf("dial realtime websocket: %w", err)
	}
	defer conn.Close()

	if err = c.joinChannel(conn); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			if err = conn.SetReadDeadline(time.Now().Add(45 * time.Second)); err != nil {
				return fmt.Errorf("set read deadline: %w", err)
			}

			_, message, readErr := conn.ReadMessage()
			if readErr != nil {
				return fmt.Errorf("read realtime message: %w", readErr)
			}

			var envelope realtimeEnvelope
			if unmarshalErr := json.Unmarshal(message, &envelope); unmarshalErr != nil {
				continue
			}

			if envelope.Event != "postgres_changes" || envelope.Payload.EventType != "INSERT" || len(envelope.Payload.New) == 0 {
				continue
			}

			var pedido models.Pedido
			if decodeErr := json.Unmarshal(envelope.Payload.New, &pedido); decodeErr != nil {
				continue
			}

			if err = onPedido(pedido); err != nil {
				return err
			}
		}
	}
}

func (c *RealtimeClient) joinChannel(conn *websocket.Conn) error {
	joinPayload := map[string]interface{}{
		"topic":   c.channel,
		"event":   "phx_join",
		"payload": map[string]interface{}{},
		"ref":     "1",
	}

	joinPayload["payload"] = map[string]interface{}{
		"config": map[string]interface{}{
			"postgres_changes": []map[string]string{
				{
					"event":  "INSERT",
					"schema": c.schema,
					"table":  c.table,
				},
			},
		},
	}

	if err := conn.WriteJSON(joinPayload); err != nil {
		return fmt.Errorf("join realtime channel: %w", err)
	}

	return nil
}
