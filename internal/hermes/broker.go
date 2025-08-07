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

// BrokerService represents a service in the broker
type BrokerService struct {
	Name        string
	Description string
	Workers     []*BrokerWorker
	Requests    []*BrokerPendingRequest
	Waiting     []*BrokerWorker
	mutex       sync.RWMutex
}

// BrokerWorker represents a worker in the broker
type BrokerWorker struct {
	Identity    string
	Service     string
	Address     string
	Expiry      time.Time
	LastPing    time.Time
	Status      string
	Liveness    int
	Requests    int
	mutex       sync.RWMutex
}

// BrokerPendingRequest represents a pending client request
type BrokerPendingRequest struct {
	ClientID  string
	MessageID string
	Service   string
	Body      []byte
	Timestamp time.Time
}

// Broker implements the Hermes Majordomo Protocol broker
type Broker struct {
	address     string
	socket      *zmq4.Socket
	services    map[string]*BrokerService
	workers     map[string]*BrokerWorker
	clients     map[string]time.Time
	heartbeat   time.Duration
	ctx         context.Context
	cancel      context.CancelFunc
	logger      zerolog.Logger
	stats       *BrokerStats
	mutex       sync.RWMutex
}

// NewBroker creates a new Hermes broker
func NewBroker(address string) *Broker {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Broker{
		address:   address,
		services:  make(map[string]*BrokerService),
		workers:   make(map[string]*BrokerWorker),
		clients:   make(map[string]time.Time),
		heartbeat: 30 * time.Second, // Default heartbeat interval
		ctx:       ctx,
		cancel:    cancel,
		logger:    logger.New(),
		stats: &BrokerStats{
			StartTime: time.Now(),
		},
	}
}

// Start starts the broker
func (b *Broker) Start() error {
	b.logger.Info().
		Str("address", b.address).
		Msg("Starting Hermes broker")

	// Create ROUTER socket
	socket, err := zmq4.NewSocket(zmq4.ROUTER)
	if err != nil {
		return fmt.Errorf("failed to create ROUTER socket: %w", err)
	}
	
	defer func() {
		if err != nil {
			socket.Close()
		}
	}()

	// Set socket options
	if err = socket.SetLinger(1000); err != nil {
		return fmt.Errorf("failed to set linger: %w", err)
	}

	if err = socket.SetRcvhwm(1000); err != nil {
		return fmt.Errorf("failed to set receive high watermark: %w", err)
	}

	if err = socket.SetSndhwm(1000); err != nil {
		return fmt.Errorf("failed to set send high watermark: %w", err)
	}

	// Bind to address
	if err = socket.Bind(b.address); err != nil {
		return fmt.Errorf("failed to bind to address: %w", err)
	}

	b.socket = socket

	b.logger.Info().Msg("Hermes broker started successfully")

	// Start message processing loop
	go b.messageLoop()

	// Start heartbeat monitor
	go b.heartbeatLoop()

	return nil
}

// Stop stops the broker
func (b *Broker) Stop() error {
	b.logger.Info().Msg("Stopping Hermes broker")

	b.cancel()

	if b.socket != nil {
		if err := b.socket.Close(); err != nil {
			b.logger.Error().Err(err).Msg("Error closing broker socket")
		}
		b.socket = nil
	}

	b.logger.Info().Msg("Hermes broker stopped")
	return nil
}

// messageLoop processes incoming messages
func (b *Broker) messageLoop() {
	b.logger.Info().Msg("Starting Hermes broker message loop")

	for {
		select {
		case <-b.ctx.Done():
			b.logger.Info().Msg("Hermes broker message loop stopping")
			return
		default:
			// Receive multipart message
			msg, err := b.socket.RecvMessageBytes(zmq4.DONTWAIT)
			if err != nil {
				if err.Error() != "resource temporarily unavailable" {
					b.logger.Error().Err(err).Msg("Failed to receive message")
				}
				time.Sleep(10 * time.Millisecond) // Small sleep to prevent busy waiting
				continue
			}

			if len(msg) < 3 {
				b.logger.Warn().
					Int("parts_count", len(msg)).
					Msg("Received malformed message (insufficient parts)")
				continue
			}

			sender := string(msg[0])
			empty := msg[1]   // Should be empty frame
			header := msg[2]  // Protocol header
			
			if len(empty) != 0 {
				b.logger.Warn().
					Str("sender", sender).
					Msg("Received message without empty delimiter")
				continue
			}

			b.logger.Debug().
				Str("sender", sender).
				Int("parts_count", len(msg)).
				Str("header_preview", string(header[:min(50, len(header))]) + "...").
				Msg("Received message")

			// Route message based on sender type and content
			if err := b.routeMessage(sender, msg[2:]); err != nil {
				b.logger.Error().
					Str("sender", sender).
					Err(err).
					Msg("Failed to route message")
			}
		}
	}
}

