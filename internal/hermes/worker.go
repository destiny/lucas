package hermes

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
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

// HermesWorker implements the Hermes Majordomo Protocol worker
type HermesWorker struct {
	broker          string
	service         string
	identity        string
	socket          *zmq4.Socket
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

// NewWorker creates a new Hermes worker
func NewWorker(broker, service, identity string, handler RequestHandler) *HermesWorker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &HermesWorker{
		broker:            broker,
		service:           service,
		identity:          identity,
		handler:           handler,
		heartbeat:         30 * time.Second, // Default heartbeat interval
		reconnect:         5 * time.Second,  // Default reconnection interval
		liveness:          10,               // Default liveness for internet tolerance
		state:             WorkerStateDisconnected,
		ctx:               ctx,
		cancel:            cancel,
		logger:            logger.New(),
		reconnectAttempt:  0,
		maxReconnectDelay: 60 * time.Second, // Maximum 60 second delay
		stats: &WorkerStats{
			StartTime: time.Now(),
		},
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

// Start starts the worker
func (w *HermesWorker) Start() error {
	w.logger.Info().
		Str("broker", w.broker).
		Str("service", w.service).
		Str("identity", w.identity).
		Msg("Starting Hermes worker")

	if err := w.connect(); err != nil {
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	// Start message processing loop
	go w.messageLoop()

	// Start heartbeat loop
	go w.heartbeatLoop()

	return nil
}

// Stop stops the worker
func (w *HermesWorker) Stop() error {
	w.logger.Info().Msg("Stopping Hermes worker")

	w.mutex.Lock()
	w.state = WorkerStateDisconnected
	w.mutex.Unlock()

	// Send disconnect message
	w.sendDisconnect()

	w.cancel()

	if w.socket != nil {
		if err := w.socket.Close(); err != nil {
			w.logger.Error().Err(err).Msg("Error closing worker socket")
		}
		w.socket = nil
	}

	w.logger.Info().Msg("Hermes worker stopped")
	return nil
}

// connect establishes connection to the broker
func (w *HermesWorker) connect() error {
	w.mutex.Lock()
	w.state = WorkerStateConnecting
	w.mutex.Unlock()

	w.logger.Info().
		Str("broker", w.broker).
		Msg("Connecting to Hermes broker")

	// Create DEALER socket
	socket, err := zmq4.NewSocket(zmq4.DEALER)
	if err != nil {
		return fmt.Errorf("failed to create DEALER socket: %w", err)
	}

	// Set socket identity
	if err = socket.SetIdentity(w.identity); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set socket identity: %w", err)
	}

	// Set socket options
	if err = socket.SetLinger(1000); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set linger: %w", err)
	}

	// Set receive timeout to be longer than heartbeat interval to avoid premature timeouts
	// Use 1.5x heartbeat interval to account for network delays and jitter
	receiveTimeout := time.Duration(float64(w.heartbeat) * 1.5)
	if err = socket.SetRcvtimeo(receiveTimeout); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set receive timeout: %w", err)
	}

	if err = socket.SetSndtimeo(30 * time.Second); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set send timeout: %w", err)
	}

	// Connect to broker
	if err = socket.Connect(w.broker); err != nil {
		socket.Close()
		return fmt.Errorf("failed to connect to broker: %w", err)
	}

	w.socket = socket
	w.liveness = 10

	// Send READY message to register with broker
	if err = w.sendReady(); err != nil {
		socket.Close()
		w.socket = nil
		return fmt.Errorf("failed to send READY message: %w", err)
	}

	w.mutex.Lock()
	w.state = WorkerStateReady
	w.mutex.Unlock()

	w.logger.Info().Msg("Connected to Hermes broker and ready for requests")
	return nil
}

// messageLoop processes incoming messages
func (w *HermesWorker) messageLoop() {
	w.logger.Info().Msg("Starting Hermes worker message loop")

	for {
		select {
		case <-w.ctx.Done():
			w.logger.Info().Msg("Hermes worker message loop stopping")
			return
		default:
			if w.socket == nil {
				time.Sleep(w.reconnect)
				continue
			}

			// Receive message from broker (non-blocking to prevent hang)
			msg, err := w.socket.RecvMessageBytes(zmq4.DONTWAIT)
			if err != nil {
				if err.Error() == "resource temporarily unavailable" {
					// No message available - normal with non-blocking mode
					// Don't decrement liveness here as this is expected behavior
					w.mutex.RLock()
					state := w.state
					w.mutex.RUnlock()
					
					if state == WorkerStateDisconnected {
						continue
					}
					
					// Small sleep to prevent busy waiting while staying responsive
					time.Sleep(10 * time.Millisecond)
					continue
				}
				
				w.logger.Error().Err(err).Msg("[HUB_DEBUG] Worker failed to receive message from broker")
				w.reconnectToBroker()
				continue
			}

			w.logger.Debug().
				Int("message_parts", len(msg)).
				Str("service", w.service).
				Msg("[HUB_DEBUG] Worker received message from broker")

			if len(msg) < 2 {
				w.logger.Warn().
					Int("parts_count", len(msg)).
					Msg("Received malformed message (insufficient parts)")
				continue
			}

			empty := msg[0] // Should be empty frame
			header := msg[1] // Protocol header

			if len(empty) != 0 {
				w.logger.Warn().Msg("Received message without empty delimiter")
				continue
			}

			w.logger.Debug().
				Int("parts_count", len(msg)).
				Str("header_preview", string(header[:min(50, len(header))]) + "...").
				Msg("Received message from broker")

			// Reset liveness on any valid message
			w.liveness = 10

			// Parse and handle message
			if err := w.handleMessage(msg[1:]); err != nil {
				w.logger.Error().
					Err(err).
					Msg("Failed to handle message from broker")
			}
		}
	}
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
		Msg("[HUB_DEBUG] Worker processing service request")

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
		Msg("[HUB_DEBUG] Worker request processed successfully, sending reply")

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

	_, err = w.socket.SendMessage("", msgBytes)
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

	_, err = w.socket.SendMessage("", msgBytes)
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

	_, err = w.socket.SendMessage("", msgBytes)
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

	_, err = w.socket.SendMessage("", msgBytes)
	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to send DISCONNECT message")
		return nil // Don't fail shutdown on send error
	}

	w.logger.Debug().Msg("Sent DISCONNECT message to broker")
	return nil
}

// heartbeatLoop sends periodic heartbeats to broker with jitter
func (w *HermesWorker) heartbeatLoop() {
	// Add jitter to prevent synchronization issues (±5 seconds)
	jitter := time.Duration(rand.Intn(10000)-5000) * time.Millisecond
	interval := w.heartbeat + jitter
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	w.logger.Info().
		Dur("base_interval", w.heartbeat).
		Dur("actual_interval", interval).
		Msg("Starting Hermes worker heartbeat loop with jitter")

	for {
		select {
		case <-ticker.C:
			w.mutex.RLock()
			state := w.state
			w.mutex.RUnlock()

			if state == WorkerStateReady && w.socket != nil {
				if err := w.sendHeartbeat(); err != nil {
					w.logger.Warn().Err(err).Msg("Failed to send heartbeat")
					w.liveness--
					if w.liveness <= 0 {
						w.reconnectToBroker()
					}
				}
			}
			
			// Update ticker with new jitter to avoid long-term synchronization
			jitter := time.Duration(rand.Intn(10000)-5000) * time.Millisecond
			newInterval := w.heartbeat + jitter
			ticker.Reset(newInterval)
		case <-w.ctx.Done():
			w.logger.Info().Msg("Hermes worker heartbeat loop stopping")
			return
		}
	}
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