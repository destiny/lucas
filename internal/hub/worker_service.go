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

package hub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"lucas/internal/hermes"
	"lucas/internal/logger"
)

// WorkerService integrates Hermes worker with hub functionality
type WorkerService struct {
	config       *Config
	deviceMgr    *DeviceManager
	workers      map[string]*hermes.HermesWorker
	logger       zerolog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	stats        *WorkerServiceStats
	mutex        sync.RWMutex
}

// WorkerServiceStats represents statistics for the worker service
type WorkerServiceStats struct {
	RegisteredServices int                              `json:"registered_services"`
	ActiveWorkers      int                              `json:"active_workers"`
	TotalRequests      int                              `json:"total_requests"`
	FailedRequests     int                              `json:"failed_requests"`
	StartTime          time.Time                        `json:"start_time"`
	LastRequest        time.Time                        `json:"last_request"`
	ServiceStats       map[string]*ServiceWorkerStats   `json:"service_stats"`
}

// ServiceWorkerStats represents statistics for a specific service worker
type ServiceWorkerStats struct {
	ServiceName    string    `json:"service_name"`
	RequestsHandled int      `json:"requests_handled"`
	RequestsFailed  int      `json:"requests_failed"`
	LastRequest    time.Time `json:"last_request"`
	IsConnected    bool      `json:"is_connected"`
	WorkerIdentity string    `json:"worker_identity"`
}

// DeviceServiceHandler removed - using single HubServiceHandler for all devices

// HubServiceHandler handles requests for any device through the hub
type HubServiceHandler struct {
	deviceMgr *DeviceManager
	config    *Config
	logger    zerolog.Logger
	stats     *ServiceHandlerStats
	mutex     sync.RWMutex
}

// ServiceHandlerStats represents statistics for a service handler
type ServiceHandlerStats struct {
	RequestsProcessed int       `json:"requests_processed"`
	RequestsFailed    int       `json:"requests_failed"`
	LastRequest       time.Time `json:"last_request"`
	AverageLatency    float64   `json:"average_latency_ms"`
	ErrorRate         float64   `json:"error_rate"`
}

// NewWorkerService creates a new worker service
func NewWorkerService(config *Config, deviceMgr *DeviceManager) *WorkerService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &WorkerService{
		config:    config,
		deviceMgr: deviceMgr,
		workers:   make(map[string]*hermes.HermesWorker),
		logger:    logger.New(),
		ctx:       ctx,
		cancel:    cancel,
		stats: &WorkerServiceStats{
			StartTime:    time.Now(),
			ServiceStats: make(map[string]*ServiceWorkerStats),
		},
	}
}

// Start starts the worker service and registers device services
func (ws *WorkerService) Start() error {
	ws.logger.Info().Msg("Starting Hub Worker Service")

	// Register hub as a single worker (not per device type)
	if err := ws.registerHubWorker(); err != nil {
		return fmt.Errorf("failed to register hub worker: %w", err)
	}

	// Start all workers
	for serviceName, worker := range ws.workers {
		if err := worker.Start(); err != nil {
			ws.logger.Error().
				Str("service", serviceName).
				Err(err).
				Msg("Failed to start worker")
			return fmt.Errorf("failed to start worker for service %s: %w", serviceName, err)
		}
	}

	// Start monitoring
	go ws.monitorWorkers()

	// Gateway will auto-detect the hub service when worker registers

	ws.logger.Info().
		Int("registered_services", len(ws.workers)).
		Msg("Hub Worker Service started successfully")


	return nil
}

// Stop stops the worker service and all workers
func (ws *WorkerService) Stop() error {
	ws.logger.Info().Msg("Stopping Hub Worker Service")

	ws.cancel()

	// Stop all workers
	ws.mutex.RLock()
	workers := make(map[string]*hermes.HermesWorker)
	for name, worker := range ws.workers {
		workers[name] = worker
	}
	ws.mutex.RUnlock()

	for serviceName, worker := range workers {
		if err := worker.Stop(); err != nil {
			ws.logger.Error().
				Str("service", serviceName).
				Err(err).
				Msg("Error stopping worker")
		}
	}

	ws.logger.Info().Msg("Hub Worker Service stopped")
	return nil
}

