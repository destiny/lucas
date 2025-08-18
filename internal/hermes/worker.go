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
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/destiny/zmq4/v25"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// WorkerState represents the state of a worker
type WorkerState int

const (
	WorkerStateDisconnected WorkerState = iota
	WorkerStateConnecting
	WorkerStateReady
	WorkerStateWorking
	WorkerStateReconnecting
)

// HermesWorker implements the Hermes Majordomo Protocol worker with channel-based architecture
type HermesWorker struct {
	broker          string
	service         string
	identity        string
	socket          zmq4.Socket
	heartbeat       time.Duration
	reconnect       time.Duration
	liveness        int
	handler         RequestHandler
	state           WorkerState
	ctx             context.Context
	cancel          context.CancelFunc
	logger          zerolog.Logger
	stats           *WorkerStats
	mutex           sync.RWMutex
	requestCount    int
	reconnectAttempt int           // Track reconnection attempts for backoff
	maxReconnectDelay time.Duration // Maximum backoff delay
	
	// Channel-based architecture
	messagesCh      chan zmq4.Msg     // Incoming messages from broker
	heartbeatCh     chan time.Time    // Heartbeat events
	reconnectCh     chan struct{}     // Reconnection requests
	shutdownCh      chan struct{}     // Shutdown signal
	errorsCh        chan error        // Error notifications
	statsCh         chan *WorkerStats // Stats updates
}

// WorkerStats represents worker statistics
type WorkerStats struct {
	RequestsHandled     int       `json:"requests_handled"`
	RequestsFailed      int       `json:"requests_failed"`
	LastRequest         time.Time `json:"last_request"`
	StartTime           time.Time `json:"start_time"`
	Reconnections       int       `json:"reconnections"`
	CurrentLiveness     int       `json:"current_liveness"`
	State               string    `json:"state"`
	HeartbeatsSent      int       `json:"heartbeats_sent"`
	HeartbeatsReceived  int       `json:"heartbeats_received"`
	LastHeartbeatSent   time.Time `json:"last_heartbeat_sent"`
	LastHeartbeatReceived time.Time `json:"last_heartbeat_received"`
}

// NewWorker creates a new Hermes worker with channel-based architecture
func NewWorker(broker, service, identity string, handler RequestHandler) *HermesWorker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HermesWorker{
		broker:            broker,
		service:           service,
		identity:          identity,
		handler:           handler,
		heartbeat:         GetMDPHeartbeatInterval(), // Use RFC 7/MDP standard interval
		reconnect:         5 * time.Second,           // Default reconnection interval
		liveness:          MDP_HEARTBEAT_LIVENESS,    // Use RFC 7/MDP standard liveness
		state:             WorkerStateDisconnected,
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger.New(),
		reconnectAttempt:  0,
		maxReconnectDelay: 60 * time.Second, // Maximum 60 second delay
		stats: &WorkerStats{
			StartTime: time.Now(),
		},
		// Initialize channels
		messagesCh:  make(chan zmq4.Msg, 100),     // Buffered for high throughput
		heartbeatCh: make(chan time.Time, 10),     // Buffered heartbeat events
		reconnectCh: make(chan struct{}, 1),       // Single reconnection signal
		shutdownCh:  make(chan struct{}, 1),       // Single shutdown signal
		errorsCh:    make(chan error, 50),         // Buffered error notifications
		statsCh:     make(chan *WorkerStats, 10),  // Buffered stats updates
	}
}

// SetHeartbeat sets the heartbeat interval
func (w *HermesWorker) SetHeartbeat(interval time.Duration) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.heartbeat = interval
}

// SetReconnectInterval sets the reconnection interval
func (w *HermesWorker) SetReconnectInterval(interval time.Duration) {
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.reconnect = interval
}

