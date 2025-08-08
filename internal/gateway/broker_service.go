package gateway

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

// BrokerService integrates Hermes broker with gateway functionality
type BrokerService struct {
	broker       *hermes.Broker
	registry     *ServiceRegistry
	database     *Database
	keys         *GatewayKeys
	logger       zerolog.Logger
	ctx          context.Context
	cancel       context.CancelFunc
	hubHandlers  map[string]*HubServiceHandler
	mutex        sync.RWMutex
}

// ServiceRegistry manages device services and their providers
type ServiceRegistry struct {
	services map[string]*DeviceService
	mutex    sync.RWMutex
}

// DeviceService represents a type of device service
type DeviceService struct {
	Name         string              `json:"name"`
	DeviceType   string              `json:"device_type"`
	Description  string              `json:"description"`
	Capabilities []string            `json:"capabilities"`
	Providers    []*ServiceProvider  `json:"providers"`
	LastSeen     time.Time           `json:"last_seen"`
	RequestCount int                 `json:"request_count"`
}

// ServiceProvider represents a hub providing a service
type ServiceProvider struct {
	HubID       string                 `json:"hub_id"`
	Identity    string                 `json:"identity"`
	Address     string                 `json:"address,omitempty"`
	Health      ServiceHealth          `json:"health"`
	LastSeen    time.Time              `json:"last_seen"`
	Requests    int                    `json:"requests"`
	Devices     []ServiceDeviceInfo    `json:"devices"`
}

// ServiceHealth represents the health status of a service provider
type ServiceHealth struct {
	Status      string    `json:"status"`
	LastCheck   time.Time `json:"last_check"`
	Latency     float64   `json:"latency_ms"`
	ErrorRate   float64   `json:"error_rate"`
	Uptime      float64   `json:"uptime_percent"`
}