// registerHubWorker registers the hub as a single Hermes worker
func (ws *WorkerService) registerHubWorker() error {
	// Get transport type from config (defaults to "zmq" if not set)
	transport := ws.config.GetTransport()
	
	ws.logger.Info().
		Int("device_count", len(ws.config.Devices)).
		Str("transport", transport).
		Str("endpoint", ws.config.Gateway.Endpoint).
		Msg("Registering hub worker")

	// Use hub service name and hub ID as worker identity
	serviceName := "hub.control"
	workerIdentity := ws.config.Hub.ID // Use hub ID directly as worker identity
	
	// Create hub service handler that can handle all device types
	handler := &HubServiceHandler{
		deviceMgr: ws.deviceMgr,
		config:    ws.config,
		logger:    ws.logger,
		stats:     &ServiceHandlerStats{},
	}

	// TODO: Future enhancement - create different worker types based on transport
	// For now, Hermes only supports ZMQ, but endpoint may vary
	// When CoAP/HTTP support is added, create appropriate worker type here
	switch transport {
	case "zmq":
		// Create ZMQ-based Hermes worker (current implementation)
	case "coap":
		ws.logger.Warn().Msg("CoAP transport configured but not yet implemented, falling back to ZMQ")
	case "http":
		ws.logger.Warn().Msg("HTTP transport configured but not yet implemented, falling back to ZMQ")
	default:
		ws.logger.Warn().Str("transport", transport).Msg("Unknown transport type, using ZMQ")
	}

	// Create Hermes worker (currently ZMQ-only)
	worker := hermes.NewWorker(
		ws.config.Gateway.Endpoint,
		serviceName,
		workerIdentity,
		handler,
	)

	// Configure worker settings for internet reliability
	worker.SetHeartbeat(45 * time.Second)       // Longer heartbeat interval for internet
	worker.SetReconnectInterval(10 * time.Second) // Longer initial reconnect delay

	ws.mutex.Lock()
	ws.workers[serviceName] = worker
	ws.stats.ServiceStats[serviceName] = &ServiceWorkerStats{
		ServiceName:    serviceName,
		WorkerIdentity: workerIdentity,
	}
	ws.stats.RegisteredServices++
	ws.mutex.Unlock()

	ws.logger.Info().
		Str("service", serviceName).
		Str("worker_identity", workerIdentity).
		Str("transport", transport).
		Str("endpoint", ws.config.Gateway.Endpoint).
		Msg("Hub worker registered")

	return nil
}

// GetServiceStats returns statistics for all services
func (ws *WorkerService) GetServiceStats() *WorkerServiceStats {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()

	// Update active workers count
	activeWorkers := 0
	for serviceName, worker := range ws.workers {
		isConnected := worker.IsConnected()
		if serviceStats, exists := ws.stats.ServiceStats[serviceName]; exists {
			serviceStats.IsConnected = isConnected
		}
		if isConnected {
			activeWorkers++
		}
	}
	
	stats := *ws.stats
	stats.ActiveWorkers = activeWorkers
	
	// Deep copy service stats
	stats.ServiceStats = make(map[string]*ServiceWorkerStats)
	for name, serviceStats := range ws.stats.ServiceStats {
		statsCopy := *serviceStats
		stats.ServiceStats[name] = &statsCopy
	}

	return &stats
}

// GetWorkerForService returns the worker for a specific service
func (ws *WorkerService) GetWorkerForService(serviceName string) (*hermes.HermesWorker, bool) {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	worker, exists := ws.workers[serviceName]
	return worker, exists
}

// IsServiceActive returns whether a service is active
func (ws *WorkerService) IsServiceActive(serviceName string) bool {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	
	if worker, exists := ws.workers[serviceName]; exists {
		return worker.IsConnected()
	}
	return false
}