// Start starts the worker with channel-based architecture
func (w *HermesWorker) Start() error {
	w.logger.Info().
		Str("broker", w.broker).
		Str("service", w.service).
		Str("identity", w.identity).
		Msg("Starting Hermes worker with channel-based architecture")

	if err := w.connect(); err != nil {
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	// Start channel-based workers
	go w.socketReader()      // Read from socket and feed messagesCh
	go w.messageProcessor()  // Process messages from messagesCh
	go w.heartbeatManager()  // Manage heartbeats using heartbeatCh
	go w.errorHandler()      // Handle errors from errorsCh
	go w.statsManager()      // Manage stats updates from statsCh

	return nil
}

// Stop stops the worker and closes all channels
func (w *HermesWorker) Stop() error {
	w.logger.Info().Msg("Stopping Hermes worker")

	w.mutex.Lock()
	w.state = WorkerStateDisconnected
	w.mutex.Unlock()

	// Send disconnect message
	w.sendDisconnect()

	// Signal shutdown to all channel workers
	select {
	case w.shutdownCh <- struct{}{}:
	default:
	}

	w.cancel()

	if w.socket != nil {
		if err := w.socket.Close(); err != nil {
			w.logger.Error().Err(err).Msg("Error closing worker socket")
		}
		w.socket = nil
	}

	// Close channels (done by workers when they see shutdown signal)
	w.logger.Info().Msg("Hermes worker stopped")
	return nil
}

// connect establishes connection to the broker with retry logic
func (w *HermesWorker) connect() error {
	w.mutex.Lock()
	w.state = WorkerStateConnecting
	w.mutex.Unlock()

	w.logger.Info().
		Str("broker", w.broker).
		Msg("Connecting to Hermes broker")

	maxRetries := 10
	baseDelay := 250 * time.Millisecond
	
	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := time.Duration(attempt) * baseDelay
			w.logger.Warn().
				Int("attempt", attempt+1).
				Int("max_retries", maxRetries).
				Dur("delay", delay).
				Msg("Retrying broker connection")
			time.Sleep(delay)
		}

		// Create DEALER socket
		socket := zmq4.NewDealer(w.ctx)

		// Set high watermark option if available
		if err := socket.SetOption(zmq4.OptionHWM, 1000); err != nil {
			w.logger.Warn().Err(err).Msg("Failed to set high watermark - continuing without it")
		}

		// Connect to broker
		if err := socket.Dial(w.broker); err != nil {
			socket.Close()
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to connect to broker after %d attempts: %w", maxRetries, err)
			}
			w.logger.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Msg("Failed to connect to broker, will retry")
			continue
		}

		w.socket = socket
		w.liveness = 10

		// Send READY message to register with broker
		if err := w.sendReady(); err != nil {
			socket.Close()
			w.socket = nil
			if attempt == maxRetries-1 {
				return fmt.Errorf("failed to send READY message after %d attempts: %w", maxRetries, err)
			}
			w.logger.Warn().
				Err(err).
				Int("attempt", attempt+1).
				Msg("Failed to send READY message, will retry")
			continue
		}

		w.logger.Info().
			Int("attempt", attempt+1).
			Msg("Connected to Hermes broker and ready for requests")
		return nil
	}

	return fmt.Errorf("failed to connect to broker after %d attempts", maxRetries)

	w.mutex.Lock()
	w.state = WorkerStateReady
	w.mutex.Unlock()

	w.logger.Info().Msg("Connected to Hermes broker and ready for requests")
	return nil
}


// handleMessage handles a message from the broker
func (w *HermesWorker) handleMessage(msgParts [][]byte) error {
	if len(msgParts) == 0 {
		return fmt.Errorf("empty message parts")
	}

	// Parse worker message
	var workerMsg WorkerMessage
	if err := json.Unmarshal(msgParts[0], &workerMsg); err != nil {
		return fmt.Errorf("failed to parse worker message: %w", err)
	}

	if workerMsg.Protocol != HERMES_WORKER {
		return fmt.Errorf("invalid protocol: %s", workerMsg.Protocol)
	}

	w.logger.Debug().
		Str("command", workerMsg.Command).
		Str("client_id", workerMsg.ClientID).
		Msg("Handling broker message")

	switch workerMsg.Command {
	case HERMES_REQUEST:
		return w.handleRequest(workerMsg.ClientID, workerMsg.Body, msgParts[1:])
	case HERMES_HEARTBEAT:
		return w.handleHeartbeat()
	case HERMES_DISCONNECT:
		return w.handleDisconnect()
	default:
		return fmt.Errorf("unknown broker command: %s", workerMsg.Command)
	}
}

