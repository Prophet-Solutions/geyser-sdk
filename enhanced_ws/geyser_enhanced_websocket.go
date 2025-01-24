package geyser_enhanced_ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Client represents a Geyser WebSocket client with reconnect logic.
type Client struct {
	wsConn        *websocket.Conn
	endpoint      string
	mu            sync.Mutex
	subscriptions map[string]*Subscription

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// ErrCh is an error channel where read/write errors are sent.
	// You can consume these to log or handle connection issues.
	ErrCh chan error
}

// Subscription represents a subscription to the Geyser WebSocket.
type Subscription struct {
	ID       string
	Method   string
	Params   interface{}
	UpdateCh chan interface{}
}

// NewClient creates a new Geyser WebSocket client.
func NewClient(endpoint string) *Client {
	return &Client{
		endpoint:      endpoint,
		subscriptions: make(map[string]*Subscription),
		ErrCh:         make(chan error, 10), // buffered
	}
}

// Connect establishes the WebSocket connection and starts the listen loop.
func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Dial the WebSocket
	if err := c.connectInternal(); err != nil {
		return err
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())

	// Start listening for messages
	c.wg.Add(1)
	go c.listen()

	return nil
}

// connectInternal dials the websocket endpoint and updates c.wsConn.
// It does NOT spawn the listen goroutine or change the client context.
func (c *Client) connectInternal() error {
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return fmt.Errorf("invalid endpoint URL: %w", err)
	}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		var status int
		if resp != nil {
			status = resp.StatusCode
		}
		return fmt.Errorf("failed to connect to WebSocket (status code: %d): %w", status, err)
	}

	c.wsConn = conn
	return nil
}

// Close closes the WebSocket connection and cancels all subscriptions.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Cancel context so listen() can exit
	if c.cancel != nil {
		c.cancel()
	}

	var err error
	if c.wsConn != nil {
		err = c.wsConn.Close()
	}

	// Wait for listen() to return
	c.wg.Wait()

	// Close error channel
	close(c.ErrCh)

	return err
}

// Reconnect closes any existing connection, dials again, and re-subscribes.
func (c *Client) Reconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Close old connection if present
	if c.wsConn != nil {
		_ = c.wsConn.Close()
	}

	// Dial again
	if err := c.connectInternal(); err != nil {
		return fmt.Errorf("reconnect: failed to connect: %w", err)
	}

	// Re-subscribe
	for id, sub := range c.subscriptions {
		switch sub.Method {
		case "transactionSubscribe":
			filter, ok := sub.Params.(TransactionSubscribeFilter)
			if !ok {
				return fmt.Errorf("reconnect: invalid params for tx subscription %s", id)
			}
			// For simplicity, we assume no custom options here.
			// If you stored them, re-use them as well.
			if _, err := c.TransactionSubscribe(id, filter, nil); err != nil {
				return fmt.Errorf("reconnect: failed to re-subscribe (tx) %s: %w", id, err)
			}

		case "accountSubscribe":
			account, ok := sub.Params.(string)
			if !ok {
				return fmt.Errorf("reconnect: invalid params for account subscription %s", id)
			}
			if _, err := c.AccountSubscribe(id, account, nil); err != nil {
				return fmt.Errorf("reconnect: failed to re-subscribe (account) %s: %w", id, err)
			}

			// Handle other subscription methods if needed.
		}
	}

	return nil
}

// TransactionSubscribe sends a transaction subscription request and returns a channel to receive messages.
func (c *Client) TransactionSubscribe(subscriptionID string, filter TransactionSubscribeFilter, options *TransactionSubscribeOptions) (chan interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.subscriptions[subscriptionID]; exists {
		return nil, fmt.Errorf("subscription with ID %s already exists", subscriptionID)
	}

	sub := &Subscription{
		ID:       subscriptionID,
		Method:   "transactionSubscribe",
		Params:   filter,
		UpdateCh: make(chan interface{}, 100), // buffered
	}

	// Build params
	params := []interface{}{filter}
	if options != nil {
		params = append(params, options)
	}

	// JSON RPC request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      subscriptionID,
		"method":  "transactionSubscribe",
		"params":  params,
	}

	if err := c.wsConn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send transaction subscription request: %w", err)
	}

	c.subscriptions[subscriptionID] = sub
	return sub.UpdateCh, nil
}

