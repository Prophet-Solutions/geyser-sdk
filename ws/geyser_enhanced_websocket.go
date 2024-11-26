package geyser_ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/gorilla/websocket"
)

// Client represents a Geyser WebSocket client.
type client struct {
	wsConn        *websocket.Conn
	endpoint      string
	mu            sync.Mutex
	subscriptions map[string]*subscription
	ctx           context.Context
	cancel        context.CancelFunc
	wg            sync.WaitGroup
	ErrCh         chan error
}

// Subscription represents a subscription to the Geyser WebSocket.
type subscription struct {
	ID       string
	Method   string
	Params   interface{}
	UpdateCh chan interface{}
}

// NewClient creates a new Geyser WebSocket client.
func NewClient(endpoint string) *client {
	return &client{
		endpoint:      endpoint,
		subscriptions: make(map[string]*subscription),
		ErrCh:         make(chan error),
	}
}

// Connect establishes the WebSocket connection.
func (c *client) Connect() error {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket (status code: %d): %w", resp.StatusCode, err)
	}

	c.wsConn = conn
	c.ctx, c.cancel = context.WithCancel(context.Background())

	// Start listening to messages
	c.wg.Add(1)
	go c.listen()

	return nil
}

// Close closes the WebSocket connection and cancels all subscriptions.
func (c *client) Close() error {
	c.cancel()
	err := c.wsConn.Close()
	c.wg.Wait()
	close(c.ErrCh)
	return err
}

// TransactionSubscribe sends a transaction subscription request and returns a channel to receive messages.
func (c *client) TransactionSubscribe(subscriptionID string, filter TransactionSubscribeFilter, options *TransactionSubscribeOptions) (chan interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.subscriptions[subscriptionID]; exists {
		return nil, fmt.Errorf("subscription with ID %s already exists", subscriptionID)
	}

	sub := &subscription{
		ID:       subscriptionID,
		Method:   "transactionSubscribe",
		UpdateCh: make(chan interface{}, 100), // Buffered channel
	}

	params := []interface{}{filter}
	if options != nil {
		params = append(params, options)
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      subscriptionID,
		"method":  "transactionSubscribe",
		"params":  params,
	}

	err := c.wsConn.WriteJSON(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction subscription request: %w", err)
	}

	sub.Params = filter
	c.subscriptions[subscriptionID] = sub
	return sub.UpdateCh, nil
}

// AccountSubscribe sends an account subscription request and returns a channel to receive messages.
func (c *client) AccountSubscribe(subscriptionID string, account string, options *AccountSubscribeOptions) (chan interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.subscriptions[subscriptionID]; exists {
		return nil, fmt.Errorf("subscription with ID %s already exists", subscriptionID)
	}

	sub := &subscription{
		ID:       subscriptionID,
		Method:   "accountSubscribe",
		UpdateCh: make(chan interface{}, 100), // Buffered channel
	}

	params := []interface{}{account}
	if options != nil {
		params = append(params, options)
	}

	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      subscriptionID,
		"method":  "accountSubscribe",
		"params":  params,
	}

	err := c.wsConn.WriteJSON(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send account subscription request: %w", err)
	}

	sub.Params = account
	c.subscriptions[subscriptionID] = sub
	return sub.UpdateCh, nil
}

// listen listens for incoming messages.
func (c *client) listen() {
	defer c.wg.Done()
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
			_, message, err := c.wsConn.ReadMessage()
			if err != nil {
				c.ErrCh <- fmt.Errorf("read error: %w", err)
				return
			}

			// Handle response messages
			c.handleMessage(message)
		}
	}
}

// handleMessage handles incoming messages and routes them to the appropriate subscription channel.
func (c *client) handleMessage(message []byte) {
	var msg map[string]interface{}
	err := json.Unmarshal(message, &msg)
	if err != nil {
		c.ErrCh <- fmt.Errorf("failed to unmarshal message: %w", err)
		return
	}

	// Handle notifications
	if method, ok := msg["method"].(string); ok && (method == "transactionNotification" || method == "accountNotification") {
		// Parse the result based on the method
		var result interface{}
		if method == "accountNotification" {
			var accountNotification AccountNotification
			if err := json.Unmarshal(message, &accountNotification); err != nil {
				c.ErrCh <- fmt.Errorf("failed to unmarshal account notification: %w", err)
				return
			}
			result = accountNotification
		} else if method == "transactionNotification" {
			var transactionNotification TransactionNotification
			if err := json.Unmarshal(message, &transactionNotification); err != nil {
				c.ErrCh <- fmt.Errorf("failed to unmarshal transaction notification: %w", err)
				return
			}
			result = transactionNotification
		}

		for _, sub := range c.subscriptions {
			if sub.Method == "transactionSubscribe" && method == "transactionNotification" {
				sub.UpdateCh <- result
			} else if sub.Method == "accountSubscribe" && method == "accountNotification" {
				sub.UpdateCh <- result
			}
		}
	} else if msgErr, ok := msg["error"]; ok {
		c.ErrCh <- fmt.Errorf("received error message: %v", msgErr)
	} else {
		c.ErrCh <- fmt.Errorf("unknown message format")
	}
}