// handleRequest handles a service request
func (w *HermesWorker) handleRequest(clientID string, body []byte, extraParts [][]byte) error {
	w.mutex.Lock()
	w.state = WorkerStateWorking
	w.requestCount++
	w.stats.LastRequest = time.Now()
	requestNum := w.requestCount
	w.mutex.Unlock()

	w.logger.Info().
		Str("client_id", clientID).
		Int("request_num", requestNum).
		Int("body_size", len(body)).
		Str("service", w.service).
		Msg("Worker processing service request")

	// Use body from extra parts if available (for large messages)
	requestBody := body
	if len(extraParts) > 0 && len(extraParts[0]) > 0 {
		requestBody = extraParts[0]
	}

	// Process request using handler
	var response []byte
	var err error
	
	if w.handler != nil {
		response, err = w.handler.Handle(requestBody)
	} else {
		err = fmt.Errorf("no request handler configured")
	}

	// Update stats
	w.mutex.Lock()
	if err != nil {
		w.stats.RequestsFailed++
	} else {
		w.stats.RequestsHandled++
	}
	w.state = WorkerStateReady
	w.mutex.Unlock()

	if err != nil {
		w.logger.Error().
			Str("client_id", clientID).
			Int("request_num", requestNum).
			Err(err).
			Msg("Request processing failed")

		// Send error response
		errorResp := CreateServiceResponse("", w.service, false, nil, err)
		response, _ = SerializeServiceResponse(errorResp)
	}

	// Send reply to broker
	if err := w.sendReply(clientID, response); err != nil {
		w.logger.Error().
			Str("client_id", clientID).
			Int("request_num", requestNum).
			Err(err).
			Msg("Failed to send reply")
		return err
	}

	w.logger.Info().
		Str("client_id", clientID).
		Int("request_num", requestNum).
		Int("response_size", len(response)).
		Str("service", w.service).
		Msg("Worker request processed successfully, sending reply")

	return nil
}

// handleHeartbeat handles heartbeat from broker
func (w *HermesWorker) handleHeartbeat() error {
	// Update heartbeat received statistics
	w.mutex.Lock()
	w.stats.HeartbeatsReceived++
	w.stats.LastHeartbeatReceived = time.Now()
	w.mutex.Unlock()

	w.logger.Debug().
		Int("total_received", w.stats.HeartbeatsReceived).
		Msg("Received heartbeat response from broker")
	w.liveness = 10
	return nil
}

// handleDisconnect handles disconnect command from broker
func (w *HermesWorker) handleDisconnect() error {
	w.logger.Info().Msg("Received disconnect command from broker")
	w.reconnectToBroker()
	return nil
}

// sendReady sends READY message to broker
func (w *HermesWorker) sendReady() error {
	if w.socket == nil {
		return fmt.Errorf("socket not initialized")
	}

	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_READY,
		Service:  w.service,
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize READY message: %w", err)
	}

	err = w.socket.Send(zmq4.NewMsgFrom([]byte(""), msgBytes))
	if err != nil {
		return fmt.Errorf("failed to send READY message: %w", err)
	}

	w.logger.Debug().
		Str("service", w.service).
		Msg("Sent READY message to broker")

	return nil
}

// sendReply sends reply to broker
func (w *HermesWorker) sendReply(clientID string, body []byte) error {
	if w.socket == nil {
		return fmt.Errorf("socket not initialized")
	}

	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_REPLY,
		ClientID: clientID,
		Body:     body,
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize REPLY message: %w", err)
	}

	err = w.socket.Send(zmq4.NewMsgFrom([]byte(""), msgBytes))
	if err != nil {
		return fmt.Errorf("failed to send REPLY message: %w", err)
	}

	w.logger.Debug().
		Str("client_id", clientID).
		Int("body_size", len(body)).
		Msg("Sent REPLY message to broker")

	return nil
}

// sendHeartbeat sends heartbeat to broker
func (w *HermesWorker) sendHeartbeat() error {
	if w.socket == nil {
		return fmt.Errorf("socket not initialized")
	}

	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_HEARTBEAT,
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to serialize HEARTBEAT message: %w", err)
	}

	err = w.socket.Send(zmq4.NewMsgFrom([]byte(""), msgBytes))
	if err != nil {
		return fmt.Errorf("failed to send HEARTBEAT message: %w", err)
	}

	// Update heartbeat statistics
	w.mutex.Lock()
	w.stats.HeartbeatsSent++
	w.stats.LastHeartbeatSent = time.Now()
	w.mutex.Unlock()

	w.logger.Debug().
		Int("total_sent", w.stats.HeartbeatsSent).
		Msg("Sent HEARTBEAT message to broker")
	return nil
}

