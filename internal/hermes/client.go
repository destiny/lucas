// Copyright 2025 Arion Yau
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// PendingClientRequest represents a pending client request
type PendingClientRequest struct {
	MessageID string
	Service   string
	Body      []byte
	Response  chan []byte
	Error     chan error
	Timestamp time.Time
	Timeout   time.Duration
	Nonce     string    // Add nonce for correlation
	FireAndForget bool  // If true, don't wait for response
}

// ClientStats represents client statistics
type ClientStats struct {
	RequestsSent     int       `json:"requests_sent"`
	ResponsesReceived int       `json:"responses_received"`
	RequestsFailed   int       `json:"requests_failed"`
	RequestsTimeout  int       `json:"requests_timeout"`
	LastRequest      time.Time `json:"last_request"`
	LastResponse     time.Time `json:"last_response"`
	StartTime        time.Time `json:"start_time"`
	AverageLatency   float64   `json:"average_latency_ms"`
}

// HermesClient implements the Hermes Majordomo Protocol client
type HermesClient struct {
	broker       string
	identity     string
	socket       *zmq4.Socket
	timeout      time.Duration
	retries      int
	pending      map[string]*PendingClientRequest  // Keyed by message ID
	pendingNonces map[string]*PendingClientRequest // Keyed by nonce for optional correlation
	ctx          context.Context
	cancel       context.CancelFunc
	logger       zerolog.Logger
	stats        *ClientStats
	mutex        sync.RWMutex
	latencies    []time.Duration
}

// NewClient creates a new Hermes client
func NewClient(broker, identity string) *HermesClient {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HermesClient{
		broker:        broker,
		identity:      identity,
		timeout:       30 * time.Second, // Default request timeout
		retries:       3,                // Default retry count
		pending:       make(map[string]*PendingClientRequest),
		pendingNonces: make(map[string]*PendingClientRequest),
		ctx:           ctx,
		cancel:        cancel,
		logger:        logger.New(),
		stats: &ClientStats{
			StartTime: time.Now(),
		},
		latencies: make([]time.Duration, 0, 100), // Keep last 100 latencies
	}
}

// SetTimeout sets the request timeout
func (c *HermesClient) SetTimeout(timeout time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.timeout = timeout
}

// SetRetries sets the number of retries for failed requests
func (c *HermesClient) SetRetries(retries int) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.retries = retries
}

// Start starts the client
func (c *HermesClient) Start() error {
	c.logger.Info().
		Str("broker", c.broker).
		Str("identity", c.identity).
		Msg("Starting Hermes client")

	if err := c.connect(); err != nil {
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	// Start message processing loop
	go c.messageLoop()

	// Start timeout monitor
	go c.timeoutMonitor()

	return nil
}

// Stop stops the client
func (c *HermesClient) Stop() error {
	c.logger.Info().Msg("Stopping Hermes client")

	c.cancel()

	// Cancel all pending requests
	c.mutex.Lock()
	for _, pending := range c.pending {
		select {
		case pending.Error <- fmt.Errorf("client shutting down"):
		default:
		}
		close(pending.Response)
		close(pending.Error)
	}
	c.pending = make(map[string]*PendingClientRequest)
	c.mutex.Unlock()

	if c.socket != nil {
		if err := c.socket.Close(); err != nil {
			c.logger.Error().Err(err).Msg("Error closing client socket")
		}
		c.socket = nil
	}

	c.logger.Info().Msg("Hermes client stopped")
	return nil
}

// connect establishes connection to the broker
func (c *HermesClient) connect() error {
	c.logger.Info().
		Str("broker", c.broker).
		Msg("Connecting to Hermes broker")

	// Create REQ socket for synchronous request-response
	socket, err := zmq4.NewSocket(zmq4.DEALER)
	if err != nil {
		return fmt.Errorf("failed to create DEALER socket: %w", err)
	}

	// Set socket identity
	if err = socket.SetIdentity(c.identity); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set socket identity: %w", err)
	}

	// Set socket options
	if err = socket.SetLinger(1000); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set linger: %w", err)
	}

	if err = socket.SetRcvtimeo(5 * time.Second); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set receive timeout: %w", err)
	}

	if err = socket.SetSndtimeo(30 * time.Second); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set send timeout: %w", err)
	}

	// Connect to broker
	if err = socket.Connect(c.broker); err != nil {
		socket.Close()
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	c.socket = socket
	c.logger.Info().Msg("Connected to Hermes broker")
	return nil
}