// ServiceDeviceInfo represents device information for a service
type ServiceDeviceInfo struct {
	DeviceID     string   `json:"device_id"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Address      string   `json:"address"`
	Capabilities []string `json:"capabilities"`
	Status       string   `json:"status"`
}

// HubServiceHandler handles hub-specific service operations
type HubServiceHandler struct {
	hubID    string
	database *Database
	registry *ServiceRegistry
	logger   zerolog.Logger
}

// NewBrokerService creates a new broker service
func NewBrokerService(address string, keys *GatewayKeys, database *Database) *BrokerService {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &BrokerService{
		broker:      hermes.NewBroker(address),
		registry:    NewServiceRegistry(),
		database:    database,
		keys:        keys,
		logger:      logger.New(),
		ctx:         ctx,
		cancel:      cancel,
		hubHandlers: make(map[string]*HubServiceHandler),
	}
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		services: make(map[string]*DeviceService),
	}
}

// Start starts the broker service
func (bs *BrokerService) Start() error {
	bs.logger.Info().Msg("Starting Gateway Broker Service")

	// Start Hermes broker
	if err := bs.broker.Start(); err != nil {
		return fmt.Errorf("failed to start Hermes broker: %w", err)
	}

	// Hub services will announce themselves when they connect

	// Start service monitoring
	go bs.monitorServices()

	bs.logger.Info().Msg("Gateway Broker Service started successfully")
	return nil
}

// Stop stops the broker service
func (bs *BrokerService) Stop() error {
	bs.logger.Info().Msg("Stopping Gateway Broker Service")

	bs.cancel()

	if bs.broker != nil {
		if err := bs.broker.Stop(); err != nil {
			bs.logger.Error().Err(err).Msg("Error stopping Hermes broker")
		}
	}

	bs.logger.Info().Msg("Gateway Broker Service stopped")
	return nil
}

// RegisterHub registers a hub and its services
func (bs *BrokerService) RegisterHub(hubID, publicKey, name, productKey string) error {
	bs.logger.Info().
		Str("hub_id", hubID).
		Str("name", name).
		Msg("Registering hub with broker service")

	// Register hub in database
	hub, err := bs.database.RegisterHub(hubID, publicKey, name, productKey)
	if err != nil {
		return fmt.Errorf("failed to register hub in database: %w", err)
	}

	// Create hub service handler
	handler := &HubServiceHandler{
		hubID:    hubID,
		database: bs.database,
		registry: bs.registry,
		logger:   bs.logger,
	}

	bs.mutex.Lock()
	bs.hubHandlers[hubID] = handler
	bs.mutex.Unlock()

	bs.logger.Info().
		Str("hub_id", hubID).
		Int("hub_db_id", hub.ID).
		Msg("Hub registered with broker service")

	return nil
}

// UnregisterHub unregisters a hub and removes its services
func (bs *BrokerService) UnregisterHub(hubID string) error {
	bs.logger.Info().
		Str("hub_id", hubID).
		Msg("Unregistering hub from broker service")

	// Remove from hub handlers
	bs.mutex.Lock()
	delete(bs.hubHandlers, hubID)
	bs.mutex.Unlock()

	// Remove hub's services from registry
	bs.registry.RemoveHubServices(hubID)

	// Update database status
	if err := bs.database.UpdateHubStatus(hubID, "offline"); err != nil {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub status in database")
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Msg("Hub unregistered from broker service")

	return nil
}

// RegisterDeviceService registers a device service from a hub
func (bs *BrokerService) RegisterDeviceService(hubID, deviceType string, devices []ServiceDeviceInfo) error {
	serviceName := fmt.Sprintf("device.%s", deviceType)
	
	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("service", serviceName).
		Int("device_count", len(devices)).
		Msg("Registering device service")

	// Get hub identity from workers (this would come from Hermes broker)
	workers := bs.broker.GetWorkers()
	var hubIdentity string
	for identity, worker := range workers {
		if worker.Service == serviceName {
			// Match by hub ID somehow - for now, use first matching service
			hubIdentity = identity
			break
		}
	}

	// Register service in registry
	bs.registry.RegisterService(&DeviceService{
		Name:         serviceName,
		DeviceType:   deviceType,
		Description:  fmt.Sprintf("%s device service", deviceType),
		Capabilities: extractCapabilities(devices),
		Providers: []*ServiceProvider{
			{
				HubID:    hubID,
				Identity: hubIdentity,
				Health: ServiceHealth{
					Status:    "healthy",
					LastCheck: time.Now(),
				},
				LastSeen: time.Now(),
				Devices:  devices,
			},
		},
		LastSeen: time.Now(),
	})

	bs.logger.Info().
		Str("hub_id", hubID).
		Str("service", serviceName).
		Msg("Device service registered")

	return nil
}

// SendDeviceCommand sends a command to a device via the appropriate service
func (bs *BrokerService) SendDeviceCommand(hubID, deviceID string, action json.RawMessage) ([]byte, error) {
	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("device_id", deviceID).
		Msg("Sending device command via broker service")

	// Get device information from database
	device, _, err := bs.database.FindDeviceByID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Create service request
	serviceName := fmt.Sprintf("device.%s", device.DeviceType)
	
	deviceRequest := hermes.ServiceRequest{
		MessageID: hermes.GenerateMessageID(),
		Service:   serviceName,
		Action:    "execute",
		Payload:   action,
	}

	requestBytes, err := json.Marshal(deviceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device request: %w", err)
	}

	// Create client for this request
	client := hermes.NewClient(bs.broker.GetAddress(), fmt.Sprintf("gateway_%d", time.Now().UnixNano()))
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("failed to start client: %w", err)
	}
	defer client.Stop()

	// Send request via Hermes
	response, err := client.RequestWithTimeout(serviceName, requestBytes, 30*time.Second)
	if err != nil {
		return nil, fmt.Errorf("failed to send device command: %w", err)
	}

	// Parse service response
	var serviceResp hermes.ServiceResponse
	if err := json.Unmarshal(response, &serviceResp); err != nil {
		return nil, fmt.Errorf("failed to parse service response: %w", err)
	}

	if !serviceResp.Success {
		return nil, fmt.Errorf("service error: %s", serviceResp.Error)
	}

	// Return the data as JSON
	dataBytes, err := json.Marshal(serviceResp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response data: %w", err)
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Str("device_id", deviceID).
		Str("message_id", serviceResp.MessageID).
		Msg("Device command executed successfully")

	return dataBytes, nil
}

// GetServiceStats returns statistics about registered services
func (bs *BrokerService) GetServiceStats() map[string]interface{} {
	brokerStats := bs.broker.GetStats()
	services := bs.broker.GetServices()
	workers := bs.broker.GetWorkers()
	registryStats := bs.registry.GetStats()

	return map[string]interface{}{
		"broker": map[string]interface{}{
			"services":      brokerStats.Services,
			"workers":       brokerStats.Workers,
			"requests":      brokerStats.Requests,
			"responses":     brokerStats.Responses,
			"start_time":    brokerStats.StartTime,
			"last_request":  brokerStats.LastRequest,
		},
		"services": services,
		"workers":  workers,
		"registry": registryStats,
	}
}

// monitorServices monitors service health and cleanup
func (bs *BrokerService) monitorServices() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	bs.logger.Info().Msg("Starting service monitoring")

	for {
		select {
		case <-ticker.C:
			bs.checkServiceHealth()
			bs.syncWorkerRegistrations()
			bs.cleanupStaleServices()
		case <-bs.ctx.Done():
			bs.logger.Info().Msg("Service monitoring stopping")
			return
		}
	}
}

// checkServiceHealth checks the health of all services
func (bs *BrokerService) checkServiceHealth() {
	services := bs.broker.GetServices()
	workers := bs.broker.GetWorkers()

	for serviceName, serviceInfo := range services {
		// Update service health based on worker status
		activeWorkers := 0
		var activeHubIDs []string

		for _, workerID := range serviceInfo.Workers {
			if worker, exists := workers[workerID]; exists {
				if time.Since(worker.LastPing) < 60*time.Second {
					activeWorkers++
					// For hub.control service, workerID is the hub ID
					if serviceName == "hub.control" {
						activeHubIDs = append(activeHubIDs, workerID)
					}
				}
			}
		}

		// Update registry with health information
		bs.registry.UpdateServiceHealth(serviceName, activeWorkers > 0)

		// Update hub status in database for hub.control services
		if serviceName == "hub.control" {
			for _, hubID := range activeHubIDs {
				if err := bs.database.UpdateHubStatus(hubID, "online"); err != nil {
					bs.logger.Warn().
						Str("hub_id", hubID).
						Err(err).
						Msg("Failed to update hub status in database")
				}
			}
		}
	}
}

// syncWorkerRegistrations tracks available services for routing
func (bs *BrokerService) syncWorkerRegistrations() {
	services := bs.broker.GetServices()
	workers := bs.broker.GetWorkers()

	// Track which hubs are currently available
	availableHubs := make(map[string]bool)

	// Update service availability for hub.control services
	if serviceInfo, exists := services["hub.control"]; exists {
		for _, workerID := range serviceInfo.Workers {
			if worker, workerExists := workers[workerID]; workerExists {
				// Check if worker is active (recent heartbeat)
				if time.Since(worker.LastPing) < 90*time.Second {
					availableHubs[workerID] = true
					
					// Check if this is a newly available hub
					bs.mutex.RLock()
					_, wasKnown := bs.hubHandlers[workerID]
					bs.mutex.RUnlock()
					
					if !wasKnown {
						// New hub detected - process service announcement
						bs.handleServiceAnnouncement(workerID)
					}
				}
			}
		}
	}

	// Log available hubs for routing
	if len(availableHubs) > 0 {
		bs.logger.Debug().
			Int("available_hubs", len(availableHubs)).
			Msg("Hub services available for routing")
	}
}

// handleServiceAnnouncement processes service ready announcements from hubs
func (bs *BrokerService) handleServiceAnnouncement(hubID string) {
	bs.logger.Info().
		Str("hub_id", hubID).
		Msg("Hub service announced - available for routing")

	// Create a simple hub handler for routing (no database dependency)
	handler := &HubServiceHandler{
		hubID:    hubID,
		database: bs.database,
		registry: bs.registry,
		logger:   bs.logger,
	}

	bs.mutex.Lock()
	bs.hubHandlers[hubID] = handler
	bs.mutex.Unlock()

	// Update hub status in database (now uses select-insert-select pattern for race condition tolerance)
	if err := bs.database.UpdateHubStatus(hubID, "online"); err != nil {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub status in database")
	} else {
		bs.logger.Debug().
			Str("hub_id", hubID).
			Msg("Hub status updated to online")
	}
}

// cleanupStaleServices removes services that haven't been seen recently
func (bs *BrokerService) cleanupStaleServices() {
	cutoff := time.Now().Add(-5 * time.Minute) // 5 minutes
	bs.registry.RemoveStaleServices(cutoff)
}


// extractCapabilities extracts unique capabilities from devices
func extractCapabilities(devices []ServiceDeviceInfo) []string {
	capabilitySet := make(map[string]bool)
	for _, device := range devices {
		for _, cap := range device.Capabilities {
			capabilitySet[cap] = true
		}
	}
	
	capabilities := make([]string, 0, len(capabilitySet))
	for cap := range capabilitySet {
		capabilities = append(capabilities, cap)
	}
	
	return capabilities
}

// ServiceRegistry methods

// RegisterService registers a service in the registry
func (sr *ServiceRegistry) RegisterService(service *DeviceService) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	sr.services[service.Name] = service
}

// UnregisterService removes a service from the registry
func (sr *ServiceRegistry) UnregisterService(serviceName string) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	delete(sr.services, serviceName)
}

// GetService returns a service by name
func (sr *ServiceRegistry) GetService(serviceName string) (*DeviceService, bool) {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	service, exists := sr.services[serviceName]
	return service, exists
}

// ListServices returns all registered services
func (sr *ServiceRegistry) ListServices() map[string]*DeviceService {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	
	services := make(map[string]*DeviceService)
	for name, service := range sr.services {
		serviceCopy := *service
		services[name] = &serviceCopy
	}
	return services
}

// RemoveHubServices removes all services provided by a hub
func (sr *ServiceRegistry) RemoveHubServices(hubID string) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	for serviceName, service := range sr.services {
		// Remove hub from providers
		newProviders := make([]*ServiceProvider, 0, len(service.Providers))
		for _, provider := range service.Providers {
			if provider.HubID != hubID {
				newProviders = append(newProviders, provider)
			}
		}
		service.Providers = newProviders
		
		// Remove service if no providers left
		if len(service.Providers) == 0 {
			delete(sr.services, serviceName)
		}
	}
}

// UpdateServiceHealth updates the health status of a service
func (sr *ServiceRegistry) UpdateServiceHealth(serviceName string, healthy bool) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	if service, exists := sr.services[serviceName]; exists {
		service.LastSeen = time.Now()
		for _, provider := range service.Providers {
			provider.Health.LastCheck = time.Now()
			if healthy {
				provider.Health.Status = "healthy"
			} else {
				provider.Health.Status = "unhealthy"
			}
		}
	}
}

// RemoveStaleServices removes services that haven't been seen since the cutoff time
func (sr *ServiceRegistry) RemoveStaleServices(cutoff time.Time) {
	sr.mutex.Lock()
	defer sr.mutex.Unlock()
	
	for serviceName, service := range sr.services {
		if service.LastSeen.Before(cutoff) {
			delete(sr.services, serviceName)
		}
	}
}

// GetStats returns registry statistics
func (sr *ServiceRegistry) GetStats() map[string]interface{} {
	sr.mutex.RLock()
	defer sr.mutex.RUnlock()
	
	totalProviders := 0
	totalDevices := 0
	servicesByType := make(map[string]int)
	
	for _, service := range sr.services {
		totalProviders += len(service.Providers)
		servicesByType[service.DeviceType]++
		
		for _, provider := range service.Providers {
			totalDevices += len(provider.Devices)
		}
	}
	
	return map[string]interface{}{
		"total_services":    len(sr.services),
		"total_providers":   totalProviders,
		"total_devices":     totalDevices,
		"services_by_type":  servicesByType,
	}
}