// sendDisconnect sends disconnect message to broker
func (w *HermesWorker) sendDisconnect() error {
	if w.socket == nil {
		return nil // Already disconnected
	}

	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_DISCONNECT,
	}

	msgBytes, err := SerializeMessage(msg)
	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to serialize DISCONNECT message")
		return nil // Don't fail shutdown on serialization error
	}

	err = w.socket.Send(zmq4.NewMsgFrom([]byte(""), msgBytes))
	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to send DISCONNECT message")
		return nil // Don't fail shutdown on send error
	}

	w.logger.Debug().Msg("Sent DISCONNECT message to broker")
	return nil
}


// reconnectToBroker attempts to reconnect to the broker with exponential backoff
func (w *HermesWorker) reconnectToBroker() {
	w.mutex.Lock()
	if w.state == WorkerStateReconnecting {
		w.mutex.Unlock()
		return // Already reconnecting
	}
	w.state = WorkerStateReconnecting
	w.stats.Reconnections++
	w.reconnectAttempt++
	w.mutex.Unlock()

	// Calculate exponential backoff delay with jitter
	// Formula: min(maxDelay, baseDelay * 2^attempt) + jitter
	baseDelay := w.reconnect
	backoffDelay := time.Duration(1 << uint(w.reconnectAttempt-1)) * baseDelay
	if backoffDelay > w.maxReconnectDelay {
		backoffDelay = w.maxReconnectDelay
	}
	
	// Add jitter (±25% of delay) to prevent thundering herd
	jitterRange := int64(backoffDelay / 4)
	jitter := time.Duration(rand.Int63n(jitterRange*2) - jitterRange)
	actualDelay := backoffDelay + jitter
	
	w.logger.Warn().
		Int("attempt", w.reconnectAttempt).
		Dur("delay", actualDelay).
		Msg("Reconnecting to broker with exponential backoff")

	// Close existing socket
	if w.socket != nil {
		w.socket.Close()
		w.socket = nil
	}

	// Wait before reconnecting
	time.Sleep(actualDelay)

	// Attempt to reconnect
	if err := w.connect(); err != nil {
		w.logger.Error().
			Err(err).
			Int("attempt", w.reconnectAttempt).
			Msg("Failed to reconnect to broker")
		w.mutex.Lock()
		w.state = WorkerStateDisconnected
		w.mutex.Unlock()
		
		// Schedule another reconnection attempt
		go func() {
			if w.ctx.Err() == nil { // Only if not shutting down
				w.reconnectToBroker()
			}
		}()
	} else {
		// Reset attempt counter on successful connection
		w.mutex.Lock()
		w.reconnectAttempt = 0
		w.mutex.Unlock()
		w.logger.Info().Msg("Successfully reconnected to broker")
	}
}

// GetStats returns worker statistics
func (w *HermesWorker) GetStats() *WorkerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()

	stats := *w.stats
	stats.CurrentLiveness = w.liveness
	stats.State = w.getStateString()
	return &stats
}

// getStateString returns string representation of worker state
func (w *HermesWorker) getStateString() string {
	switch w.state {
	case WorkerStateDisconnected:
		return "disconnected"
	case WorkerStateConnecting:
		return "connecting"
	case WorkerStateReady:
		return "ready"
	case WorkerStateWorking:
		return "working"
	case WorkerStateReconnecting:
		return "reconnecting"
	default:
		return "unknown"
	}
}

// IsConnected returns whether the worker is connected to the broker
func (w *HermesWorker) IsConnected() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	return w.state == WorkerStateReady || w.state == WorkerStateWorking
}

// GetService returns the service name this worker provides
func (w *HermesWorker) GetService() string {
	return w.service
}

// GetIdentity returns the worker identity
func (w *HermesWorker) GetIdentity() string {
	return w.identity
}

// isTemporaryError checks if an error is temporary and expected
func (w *HermesWorker) isTemporaryError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return errStr == "resource temporarily unavailable" ||
		   errStr == "no message available" ||
		   errStr == "operation would block"
}

// isConnectionError checks if an error indicates a connection problem
func (w *HermesWorker) isConnectionError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "connection") ||
		   strings.Contains(errStr, "network") ||
		   strings.Contains(errStr, "broken pipe") ||
		   strings.Contains(errStr, "socket") ||
		   strings.Contains(errStr, "peer") ||
		   strings.Contains(errStr, "closed") ||
		   strings.Contains(errStr, "reset")
}

// Channel-based Worker Methods