// Request sends a synchronous request to a service
func (c *HermesClient) Request(service string, body []byte) ([]byte, error) {
	return c.RequestWithTimeout(service, body, c.timeout)
}

// RequestWithTimeout sends a synchronous request with custom timeout
func (c *HermesClient) RequestWithTimeout(service string, body []byte, timeout time.Duration) ([]byte, error) {
	messageID := GenerateMessageID()
	
	c.logger.Info().
		Str("service", service).
		Str("message_id", messageID).
		Int("body_size", len(body)).
		Dur("timeout", timeout).
		Int("pending_requests", len(c.pending)).
		Msg("Sending request to service")

	// Create pending request
	pending := &PendingClientRequest{
		MessageID: messageID,
		Service:   service,
		Body:      body,
		Response:  make(chan []byte, 1),
		Error:     make(chan error, 1),
		Timestamp: time.Now(),
		Timeout:   timeout,
	}

	// Store pending request
	c.mutex.Lock()
	c.pending[messageID] = pending
	c.mutex.Unlock()

	// Try request with retries
	var lastError error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			c.logger.Warn().
				Str("service", service).
				Str("message_id", messageID).
				Int("attempt", attempt).
				Msg("Retrying request")
		}

		// Send request
		if err := c.sendRequest(service, messageID, body); err != nil {
			lastError = err
			continue
		}

		// Wait for response or timeout
		select {
		case response := <-pending.Response:
			// Calculate and store latency
			latency := time.Since(pending.Timestamp)
			c.recordLatency(latency)
			
			c.logger.Info().
				Str("service", service).
				Str("message_id", messageID).
				Dur("latency", latency).
				Int("response_size", len(response)).
				Int("attempt", attempt).
				Msg("Received successful response")
			
			// Clean up pending request
			c.mutex.Lock()
			delete(c.pending, messageID)
			c.stats.ResponsesReceived++
			c.stats.LastResponse = time.Now()
			c.mutex.Unlock()
			
			return response, nil
		case err := <-pending.Error:
			lastError = err
		case <-time.After(timeout):
			c.logger.Warn().
				Str("service", service).
				Str("message_id", messageID).
				Dur("timeout", timeout).
				Msg("Request timeout")
			lastError = fmt.Errorf("request timeout after %v", timeout)
			c.mutex.Lock()
			c.stats.RequestsTimeout++
			c.mutex.Unlock()
		case <-c.ctx.Done():
			lastError = fmt.Errorf("client shutting down")
		}

		// Exponential backoff for retries
		if attempt < c.retries {
			backoff := time.Duration(attempt+1) * time.Second
			time.Sleep(backoff)
		}
	}

	// Clean up pending request
	c.mutex.Lock()
	delete(c.pending, messageID)
	c.stats.RequestsFailed++
	c.mutex.Unlock()

	close(pending.Response)
	close(pending.Error)

	if lastError == nil {
		lastError = fmt.Errorf("request failed after %d retries", c.retries)
	}

	c.logger.Error().
		Str("service", service).
		Str("message_id", messageID).
		Err(lastError).
		Msg("Request failed")

	return nil, lastError
}