// routeMessage routes a message to the appropriate handler
func (b *Broker) routeMessage(sender string, msgParts [][]byte) error {
	if len(msgParts) == 0 {
		return fmt.Errorf("empty message parts")
	}

	// Try to parse as worker message first
	var workerMsg WorkerMessage
	if err := json.Unmarshal(msgParts[0], &workerMsg); err == nil && workerMsg.Protocol == HERMES_WORKER {
		return b.handleWorkerMessage(sender, &workerMsg, msgParts[1:])
	}

	// Try to parse as client message
	var clientMsg ClientMessage
	if err := json.Unmarshal(msgParts[0], &clientMsg); err == nil && clientMsg.Protocol == HERMES_CLIENT {
		return b.handleClientMessage(sender, &clientMsg)
	}

	return fmt.Errorf("unknown message format")
}

// handleWorkerMessage handles messages from workers
func (b *Broker) handleWorkerMessage(workerID string, msg *WorkerMessage, extraParts [][]byte) error {
	b.logger.Debug().
		Str("worker_id", workerID).
		Str("command", msg.Command).
		Str("service", msg.Service).
		Msg("Handling worker message")

	switch msg.Command {
	case HERMES_READY:
		return b.handleWorkerReady(workerID, msg.Service)
	case HERMES_REPLY:
		if len(extraParts) > 0 {
			return b.handleWorkerReply(workerID, msg.ClientID, extraParts[0])
		}
		return b.handleWorkerReply(workerID, msg.ClientID, msg.Body)
	case HERMES_HEARTBEAT:
		return b.handleWorkerHeartbeat(workerID)
	case HERMES_DISCONNECT:
		return b.handleWorkerDisconnect(workerID)
	default:
		return fmt.Errorf("unknown worker command: %s", msg.Command)
	}
}

// handleClientMessage handles messages from clients
func (b *Broker) handleClientMessage(clientID string, msg *ClientMessage) error {
	b.logger.Debug().
		Str("client_id", clientID).
		Str("command", msg.Command).
		Str("service", msg.Service).
		Str("message_id", msg.MessageID).
		Msg("Handling client message")

	switch msg.Command {
	case HERMES_REQ:
		return b.handleClientRequest(clientID, msg)
	default:
		return fmt.Errorf("unknown client command: %s", msg.Command)
	}
}

// handleWorkerReady handles worker registration
func (b *Broker) handleWorkerReady(workerID, serviceName string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	// Create or get service
	service, exists := b.services[serviceName]
	if !exists {
		service = &BrokerService{
			Name:        serviceName,
			Description: fmt.Sprintf("Service %s", serviceName),
			Workers:     []*BrokerWorker{},
			Requests:    []*BrokerPendingRequest{},
			Waiting:     []*BrokerWorker{},
		}
		b.services[serviceName] = service
	}

	// Create or update worker
	worker, exists := b.workers[workerID]
	if !exists {
		worker = &BrokerWorker{
			Identity: workerID,
			Service:  serviceName,
			Status:   "ready",
			Liveness: 3, // Default liveness
		}
		b.workers[workerID] = worker
	} else {
		// Update existing worker
		worker.Service = serviceName
		worker.Status = "ready"
		worker.Liveness = 3
	}

	worker.Expiry = time.Now().Add(b.heartbeat * 3) // 3 heartbeat intervals
	worker.LastPing = time.Now()

	// Add worker to service
	service.mutex.Lock()
	service.Workers = append(service.Workers, worker)
	service.Waiting = append(service.Waiting, worker)
	service.mutex.Unlock()

	b.logger.Info().
		Str("worker_id", workerID).
		Str("service", serviceName).
		Msg("Worker registered")

	// Process any pending requests for this service
	b.processPendingRequests(serviceName)

	return nil
}

