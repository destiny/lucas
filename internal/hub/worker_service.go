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

// DeviceServiceHandler handles device-specific service requests
type DeviceServiceHandler struct {
	deviceType string
	deviceMgr  *DeviceManager
	config     *Config
	logger     zerolog.Logger
	stats      *ServiceHandlerStats
	mutex      sync.RWMutex
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

	// Register device services based on configuration
	if err := ws.registerDeviceServices(); err != nil {
		return fmt.Errorf("failed to register device services: %w", err)
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

// registerDeviceServices registers device services as Hermes workers
func (ws *WorkerService) registerDeviceServices() error {
	ws.logger.Info().
		Int("device_count", len(ws.config.Devices)).
		Msg("Registering device services")

	// Group devices by type
	devicesByType := make(map[string][]DeviceConfig)
	for _, device := range ws.config.Devices {
		devicesByType[device.Type] = append(devicesByType[device.Type], device)
	}

	// Create a worker for each device type
	for deviceType, devices := range devicesByType {
		serviceName := fmt.Sprintf("device.%s", deviceType)
		
		// Create service handler
		handler := &DeviceServiceHandler{
			deviceType: deviceType,
			deviceMgr:  ws.deviceMgr,
			config:     ws.config,
			logger:     ws.logger,
			stats: &ServiceHandlerStats{},
		}

		// Generate unique worker identity
		workerIdentity := fmt.Sprintf("hub_%s_%s_%d", 
			ws.config.Hub.ID, deviceType, time.Now().UnixNano())

		// Create Hermes worker
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
			Str("device_type", deviceType).
			Int("device_count", len(devices)).
			Str("worker_identity", workerIdentity).
			Msg("Device service worker registered")
	}

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
			serviceStats.RequestsHandled = workerStats.RequestsHandled
			serviceStats.RequestsFailed = workerStats.RequestsFailed
			serviceStats.LastRequest = workerStats.LastRequest
			serviceStats.IsConnected = worker.IsConnected()
		}
	}
}

// DeviceServiceHandler methods

// NewDeviceServiceHandler creates a new device service handler
func NewDeviceServiceHandler(deviceType string, deviceMgr *DeviceManager, config *Config) *DeviceServiceHandler {
	return &DeviceServiceHandler{
		deviceType: deviceType,
		deviceMgr:  deviceMgr,
		config:     config,
		logger:     logger.New(),
		stats:      &ServiceHandlerStats{},
	}
}

// Handle implements the hermes.RequestHandler interface
func (dsh *DeviceServiceHandler) Handle(request []byte) ([]byte, error) {
	startTime := time.Now()
	
	dsh.mutex.Lock()
	dsh.stats.RequestsProcessed++
	dsh.stats.LastRequest = startTime
	dsh.mutex.Unlock()

	dsh.logger.Debug().
		Str("device_type", dsh.deviceType).
		Int("request_size", len(request)).
		Msg("Processing device service request")

	// Parse service request
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(request, &serviceReq); err != nil {
		dsh.recordError()
		return nil, fmt.Errorf("failed to parse service request: %w", err)
	}

	// Validate request
	if err := hermes.ValidateMessage(&serviceReq); err != nil {
		dsh.recordError()
		return nil, fmt.Errorf("invalid service request: %w", err)
	}

	// Process based on action
	var response *hermes.ServiceResponse
	var err error

	switch serviceReq.Action {
	case "execute":
		response, err = dsh.handleExecuteAction(&serviceReq)
	case "list":
		response, err = dsh.handleListAction(&serviceReq)
	case "status":
		response, err = dsh.handleStatusAction(&serviceReq)
	case "info":
		response, err = dsh.handleInfoAction(&serviceReq)
	default:
		dsh.recordError()
		return nil, fmt.Errorf("unknown action: %s", serviceReq.Action)
	}

	if err != nil {
		dsh.recordError()
		// Create error response
		response = hermes.CreateServiceResponse(
			serviceReq.MessageID,
			serviceReq.Service,
			false,
			nil,
			err,
		)
	}

	// Set nonce if present
	if serviceReq.Nonce != "" {
		response.Nonce = serviceReq.Nonce
	}

	// Record latency
	latency := time.Since(startTime)
	dsh.recordLatency(latency)

	// Serialize response
	responseBytes, err := hermes.SerializeServiceResponse(response)
	if err != nil {
		dsh.recordError()
		return nil, fmt.Errorf("failed to serialize response: %w", err)
	}

	dsh.logger.Debug().
		Str("device_type", dsh.deviceType).
		Str("action", serviceReq.Action).
		Str("message_id", serviceReq.MessageID).
		Bool("success", response.Success).
		Dur("latency", latency).
		Msg("Device service request processed")

	return responseBytes, nil
}