// RequestAsync sends an asynchronous request to a service
func (c *HermesClient) RequestAsync(service string, body []byte, callback func([]byte, error)) error {
	messageID := GenerateMessageID()
	
	c.logger.Debug().
		Str("service", service).
		Str("message_id", messageID).
		Int("body_size", len(body)).
		Msg("Sending async request to service")

	// Create pending request
	pending := &PendingClientRequest{
		MessageID: messageID,
		Service:   service,
		Body:      body,
		Response:  make(chan []byte, 1),
		Error:     make(chan error, 1),
		Timestamp: time.Now(),
		Timeout:   c.timeout,
	}

	// Store pending request
	c.mutex.Lock()
	c.pending[messageID] = pending
	c.mutex.Unlock()

	// Send request
	if err := c.sendRequest(service, messageID, body); err != nil {
		c.mutex.Lock()
		delete(c.pending, messageID)
		c.stats.RequestsFailed++
		c.mutex.Unlock()
		return err
	}

	// Handle response asynchronously
	go func() {
		defer func() {
			c.mutex.Lock()
			delete(c.pending, messageID)
			c.mutex.Unlock()
			close(pending.Response)
			close(pending.Error)
		}()

		select {
		case response := <-pending.Response:
			// Calculate and store latency
			latency := time.Since(pending.Timestamp)
			c.recordLatency(latency)
			
			c.mutex.Lock()
			c.stats.ResponsesReceived++
			c.stats.LastResponse = time.Now()
			c.mutex.Unlock()
			
			callback(response, nil)
		case err := <-pending.Error:
			c.mutex.Lock()
			c.stats.RequestsFailed++
			c.mutex.Unlock()
			callback(nil, err)
		case <-time.After(pending.Timeout):
			c.mutex.Lock()
			c.stats.RequestsTimeout++
			c.mutex.Unlock()
			callback(nil, fmt.Errorf("request timeout after %v", pending.Timeout))
		case <-c.ctx.Done():
			callback(nil, fmt.Errorf("client shutting down"))
		}
	}()

	return nil
}

// RequestFireAndForget sends a request that doesn't wait for a response (fire-and-forget)
// Uses nonce-based correlation for optional response matching
func (c *HermesClient) RequestFireAndForget(service string, body []byte, nonce string) error {
	messageID := GenerateMessageID()
	
	c.logger.Debug().
		Str("service", service).
		Str("message_id", messageID).
		Str("nonce", nonce).
		Int("body_size", len(body)).
		Msg("Sending fire-and-forget request to service")

	// Create pending request for optional response tracking
	pending := &PendingClientRequest{
		MessageID:     messageID,
		Service:       service,
		Body:          body,
		Response:      nil, // No response channel for fire-and-forget
		Error:         nil, // No error channel for fire-and-forget
		Timestamp:     time.Now(),
		Timeout:       5 * time.Second, // Short timeout for cleanup only
		Nonce:         nonce,
		FireAndForget: true,
	}

	// Store pending request for nonce-based response correlation (optional)
	c.mutex.Lock()
	if nonce != "" {
		c.pendingNonces[nonce] = pending
	}
	c.mutex.Unlock()

	// Send request
	if err := c.sendRequest(service, messageID, body); err != nil {
		c.mutex.Lock()
		if nonce != "" {
			delete(c.pendingNonces, nonce)
		}
		c.stats.RequestsFailed++
		c.mutex.Unlock()
		return err
	}

	// Update stats - consider fire-and-forget as successful send
	c.mutex.Lock()
	c.stats.RequestsSent++
	c.stats.LastRequest = time.Now()
	c.mutex.Unlock()

	// Clean up nonce after timeout (to prevent memory leaks)
	if nonce != "" {
		go func() {
			time.Sleep(pending.Timeout)
			c.mutex.Lock()
			delete(c.pendingNonces, nonce)
			c.mutex.Unlock()
		}()
	}

	return nil
}