// handleWorkerReply handles replies from workers
func (b *Broker) handleWorkerReply(workerID, clientID string, reply []byte) error {
	b.mutex.RLock()
	worker, exists := b.workers[workerID]
	b.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("unknown worker: %s", workerID)
	}

	// Update worker stats
	worker.mutex.Lock()
	worker.Requests++
	worker.LastPing = time.Now()
	worker.Expiry = time.Now().Add(b.heartbeat * 3)
	worker.mutex.Unlock()

	// Send reply to client
	return b.sendToClient(clientID, reply)
}

// handleWorkerHeartbeat handles heartbeat from workers
func (b *Broker) handleWorkerHeartbeat(workerID string) error {
	b.mutex.RLock()
	worker, exists := b.workers[workerID]
	b.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("unknown worker: %s", workerID)
	}

	worker.mutex.Lock()
	worker.LastPing = time.Now()
	worker.Expiry = time.Now().Add(b.heartbeat * 3)
	worker.Liveness = 3
	worker.mutex.Unlock()

	b.logger.Debug().
		Str("worker_id", workerID).
		Msg("Worker heartbeat received")

	return nil
}

// handleWorkerDisconnect handles worker disconnection
func (b *Broker) handleWorkerDisconnect(workerID string) error {
	return b.removeWorker(workerID)
}

// handleClientRequest handles requests from clients
func (b *Broker) handleClientRequest(clientID string, msg *ClientMessage) error {
	b.mutex.Lock()
	b.clients[clientID] = time.Now()
	b.stats.Requests++
	b.stats.LastRequest = time.Now()
	b.mutex.Unlock()

	// Get service
	b.mutex.RLock()
	service, exists := b.services[msg.Service]
	b.mutex.RUnlock()

	if !exists {
		// Service doesn't exist, send error to client
		errorResp := CreateServiceResponse(msg.MessageID, msg.Service, false, nil, 
			fmt.Errorf("service not available: %s", msg.Service))
		respBytes, _ := SerializeServiceResponse(errorResp)
		return b.sendToClient(clientID, respBytes)
	}

	// Try to assign to available worker
	service.mutex.Lock()
	defer service.mutex.Unlock()

	if len(service.Waiting) == 0 {
		// No workers available, queue the request
		request := &BrokerPendingRequest{
			ClientID:  clientID,
			MessageID: msg.MessageID,
			Service:   msg.Service,
			Body:      msg.Body,
			Timestamp: time.Now(),
		}
		service.Requests = append(service.Requests, request)

		b.logger.Debug().
			Str("client_id", clientID).
			Str("service", msg.Service).
			Str("message_id", msg.MessageID).
			Msg("Request queued - no workers available")
		return nil
	}

	// Get available worker
	worker := service.Waiting[0]
	service.Waiting = service.Waiting[1:]

	// Send request to worker
	return b.sendToWorker(worker.Identity, clientID, msg.Body)
}

// processPendingRequests processes queued requests for a service
func (b *Broker) processPendingRequests(serviceName string) {
	b.mutex.RLock()
	service, exists := b.services[serviceName]
	b.mutex.RUnlock()

	if !exists {
		return
	}

	service.mutex.Lock()
	defer service.mutex.Unlock()

	for len(service.Requests) > 0 && len(service.Waiting) > 0 {
		request := service.Requests[0]
		service.Requests = service.Requests[1:]

		worker := service.Waiting[0]
		service.Waiting = service.Waiting[1:]

		// Send request to worker
		if err := b.sendToWorker(worker.Identity, request.ClientID, request.Body); err != nil {
			b.logger.Error().
				Str("worker_id", worker.Identity).
				Str("client_id", request.ClientID).
				Err(err).
				Msg("Failed to send pending request to worker")
		}
	}
}

// sendToWorker sends a message to a worker
func (b *Broker) sendToWorker(workerID, clientID string, body []byte) error {
	if b.socket == nil {
		return fmt.Errorf("broker socket not initialized")
	}

	workerMsg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_REQUEST,
		Body:     body,
		ClientID: clientID,
	}

	msgBytes, err := SerializeMessage(workerMsg)
	if err != nil {
		return fmt.Errorf("failed to serialize worker message: %w", err)
	}

	_, err = b.socket.SendMessage(workerID, "", msgBytes)
	if err != nil {
		return fmt.Errorf("failed to send message to worker: %w", err)
	}

	b.logger.Debug().
		Str("worker_id", workerID).
		Str("client_id", clientID).
		Msg("Request sent to worker")

	return nil
}