// IsConnected returns whether any workers are connected to the gateway
func (ws *WorkerService) IsConnected() bool {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	
	for _, worker := range ws.workers {
		if worker.IsConnected() {
			return true
		}
	}
	return false
}

// IsGatewayReachable returns whether the gateway is actually reachable (better health check)
func (ws *WorkerService) IsGatewayReachable() bool {
	ws.mutex.RLock()
	defer ws.mutex.RUnlock()
	
	// Check if any workers are connected AND recently communicated with gateway
	for _, worker := range ws.workers {
		if worker.IsConnected() {
			// Check if worker has recent heartbeat activity
			stats := worker.GetStats()
			timeSinceLastHeartbeat := time.Since(stats.LastHeartbeatReceived)
			
			// Consider gateway reachable if heartbeat was recent (within 2 minutes)
			if timeSinceLastHeartbeat < 2*time.Minute {
				return true
			}
		}
	}
	return false
}

// monitorWorkers monitors worker health and statistics
func (ws *WorkerService) monitorWorkers() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	ws.logger.Info().Msg("Starting worker monitoring")

	for {
		select {
		case <-ticker.C:
			ws.updateWorkerStats()
		case <-ws.ctx.Done():
			ws.logger.Info().Msg("Worker monitoring stopping")
			return
		}
	}
}

// updateWorkerStats updates statistics for all workers
func (ws *WorkerService) updateWorkerStats() {
	ws.mutex.Lock()
	defer ws.mutex.Unlock()

	for serviceName, worker := range ws.workers {
		if serviceStats, exists := ws.stats.ServiceStats[serviceName]; exists {
			workerStats := worker.GetStats()
			wasConnected := serviceStats.IsConnected
			isConnected := worker.IsConnected()
			
			serviceStats.RequestsHandled = workerStats.RequestsHandled
			serviceStats.RequestsFailed = workerStats.RequestsFailed
			serviceStats.LastRequest = workerStats.LastRequest
			serviceStats.IsConnected = isConnected
			
			// Detect reconnection after disconnection 
			if !wasConnected && isConnected {
				ws.logger.Info().
					Str("service", serviceName).
					Msg("Worker reconnected - gateway will auto-detect")
			}
		}
	}
}

// DeviceServiceHandler methods removed - using single HubServiceHandler

// HubServiceHandler methods

// Handle implements the hermes.RequestHandler interface for hub service
func (hsh *HubServiceHandler) Handle(request []byte) ([]byte, error) {
	startTime := time.Now()
	
	hsh.mutex.Lock()
	hsh.stats.RequestsProcessed++
	hsh.stats.LastRequest = startTime
	hsh.mutex.Unlock()

	hsh.logger.Debug().
		Int("request_size", len(request)).
		Msg("Processing hub service request")

	// Parse service request
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(request, &serviceReq); err != nil {
		hsh.recordError()
		return nil, fmt.Errorf("failed to parse service request: %w", err)
	}

	// Validate request
	if err := hermes.ValidateMessage(&serviceReq); err != nil {
		hsh.recordError()
		return nil, fmt.Errorf("invalid service request: %w", err)
	}

	// Process based on action
	var response *hermes.ServiceResponse
	var err error

	switch serviceReq.Action {
	case "execute":
		response, err = hsh.handleExecuteAction(&serviceReq)
	case "list":
		response, err = hsh.handleListAction(&serviceReq)
	case "status":
		response, err = hsh.handleStatusAction(&serviceReq)
	case "info":
		response, err = hsh.handleInfoAction(&serviceReq)
	default:
		hsh.recordError()
		return nil, fmt.Errorf("unknown action: %s", serviceReq.Action)
	}

	if err != nil {
		hsh.recordError()
		// Create error response with nonce
		response = hermes.CreateServiceResponseWithNonce(
			serviceReq.MessageID,
			serviceReq.Service,
			serviceReq.Nonce,
			false,
			nil,
			err,
		)
	}

	// Record latency
	latency := time.Since(startTime)
	hsh.recordLatency(latency)

	// Serialize response
	responseBytes, err := hermes.SerializeServiceResponse(response)
	if err != nil {
		hsh.recordError()
		return nil, fmt.Errorf("failed to serialize response: %w", err)
	}

	hsh.logger.Debug().
		Str("action", serviceReq.Action).
		Str("message_id", serviceReq.MessageID).
		Bool("success", response.Success).
		Dur("latency", latency).
		Msg("Hub service request processed")

	return responseBytes, nil
}