// AccountSubscribe sends an account subscription request and returns a channel to receive messages.
func (c *Client) AccountSubscribe(subscriptionID string, account string, options *AccountSubscribeOptions) (chan interface{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.subscriptions[subscriptionID]; exists {
		return nil, fmt.Errorf("subscription with ID %s already exists", subscriptionID)
	}

	sub := &Subscription{
		ID:       subscriptionID,
		Method:   "accountSubscribe",
		Params:   account,
		UpdateCh: make(chan interface{}, 100), // buffered
	}

	// Build params
	params := []interface{}{account}
	if options != nil {
		params = append(params, options)
	}

	// JSON RPC request
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      subscriptionID,
		"method":  "accountSubscribe",
		"params":  params,
	}

	if err := c.wsConn.WriteJSON(req); err != nil {
		return nil, fmt.Errorf("failed to send account subscription request: %w", err)
	}

	c.subscriptions[subscriptionID] = sub
	return sub.UpdateCh, nil
}

// listen listens for incoming messages in a loop.
// If the connection breaks, it attempts to reconnect automatically (example).
func (c *Client) listen() {
	defer c.wg.Done()

	for {
		_, message, err := c.wsConn.ReadMessage()
		if err != nil {
			select {
			case <-c.ctx.Done():
				// Context canceled -> we are shutting down
				return
			default:
				// Connection broke. Send error & try reconnect.
				c.ErrCh <- fmt.Errorf("read error: %w, reconnecting...", err)

				// Attempt a reconnect with a simple retry/backoff loop
				backoff := time.Second
				for i := 0; i < 5; i++ { // up to 5 tries
					if rerr := c.Reconnect(); rerr == nil {
						c.ErrCh <- fmt.Errorf("reconnected successfully after error: %v", err)
						break
					}
					time.Sleep(backoff)
					backoff *= 2
				}
				// After we (possibly) reconnect, continue the loop.
				// If reconnect fully fails after 5 tries, we're still
				// using the old connection object, so subsequent ReadMessage()
				// will likely fail again. Adjust logic as needed.
				continue
			}
		}

		// If we read successfully, handle the message
		c.handleMessage(message)
	}
}

// handleMessage handles incoming messages and routes them to the appropriate subscription channel.
func (c *Client) handleMessage(message []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(message, &msg); err != nil {
		c.ErrCh <- fmt.Errorf("failed to unmarshal message: %w", err)
		return
	}

	// Check if it's an error response
	if msgErr, ok := msg["error"]; ok {
		c.ErrCh <- fmt.Errorf("received error message: %v", msgErr)
		return
	}

	method, _ := msg["method"].(string)
	switch method {

	case "transactionNotification":
		// Example for transaction notifications
		var notification TransactionNotification
		if err := json.Unmarshal(message, &notification); err != nil {
			c.ErrCh <- fmt.Errorf("failed to unmarshal transaction notification: %w", err)
			return
		}
		// Broadcast to all transaction subscribers
		for _, sub := range c.subscriptions {
			if sub.Method == "transactionSubscribe" {
				sub.UpdateCh <- notification
			}
		}

	case "accountNotification":
		// Example for account notifications
		var notification AccountNotification
		if err := json.Unmarshal(message, &notification); err != nil {
			c.ErrCh <- fmt.Errorf("failed to unmarshal account notification: %w", err)
			return
		}
		// Broadcast to all account subscribers
		for _, sub := range c.subscriptions {
			if sub.Method == "accountSubscribe" {
				sub.UpdateCh <- notification
			}
		}

	default:
		// If there's no recognized 'method', treat it as unknown or handle differently.
		c.ErrCh <- fmt.Errorf("unknown message format or method: %s", method)
	}
}