// sendToClient sends a message to a client
func (b *Broker) sendToClient(clientID string, body []byte) error {
	if b.socket == nil {
		return fmt.Errorf("broker socket not initialized")
	}

	_, err := b.socket.SendMessage(clientID, "", body)
	if err != nil {
		return fmt.Errorf("failed to send message to client: %w", err)
	}

	b.mutex.Lock()
	b.stats.Responses++
	b.mutex.Unlock()

	b.logger.Debug().
		Str("client_id", clientID).
		Msg("Response sent to client")

	return nil
}

// heartbeatLoop manages worker heartbeats and cleanup
func (b *Broker) heartbeatLoop() {
	ticker := time.NewTicker(b.heartbeat)
	defer ticker.Stop()

	b.logger.Info().
		Dur("interval", b.heartbeat).
		Msg("Starting Hermes broker heartbeat loop")

	for {
		select {
		case <-ticker.C:
			b.checkWorkerLiveness()
		case <-b.ctx.Done():
			b.logger.Info().Msg("Hermes broker heartbeat loop stopping")
			return
		}
	}
}

// checkWorkerLiveness checks and removes expired workers
func (b *Broker) checkWorkerLiveness() {
	now := time.Now()
	expiredWorkers := make([]string, 0)

	b.mutex.RLock()
	for workerID, worker := range b.workers {
		worker.mutex.RLock()
		if now.After(worker.Expiry) {
			expiredWorkers = append(expiredWorkers, workerID)
		}
		worker.mutex.RUnlock()
	}
	b.mutex.RUnlock()

	// Remove expired workers
	for _, workerID := range expiredWorkers {
		b.logger.Warn().
			Str("worker_id", workerID).
			Msg("Worker expired - removing")
		b.removeWorker(workerID)
	}
}

// removeWorker removes a worker from all data structures
func (b *Broker) removeWorker(workerID string) error {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	worker, exists := b.workers[workerID]
	if !exists {
		return fmt.Errorf("worker not found: %s", workerID)
	}

	// Remove from service
	if service, exists := b.services[worker.Service]; exists {
		service.mutex.Lock()
		// Remove from workers list
		for i, w := range service.Workers {
			if w.Identity == workerID {
				service.Workers = append(service.Workers[:i], service.Workers[i+1:]...)
				break
			}
		}
		// Remove from waiting list
		for i, w := range service.Waiting {
			if w.Identity == workerID {
				service.Waiting = append(service.Waiting[:i], service.Waiting[i+1:]...)
				break
			}
		}
		service.mutex.Unlock()
	}

	// Remove from workers map
	delete(b.workers, workerID)

	b.logger.Info().
		Str("worker_id", workerID).
		Str("service", worker.Service).
		Msg("Worker removed")

	return nil
}

// GetStats returns broker statistics
func (b *Broker) GetStats() *BrokerStats {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	stats := *b.stats
	stats.Services = len(b.services)
	stats.Workers = len(b.workers)
	return &stats
}

// GetServices returns information about all services
func (b *Broker) GetServices() map[string]*ServiceInfo {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	services := make(map[string]*ServiceInfo)
	for name, service := range b.services {
		service.mutex.RLock()
		workers := make([]string, len(service.Workers))
		for i, worker := range service.Workers {
			workers[i] = worker.Identity
		}
		
		serviceInfo := &ServiceInfo{
			Name:        service.Name,
			Description: service.Description,
			Workers:     workers,
			Status:      "active",
		}
		if len(service.Workers) > 0 {
			serviceInfo.LastSeen = service.Workers[0].LastPing
		}
		services[name] = serviceInfo
		service.mutex.RUnlock()
	}

	return services
}

// GetWorkers returns information about all workers
func (b *Broker) GetWorkers() map[string]*WorkerInfo {
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	workers := make(map[string]*WorkerInfo)
	for identity, worker := range b.workers {
		worker.mutex.RLock()
		workers[identity] = &WorkerInfo{
			Identity: worker.Identity,
			Service:  worker.Service,
			Expiry:   worker.Expiry,
			LastPing: worker.LastPing,
			Status:   worker.Status,
			Liveness: worker.Liveness,
			Requests: worker.Requests,
		}
		worker.mutex.RUnlock()
	}

	return workers
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetAddress returns the broker's bind address
func (b *Broker) GetAddress() string {
	return b.address
}