// handleExecuteAction handles device command execution through hub
func (hsh *HubServiceHandler) handleExecuteAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	// Parse device command from payload
	var deviceCmd struct {
		DeviceID string          `json:"device_id"`
		Action   json.RawMessage `json:"action"`
	}
	
	if err := json.Unmarshal(req.Payload, &deviceCmd); err != nil {
		return nil, fmt.Errorf("failed to parse device command: %w", err)
	}

	if deviceCmd.DeviceID == "" {
		return nil, fmt.Errorf("device_id is required")
	}

	// Execute device action with nonce support
	var response interface{}
	var err error
	
	if req.Nonce != "" {
		// Use nonce-based deduplication
		deviceResponse, deviceErr := hsh.deviceMgr.ProcessDeviceActionWithNonce(
			deviceCmd.DeviceID,
			req.Nonce,
			deviceCmd.Action,
		)
		response = deviceResponse
		err = deviceErr
	} else {
		// Standard processing without nonce
		deviceResponse, deviceErr := hsh.deviceMgr.ProcessDeviceAction(
			deviceCmd.DeviceID,
			deviceCmd.Action,
		)
		response = deviceResponse
		err = deviceErr
	}

	return hermes.CreateServiceResponseWithNonce(
		req.MessageID,
		req.Service,
		req.Nonce,
		err == nil,
		response,
		err,
	), nil
}

// handleListAction handles device listing requests
func (hsh *HubServiceHandler) handleListAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	hsh.logger.Info().
		Str("hub_id", hsh.config.Hub.ID).
		Str("message_id", req.MessageID).
		Msg("Hub received device list request")
	
	// Get all devices managed by this hub
	devices := make([]interface{}, 0)
	
	for _, deviceConfig := range hsh.config.Devices {
		hsh.logger.Debug().
			Str("device_id", deviceConfig.ID).
			Str("device_type", deviceConfig.Type).
			Str("device_address", deviceConfig.Address).
			Msg("Processing device from config")
		
		// Create device data from static config only - don't call device network operations
		// Device list should work regardless of device online/offline status
		completeDeviceInfo := map[string]interface{}{
			"id":           deviceConfig.ID,           // From config
			"name":         deviceConfig.Model,       // Use model as name
			"type":         deviceConfig.Type,        // From config
			"model":        deviceConfig.Model,       // From config (use model as device model)
			"address":      deviceConfig.Address,     // From config
			"status":       "unknown",                // Status unknown without network check
			"capabilities": deviceConfig.Capabilities, // From config
		}
		
		hsh.logger.Info().
			Str("device_id", deviceConfig.ID).
			Str("device_name", deviceConfig.Model).
			Str("device_type", deviceConfig.Type).
			Str("device_model", deviceConfig.Model).
			Str("device_address", deviceConfig.Address).
			Interface("capabilities", deviceConfig.Capabilities).
			Msg("Hub sending device data")
		
		devices = append(devices, completeDeviceInfo)
	}

	responseData := map[string]interface{}{
		"devices": devices,
		"hub_id":  hsh.config.Hub.ID,
		"count":   len(devices),
	}
	
	hsh.logger.Info().
		Str("hub_id", hsh.config.Hub.ID).
		Int("device_count", len(devices)).
		Interface("response_data", responseData).
		Msg("Hub sending device list response")

	hsh.logger.Info().
		Str("request_message_id", req.MessageID).
		Str("request_service", req.Service).
		Msg("Hub creating response with message ID from request")

	return hermes.CreateServiceResponseWithNonce(
		req.MessageID,
		req.Service,
		req.Nonce,
		true,
		responseData,
		nil,
	), nil
}