// socketReader reads messages from socket and feeds them to messagesCh
func (w *HermesWorker) socketReader() {
	w.logger.Info().Msg("Starting worker socket reader")
	defer w.logger.Info().Msg("Worker socket reader stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownCh:
			return
		default:
			if w.socket == nil {
				time.Sleep(w.reconnect)
				continue
			}

			// Receive message from broker (non-blocking)
			rawMsg, err := w.socket.Recv()
			if err != nil {
				if w.isTemporaryError(err) {
					w.mutex.RLock()
					state := w.state
					w.mutex.RUnlock()
					
					if state == WorkerStateDisconnected {
						continue
					}
					
					time.Sleep(10 * time.Millisecond)
					continue
				}
				
				// Send error to error handler
				select {
				case w.errorsCh <- err:
				default:
				}
				continue
			}
			
			// Send message to processor
			select {
			case w.messagesCh <- rawMsg:
			case <-w.ctx.Done():
				return
			case <-w.shutdownCh:
				return
			default:
				w.logger.Warn().Msg("Message channel full, dropping message")
			}
		}
	}
}

// messageProcessor processes messages from messagesCh
func (w *HermesWorker) messageProcessor() {
	w.logger.Info().Msg("Starting worker message processor")
	defer w.logger.Info().Msg("Worker message processor stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownCh:
			return
		case rawMsg := <-w.messagesCh:
			if err := w.processMessage(rawMsg); err != nil {
				select {
				case w.errorsCh <- err:
				default:
				}
			}
		}
	}
}

// processMessage processes a single message
func (w *HermesWorker) processMessage(rawMsg zmq4.Msg) error {
	// Convert message to bytes
	msg := make([][]byte, len(rawMsg.Frames))
	for i, frame := range rawMsg.Frames {
		msg[i] = frame
	}

	w.logger.Debug().
		Int("message_parts", len(msg)).
		Str("service", w.service).
		Msg("Worker received message from broker")

	if len(msg) < 2 {
		return fmt.Errorf("received malformed message (insufficient parts): %d", len(msg))
	}

	empty := msg[0] // Should be empty frame

	if len(empty) != 0 {
		return fmt.Errorf("received message without empty delimiter")
	}

	// Reset liveness on any valid message
	w.liveness = MDP_HEARTBEAT_LIVENESS

	// Parse and handle message
	return w.handleMessage(msg[1:])
}

// heartbeatManager manages heartbeat timing using channels
func (w *HermesWorker) heartbeatManager() {
	w.logger.Info().Msg("Starting worker heartbeat manager")
	defer w.logger.Info().Msg("Worker heartbeat manager stopped")

	// Add jitter to prevent synchronization issues (±5 seconds)
	jitter := time.Duration(rand.Intn(10000)-5000) * time.Millisecond
	interval := w.heartbeat + jitter
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownCh:
			return
		case t := <-ticker.C:
			w.heartbeatCh <- t
			w.mutex.RLock()
			state := w.state
			w.mutex.RUnlock()

			if state == WorkerStateReady && w.socket != nil {
				if err := w.sendHeartbeat(); err != nil {
					select {
					case w.errorsCh <- fmt.Errorf("heartbeat failed: %w", err):
					default:
					}
					w.liveness--
					if w.liveness <= 0 {
						select {
						case w.reconnectCh <- struct{}{}:
						default:
						}
					}
				}
			}
			
			// Update ticker with new jitter
			jitter := time.Duration(rand.Intn(10000)-5000) * time.Millisecond
			newInterval := w.heartbeat + jitter
			ticker.Reset(newInterval)
		}
	}
}

// errorHandler handles errors from errorsCh
func (w *HermesWorker) errorHandler() {
	w.logger.Info().Msg("Starting worker error handler")
	defer w.logger.Info().Msg("Worker error handler stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownCh:
			return
		case err := <-w.errorsCh:
			if w.isConnectionError(err) {
				w.logger.Warn().Err(err).Msg("Worker connection error - triggering reconnection")
				select {
				case w.reconnectCh <- struct{}{}:
				default:
				}
			} else {
				w.logger.Error().Err(err).Msg("Worker error")
			}
		}
	}
}

// statsManager manages statistics updates from statsCh
func (w *HermesWorker) statsManager() {
	w.logger.Info().Msg("Starting worker stats manager")
	defer w.logger.Info().Msg("Worker stats manager stopped")

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.shutdownCh:
			return
		case stats := <-w.statsCh:
			w.mutex.Lock()
			w.stats = stats
			w.mutex.Unlock()
		}
	}
}