// sendRequest sends a request to the broker
func (c *HermesClient) sendRequest(service, messageID string, body []byte) error {
	if c.socket == nil {
		return fmt.Errorf("socket not initialized")
	}

	msg := &ClientMessage{
		Protocol:  HERMES_CLIENT,
		Command:   HERMES_REQ,
		Service:   service,
		MessageID: messageID,
		Body:      body,
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize client message: %w", err)
	}

	_, err = c.socket.SendMessage("", msgBytes)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}

	c.mutex.Lock()
	c.stats.RequestsSent++
	c.stats.LastRequest = time.Now()
	c.mutex.Unlock()

	c.logger.Debug().
		Str("service", service).
		Str("message_id", messageID).
		Int("body_size", len(body)).
		Msg("Request sent to broker")

	return nil
}

// messageLoop processes incoming messages
func (c *HermesClient) messageLoop() {
	c.logger.Info().Msg("Starting Hermes client message loop")

	for {
		select {
		case <-c.ctx.Done():
			c.logger.Info().Msg("Hermes client message loop stopping")
			return
		default:
			if c.socket == nil {
				time.Sleep(100 * time.Millisecond)
				continue
			}

			// Receive message from broker (non-blocking)
			msg, err := c.socket.RecvMessageBytes(zmq4.DONTWAIT)
			if err != nil {
				if err.Error() == "resource temporarily unavailable" {
					// No message available - sleep briefly to prevent busy waiting
					time.Sleep(10 * time.Millisecond)
					continue
				} else {
					c.logger.Error().Err(err).Msg("Failed to receive message from broker")
				}
				continue
			}

			c.logger.Debug().
				Int("message_parts", len(msg)).
				Msg("Client received message from broker")

			if len(msg) < 2 {
				c.logger.Warn().
					Int("parts_count", len(msg)).
					Msg("Received malformed message (insufficient parts)")
				continue
			}

			empty := msg[0] // Should be empty frame
			response := msg[1] // Response body

			if len(empty) != 0 {
				c.logger.Warn().Msg("Received message without empty delimiter")
				continue
			}

			c.logger.Debug().
				Int("response_size", len(response)).
				Msg("Received response from broker")

			// Handle response
			if err := c.handleResponse(response); err != nil {
				c.logger.Error().
					Err(err).
					Msg("Failed to handle response from broker")
			}
		}
	}
}

// handleResponse handles a response from the broker
func (c *HermesClient) handleResponse(responseBytes []byte) error {
	// Try to parse as service response first
	var serviceResp ServiceResponse
	if err := json.Unmarshal(responseBytes, &serviceResp); err == nil && serviceResp.MessageID != "" {
		return c.handleServiceResponse(&serviceResp)
	}

	// If not a service response, treat as raw response
	// We need to find the corresponding request, but without message ID this is difficult
	// For now, log and ignore
	c.logger.Warn().
		Int("response_size", len(responseBytes)).
		Str("response_preview", string(responseBytes[:min(100, len(responseBytes))])).
		Msg("Received response without message ID - ignoring")

	return nil
}

