package finnhub

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"FinPull/internal/domain/models"
	drepo "FinPull/internal/domain/repository"

	"github.com/gorilla/websocket"
)

// Client implements a MarketStream backed by Finnhub WebSocket.
type Client struct {
	apiKey         string
	websocketURL   string
	symbols        []string
	reconnectDelay time.Duration
	pingInterval   time.Duration

	conn      *websocket.Conn
	connected bool
}

// New creates a new Finnhub MarketStream.
func New(apiKey, websocketURL string, symbols []string, reconnectDelay, pingInterval time.Duration) drepo.MarketStream {
	return &Client{
		apiKey:         apiKey,
		websocketURL:   websocketURL,
		symbols:        symbols,
		reconnectDelay: reconnectDelay,
		pingInterval:   pingInterval,
	}
}

// Connect establishes the WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	u := fmt.Sprintf("%s?token=%s", c.websocketURL, c.apiKey)
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u, nil)
	if err != nil {
		return fmt.Errorf("finnhub connect: %w", err)
	}
	c.conn = conn
	c.connected = true
	log.Printf("finnhub: connected")
	return nil
}

// Subscribe subscribes to configured symbols.
func (c *Client) Subscribe(ctx context.Context) error {
	if c.conn == nil || !c.connected {
		return fmt.Errorf("finnhub not connected")
	}
	for _, s := range c.symbols {
		msg := map[string]string{"type": "subscribe", "symbol": s}
		if err := c.conn.WriteJSON(msg); err != nil {
			return fmt.Errorf("subscribe %s: %w", s, err)
		}
		log.Printf("finnhub: subscribed %s", s)
	}
	return nil
}

type fhTrade struct {
	S string  `json:"s"`
	P float64 `json:"p"`
	V float64 `json:"v"`
	T int64   `json:"t"` // ms
}

type fhMessage struct {
	Type string    `json:"type"`
	Data []fhTrade `json:"data"`
}

// Read streams Trade events and errors.
func (c *Client) Read(ctx context.Context) (<-chan *models.Trade, <-chan error) {
	trades := make(chan *models.Trade, 1024)
	errs := make(chan error, 1)

	// ping loop
	go func() {
		ticker := time.NewTicker(c.pingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if c.conn != nil {
					_ = c.conn.WriteMessage(websocket.PingMessage, nil)
				}
			}
		}
	}()

	// read loop
	go func() {
		defer close(trades)
		defer close(errs)
		for {
			select {
			case <-ctx.Done():
				return
			default:
				if c.conn == nil {
					errs <- fmt.Errorf("finnhub conn nil")
					return
				}
				_, b, err := c.conn.ReadMessage()
				if err != nil {
					errs <- fmt.Errorf("finnhub read: %w", err)
					return
				}
				var m fhMessage
				if err := json.Unmarshal(b, &m); err != nil {
					// ignore non-trade frames
					continue
				}
				if m.Type != "trade" {
					continue
				}
				for _, d := range m.Data {
					sec := d.T / 1000
					trade := &models.Trade{Symbol: d.S, Timestamp: sec, Price: d.P, Volume: d.V}
					select {
					case trades <- trade:
					default:
						// drop on backpressure
					}
				}
			}
		}
	}()

	return trades, errs
}

// Reconnect closes and reconnects.
func (c *Client) Reconnect(ctx context.Context) error {
	_ = c.Close()
	time.Sleep(c.reconnectDelay)
	if err := c.Connect(ctx); err != nil {
		return err
	}
	return c.Subscribe(ctx)
}

// Close closes the WS connection.
func (c *Client) Close() error {
	c.connected = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// IsConnected indicates status.
func (c *Client) IsConnected() bool { return c.connected }