// handleExecuteAction handles device command execution
func (dsh *DeviceServiceHandler) handleExecuteAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
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
		deviceResponse, deviceErr := dsh.deviceMgr.ProcessDeviceActionWithNonce(
			deviceCmd.DeviceID,
			req.Nonce,
			deviceCmd.Action,
		)
		response = deviceResponse
		err = deviceErr
	} else {
		// Standard processing without nonce
		deviceResponse, deviceErr := dsh.deviceMgr.ProcessDeviceAction(
			deviceCmd.DeviceID,
			deviceCmd.Action,
		)
		response = deviceResponse
		err = deviceErr
	}

	return hermes.CreateServiceResponse(
		req.MessageID,
		req.Service,
		err == nil,
		response,
		err,
	), nil
}

// handleListAction handles device listing requests
func (dsh *DeviceServiceHandler) handleListAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	// Get all devices of this type
	devices := make([]interface{}, 0)
	
	for _, deviceConfig := range dsh.config.Devices {
		if deviceConfig.Type == dsh.deviceType {
			deviceInfo, err := dsh.deviceMgr.GetDeviceInfo(deviceConfig.ID)
			if err != nil {
				dsh.logger.Warn().
					Str("device_id", deviceConfig.ID).
					Err(err).
					Msg("Failed to get device info")
				continue
			}
			devices = append(devices, deviceInfo)
		}
	}

	return hermes.CreateServiceResponse(
		req.MessageID,
		req.Service,
		true,
		map[string]interface{}{
			"devices":     devices,
			"device_type": dsh.deviceType,
			"count":       len(devices),
		},
		nil,
	), nil
}

// handleStatusAction handles service status requests
func (dsh *DeviceServiceHandler) handleStatusAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	dsh.mutex.RLock()
	stats := *dsh.stats
	dsh.mutex.RUnlock()

	// Count devices of this type
	deviceCount := 0
	for _, deviceConfig := range dsh.config.Devices {
		if deviceConfig.Type == dsh.deviceType {
			deviceCount++
		}
	}

	statusData := map[string]interface{}{
		"device_type":        dsh.deviceType,
		"device_count":       deviceCount,
		"requests_processed": stats.RequestsProcessed,
		"requests_failed":    stats.RequestsFailed,
		"last_request":       stats.LastRequest,
		"average_latency_ms": stats.AverageLatency,
		"error_rate":         stats.ErrorRate,
		"status":             "healthy",
	}

	return hermes.CreateServiceResponse(
		req.MessageID,
		req.Service,
		true,
		statusData,
		nil,
	), nil
}

// handleInfoAction handles service information requests
func (dsh *DeviceServiceHandler) handleInfoAction(req *hermes.ServiceRequest) (*hermes.ServiceResponse, error) {
	// Collect capabilities from devices of this type
	capabilities := make(map[string]bool)
	devices := make([]string, 0)
	
	for _, deviceConfig := range dsh.config.Devices {
		if deviceConfig.Type == dsh.deviceType {
			devices = append(devices, deviceConfig.ID)
			for _, cap := range deviceConfig.Capabilities {
				capabilities[cap] = true
			}
		}
	}

	// Convert capabilities map to slice
	capSlice := make([]string, 0, len(capabilities))
	for cap := range capabilities {
		capSlice = append(capSlice, cap)
	}

	infoData := map[string]interface{}{
		"service_name":  fmt.Sprintf("device.%s", dsh.deviceType),
		"device_type":   dsh.deviceType,
		"description":   fmt.Sprintf("Service for %s devices", dsh.deviceType),
		"capabilities":  capSlice,
		"device_ids":    devices,
		"device_count":  len(devices),
		"version":       "1.0.0",
	}

	return hermes.CreateServiceResponse(
		req.MessageID,
		req.Service,
		true,
		infoData,
		nil,
	), nil
}

// recordError records a failed request
func (dsh *DeviceServiceHandler) recordError() {
	dsh.mutex.Lock()
	defer dsh.mutex.Unlock()
	
	dsh.stats.RequestsFailed++
	
	// Calculate error rate
	if dsh.stats.RequestsProcessed > 0 {
		dsh.stats.ErrorRate = float64(dsh.stats.RequestsFailed) / float64(dsh.stats.RequestsProcessed)
	}
}

// recordLatency records request latency for statistics
func (dsh *DeviceServiceHandler) recordLatency(latency time.Duration) {
	dsh.mutex.Lock()
	defer dsh.mutex.Unlock()
	
	// Simple moving average calculation
	// In a production system, you might want to use a more sophisticated approach
	latencyMs := float64(latency.Nanoseconds()) / 1e6
	
	if dsh.stats.AverageLatency == 0 {
		dsh.stats.AverageLatency = latencyMs
	} else {
		// Exponential moving average with alpha = 0.1
		dsh.stats.AverageLatency = 0.9*dsh.stats.AverageLatency + 0.1*latencyMs
	}
}

// GetStats returns handler statistics
func (dsh *DeviceServiceHandler) GetStats() *ServiceHandlerStats {
	dsh.mutex.RLock()
	defer dsh.mutex.RUnlock()
	
	stats := *dsh.stats
	return &stats
}