// handleServiceResponse handles a structured service response
func (c *HermesClient) handleServiceResponse(resp *ServiceResponse) error {
	// Try message ID correlation first
	c.mutex.RLock()
	pending, exists := c.pending[resp.MessageID]
	c.mutex.RUnlock()

	// If no message ID match, try nonce-based correlation
	if !exists && resp.Nonce != "" {
		c.mutex.RLock()
		noncePending, nonceExists := c.pendingNonces[resp.Nonce]
		c.mutex.RUnlock()

		if nonceExists {
			c.logger.Debug().
				Str("message_id", resp.MessageID).
				Str("nonce", resp.Nonce).
				Bool("fire_and_forget", noncePending.FireAndForget).
				Msg("Matched response using nonce correlation")
			
			// For fire-and-forget requests, just log and cleanup
			if noncePending.FireAndForget {
				c.mutex.Lock()
				delete(c.pendingNonces, resp.Nonce)
				c.stats.ResponsesReceived++
				c.stats.LastResponse = time.Now()
				c.mutex.Unlock()
				
				c.logger.Debug().
					Str("nonce", resp.Nonce).
					Bool("success", resp.Success).
					Msg("Received response for fire-and-forget request")
				return nil
			}
			
			// For regular async requests with nonce, use nonce pending request
			pending = noncePending
			exists = true
		}
	}

	if !exists {
		// Only warn if this doesn't look like a fire-and-forget response
		if resp.Nonce == "" {
			c.logger.Warn().
				Str("message_id", resp.MessageID).
				Msg("Received response for unknown request")
		} else {
			c.logger.Debug().
				Str("message_id", resp.MessageID).
				Str("nonce", resp.Nonce).
				Msg("Received response for unknown nonce (likely fire-and-forget)")
		}
		return nil
	}

	c.logger.Debug().
		Str("message_id", resp.MessageID).
		Str("service", resp.Service).
		Bool("success", resp.Success).
		Msg("Handling service response")

	// Convert response to bytes
	respBytes, err := json.Marshal(resp)
	if err != nil {
		select {
		case pending.Error <- fmt.Errorf("failed to marshal response: %w", err):
		default:
		}
		return err
	}

	// Send response to waiting request
	if resp.Success {
		select {
		case pending.Response <- respBytes:
		default:
		}
	} else {
		select {
		case pending.Error <- fmt.Errorf("service error: %s", resp.Error):
		default:
		}
	}

	return nil
}

// timeoutMonitor monitors and cleans up timed out requests
func (c *HermesClient) timeoutMonitor() {
	ticker := time.NewTicker(5 * time.Second) // Check every 5 seconds
	defer ticker.Stop()

	c.logger.Debug().Msg("Starting Hermes client timeout monitor")

	for {
		select {
		case <-ticker.C:
			c.cleanupTimeoutRequests()
		case <-c.ctx.Done():
			c.logger.Debug().Msg("Hermes client timeout monitor stopping")
			return
		}
	}
}

// cleanupTimeoutRequests removes requests that have exceeded their timeout
func (c *HermesClient) cleanupTimeoutRequests() {
	now := time.Now()
	expiredRequests := make([]string, 0)

	c.mutex.RLock()
	for messageID, pending := range c.pending {
		if now.Sub(pending.Timestamp) > pending.Timeout {
			expiredRequests = append(expiredRequests, messageID)
		}
	}
	c.mutex.RUnlock()

	// Clean up expired requests
	for _, messageID := range expiredRequests {
		c.mutex.Lock()
		if pending, exists := c.pending[messageID]; exists {
			delete(c.pending, messageID)
			c.stats.RequestsTimeout++
			
			// Notify waiting request of timeout
			select {
			case pending.Error <- fmt.Errorf("request timeout"):
			default:
			}
			close(pending.Response)
			close(pending.Error)
		}
		c.mutex.Unlock()

		c.logger.Debug().
			Str("message_id", messageID).
			Msg("Cleaned up timed out request")
	}
}

// recordLatency records a request latency for statistics
func (c *HermesClient) recordLatency(latency time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Add to latencies slice (keep last 100)
	c.latencies = append(c.latencies, latency)
	if len(c.latencies) > 100 {
		c.latencies = c.latencies[1:]
	}

	// Calculate average latency
	var total time.Duration
	for _, l := range c.latencies {
		total += l
	}
	if len(c.latencies) > 0 {
		c.stats.AverageLatency = float64(total.Nanoseconds()) / float64(len(c.latencies)) / 1e6 // Convert to milliseconds
	}
}

// GetStats returns client statistics
func (c *HermesClient) GetStats() *ClientStats {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	stats := *c.stats
	return &stats
}

// IsConnected returns whether the client is connected
func (c *HermesClient) IsConnected() bool {
	return c.socket != nil
}

// GetPendingCount returns the number of pending requests
func (c *HermesClient) GetPendingCount() int {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return len(c.pending)
}