// handleStatusAction handles hub status requests
func (hsh *HubServiceHandler) handleStatusAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	hsh.mutex.RLock()
	stats := *hsh.stats
	hsh.mutex.RUnlock()

	// Count total devices managed by hub
	deviceCount := len(hsh.config.Devices)

	statusData := map[string]interface{}{
		"hub_id":             hsh.config.Hub.ID,
		"device_count":       deviceCount,
		"requests_processed": stats.RequestsProcessed,
		"requests_failed":    stats.RequestsFailed,
		"last_request":       stats.LastRequest,
		"average_latency_ms": stats.AverageLatency,
		"error_rate":         stats.ErrorRate,
		"status":             "healthy",
	}

	return hermes.CreateServiceResponseWithNonce(
		req.MessageID,
		req.Service,
		req.Nonce,
		true,
		statusData,
		nil,
	), nil
}

// handleInfoAction handles hub information requests
func (hsh *HubServiceHandler) handleInfoAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	// Collect capabilities from all devices managed by hub
	capabilities := make(map[string]bool)
	devices := make([]string, 0)
	deviceTypes := make(map[string]bool)
	
	for _, deviceConfig := range hsh.config.Devices {
		devices = append(devices, deviceConfig.ID)
		deviceTypes[deviceConfig.Type] = true
		for _, cap := range deviceConfig.Capabilities {
			capabilities[cap] = true
		}
	}

	// Convert maps to slices
	capSlice := make([]string, 0, len(capabilities))
	for cap := range capabilities {
		capSlice = append(capSlice, cap)
	}
	
	typeSlice := make([]string, 0, len(deviceTypes))
	for deviceType := range deviceTypes {
		typeSlice = append(typeSlice, deviceType)
	}

	infoData := map[string]interface{}{
		"service_name":   "hub.control",
		"hub_id":         hsh.config.Hub.ID,
		"description":    fmt.Sprintf("Hub control service for %s", hsh.config.Hub.ID),
		"capabilities":   capSlice,
		"device_ids":     devices,
		"device_types":   typeSlice,
		"device_count":   len(devices),
		"version":        "1.0.0",
	}

	return hermes.CreateServiceResponseWithNonce(
		req.MessageID,
		req.Service,
		req.Nonce,
		true,
		infoData,
		nil,
	), nil
}

// recordError records a failed request
func (hsh *HubServiceHandler) recordError() {
	hsh.mutex.Lock()
	defer hsh.mutex.Unlock()
	
	hsh.stats.RequestsFailed++
	
	// Calculate error rate
	if hsh.stats.RequestsProcessed > 0 {
		hsh.stats.ErrorRate = float64(hsh.stats.RequestsFailed) / float64(hsh.stats.RequestsProcessed)
	}
}

// recordLatency records request latency for statistics
func (hsh *HubServiceHandler) recordLatency(latency time.Duration) {
	hsh.mutex.Lock()
	defer hsh.mutex.Unlock()
	
	// Simple moving average calculation
	latencyMs := float64(latency.Nanoseconds()) / 1e6
	
	if hsh.stats.AverageLatency == 0 {
		hsh.stats.AverageLatency = latencyMs
	} else {
		// Exponential moving average with alpha = 0.1
		hsh.stats.AverageLatency = 0.9*hsh.stats.AverageLatency + 0.1*latencyMs
	}
}

// GetStats returns handler statistics
func (hsh *HubServiceHandler) GetStats() *ServiceHandlerStats {
	hsh.mutex.RLock()
	defer hsh.mutex.RUnlock()
	
	stats := *hsh.stats
	return &stats
}