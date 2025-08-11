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

package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	// Single persistent client for all gateway-hub communication
	client       *hermes.HermesClient
	clientMutex  sync.Mutex
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
	
	bs := &BrokerService{
		broker:      hermes.NewBroker(address),
		registry:    NewServiceRegistry(),
		database:    database,
		keys:        keys,
		logger:      logger.New(),
		ctx:         ctx,
		cancel:      cancel,
		hubHandlers: make(map[string]*HubServiceHandler),
	}
	
	// Initialize persistent client for all gateway-hub communication
	// Use standardized client ID from jargon specification
	clientAddress := bs.convertBrokerAddressToClient(address)
	bs.client = hermes.NewClient(clientAddress, "gateway_main")
	
	return bs
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

	// Start persistent client first
	bs.clientMutex.Lock()
	if err := bs.client.Start(); err != nil {
		bs.clientMutex.Unlock()
		return fmt.Errorf("failed to start persistent client: %w", err)
	}
	bs.clientMutex.Unlock()

	// Set broker service reference for immediate device list processing
	bs.broker.SetBrokerService(bs)

	// Start Hermes broker
	if err := bs.broker.Start(); err != nil {
		// Stop client if broker fails
		bs.clientMutex.Lock()
		bs.client.Stop()
		bs.clientMutex.Unlock()
		return fmt.Errorf("failed to start Hermes broker: %w", err)
	}

	// Hub services will announce themselves when they connect

	// Start service monitoring (simplified)
	go bs.monitorServices()

	bs.logger.Info().Msg("Gateway Broker Service started successfully")
	return nil
}

// Stop stops the broker service
func (bs *BrokerService) Stop() error {
	bs.logger.Info().Msg("Stopping Gateway Broker Service")

	bs.cancel()

	// Stop persistent client
	bs.clientMutex.Lock()
	if bs.client != nil {
		if err := bs.client.Stop(); err != nil {
			bs.logger.Error().Err(err).Msg("Error stopping persistent client")
		}
	}
	bs.clientMutex.Unlock()

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

	// Update hub status to offline
	if err := bs.database.UpdateHubStatus(hubID, "offline"); err != nil {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub status in database")
	}

	// Update all devices belonging to this hub to offline status
	if err := bs.updateHubDevicesStatus(hubID, "offline"); err != nil {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub devices status to offline")
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Msg("Hub and its devices marked as offline")

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

	// Verify device exists (but we don't need device details for routing)
	_, _, err := bs.database.FindDeviceByID(deviceID)
	if err != nil {
		return nil, fmt.Errorf("device not found: %w", err)
	}

	// Always route through hub.control (single hub worker)
	serviceName := "hub.control"
	
	// Create device command that hub will route internally
	deviceCommand := map[string]interface{}{
		"device_id": deviceID,
		"action":    json.RawMessage(action),
	}
	
	deviceCommandBytes, err := json.Marshal(deviceCommand)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device command: %w", err)
	}
	
	// Generate simple nonce for request deduplication
	nonce := hermes.GenerateNonce()
	
	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("device_id", deviceID).
		Str("nonce", nonce).
		Msg("Generated simple nonce for device command")
	
	messageID := hermes.GenerateMessageID()
	
	deviceRequest := hermes.ServiceRequest{
		MessageID: messageID,
		Service:   serviceName,
		Action:    "execute",
		Payload:   json.RawMessage(deviceCommandBytes),
		Nonce:     nonce,
	}
	
	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("device_id", deviceID).
		Str("message_id", messageID).
		Str("nonce", nonce).
		Str("service", serviceName).
		Msg("Sending service request to hub")

	requestBytes, err := json.Marshal(deviceRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device request: %w", err)
	}

	// Use fire-and-forget mode for remote control commands (now fixed in broker)
	// This provides immediate response to user and handles responses asynchronously
	bs.clientMutex.Lock()
	defer bs.clientMutex.Unlock()
	
	if bs.client == nil {
		return nil, fmt.Errorf("persistent client not initialized")
	}

	// Send as fire-and-forget request using nonce correlation
	err = bs.client.RequestFireAndForget(serviceName, requestBytes, nonce)
	if err != nil {
		bs.logger.Error().
			Str("hub_id", hubID).
			Str("device_id", deviceID).
			Str("nonce", nonce).
			Str("message_id", messageID).
			Err(err).
			Msg("[HUB_DEBUG] Device command failed to send")
		return nil, fmt.Errorf("failed to send device command: %w", err)
	}

	// Return success immediately - response will be handled asynchronously if it arrives
	successResponse := map[string]interface{}{
		"success":    true,
		"message":    "Command sent to device",
		"device_id":  deviceID,
		"nonce":      nonce,
		"message_id": messageID,
	}

	dataBytes, err := json.Marshal(successResponse)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal success response: %w", err)
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Str("device_id", deviceID).
		Str("nonce", nonce).
		Msg("[HUB_DEBUG] Device command sent successfully")

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

// monitorServices monitors service health and cleanup (simplified)
func (bs *BrokerService) monitorServices() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	bs.logger.Info().Msg("Starting service monitoring")

	for {
		select {
		case <-ticker.C:
			bs.checkServiceHealth()
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

// processHubWorkerRegistration handles hub worker registration and immediate device list request
func (bs *BrokerService) processHubWorkerRegistration(hubID, serviceName string) {
	bs.logger.Info().
		Str("hub_id", hubID).
		Str("service", serviceName).
		Msg("Processing hub worker registration")

	// Ensure hub exists in database first
	if err := bs.database.EnsureHubExists(hubID); err != nil {
		bs.logger.Error().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to ensure hub exists - skipping device registration")
		return
	}

	// Update hub status to online
	if err := bs.database.UpdateHubStatus(hubID, "online"); err != nil {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub status to online")
	}

	// Create hub handler for tracking
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
		Msg("Hub registered successfully - device list will be requested via broker")
}


// convertBrokerAddressToClient converts broker bind address to client connection address
func (bs *BrokerService) convertBrokerAddressToClient(brokerAddr string) string {
	// Convert bind addresses to client connection addresses
	if strings.Contains(brokerAddr, "tcp://*:") {
		// Replace * with localhost
		return strings.Replace(brokerAddr, "*", "localhost", 1)
	}
	if strings.Contains(brokerAddr, "tcp://0.0.0.0:") {
		// Replace 0.0.0.0 with localhost
		return strings.Replace(brokerAddr, "0.0.0.0", "localhost", 1)
	}
	
	// Return as-is for other addresses
	return brokerAddr
}

// getClientAddress converts broker bind address to client connection address (legacy method)
func (bs *BrokerService) getClientAddress() string {
	return bs.convertBrokerAddressToClient(bs.broker.GetAddress())
}

// ProcessDeviceListResponse processes the device list response and registers devices
func (bs *BrokerService) ProcessDeviceListResponse(hubID string, response []byte) {
	bs.logger.Info().
		Str("hub_id", hubID).
		Msg("Processing immediate device list response from hub")

	// Ensure hub is registered in gateway database
	bs.processHubWorkerRegistration(hubID, "hub.control")
		
	// Parse service response
	var serviceResp hermes.ServiceResponse
	if err := json.Unmarshal(response, &serviceResp); err != nil {
		bs.logger.Error().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to parse device list response")
		return
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Bool("success", serviceResp.Success).
		Str("message_id", serviceResp.MessageID).
		Interface("data", serviceResp.Data).
		Msg("[DEBUG] Gateway parsed service response")

	if !serviceResp.Success {
		bs.logger.Error().
			Str("hub_id", hubID).
			Str("error", serviceResp.Error).
			Msg("Hub returned error for device list request")
		return
	}

	// Check if this is actually a device action response, not a device list response
	// Device action responses have different data structure
	if dataMap, ok := serviceResp.Data.(map[string]interface{}); ok {
		// Check if this looks like a device action response (has "data" field but no "devices" field)
		if _, hasData := dataMap["data"]; hasData {
			if _, hasDevices := dataMap["devices"]; !hasDevices {
				bs.logger.Debug().
					Str("hub_id", hubID).
					Str("message_id", serviceResp.MessageID).
					Interface("response_data", dataMap).
					Msg("This appears to be a device action response, not a device list - ignoring in device list handler")
				return
			}
		}
	}

	// Get hub record from database
	hub, err := bs.database.GetHubByHubID(hubID)
	if err != nil {
		bs.logger.Error().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to get hub record for device registration")
		return
	}

	// Parse device list data
	dataMap, ok := serviceResp.Data.(map[string]interface{})
	if !ok {
		bs.logger.Error().
			Str("hub_id", hubID).
			Str("data_type", fmt.Sprintf("%T", serviceResp.Data)).
			Msg("Invalid device list data format")
		return
	}

	bs.logger.Debug().
		Str("hub_id", hubID).
		Interface("data_map", dataMap).
		Msg("[DEBUG] Gateway extracted data map")

	// Check if the data contains error information instead of device list
	if errorMsg, hasError := dataMap["error"]; hasError {
		bs.logger.Warn().
			Str("hub_id", hubID).
			Interface("error", errorMsg).
			Msg("Hub returned error data instead of device list - this is expected for offline devices")
		return
	}

	// Check if inner response indicates failure
	if innerSuccess, hasInnerSuccess := dataMap["success"]; hasInnerSuccess {
		if success, ok := innerSuccess.(bool); ok && !success {
			bs.logger.Warn().
				Str("hub_id", hubID).
				Interface("available_keys", getKeys(dataMap)).
				Msg("Hub returned unsuccessful response - this is expected for offline devices")
			return
		}
	}

	devicesData, ok := dataMap["devices"]
	if !ok {
		bs.logger.Error().
			Str("hub_id", hubID).
			Interface("available_keys", getKeys(dataMap)).
			Msg("No devices field in response")
		return
	}

	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("devices_type", fmt.Sprintf("%T", devicesData)).
		Msg("[DEBUG] Gateway found devices field")

	devicesSlice, ok := devicesData.([]interface{})
	if !ok {
		bs.logger.Error().
			Str("hub_id", hubID).
			Str("devices_type", fmt.Sprintf("%T", devicesData)).
			Msg("Devices field is not an array")
		return
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Int("devices_count", len(devicesSlice)).
		Msg("[DEBUG] Gateway found devices array")

	// Register each device
	deviceCount := 0
	for _, deviceData := range devicesSlice {
		deviceMap, ok := deviceData.(map[string]interface{})
		if !ok {
			bs.logger.Warn().
				Str("hub_id", hubID).
				Msg("Skipping invalid device data")
			continue
		}

		// Extract device information
		deviceID, _ := deviceMap["id"].(string)
		deviceType, _ := deviceMap["type"].(string)
		deviceName, _ := deviceMap["name"].(string)
		deviceModel, _ := deviceMap["model"].(string)
		deviceAddress, _ := deviceMap["address"].(string)
		deviceStatus, _ := deviceMap["status"].(string)

		if deviceID == "" || deviceType == "" {
			bs.logger.Warn().
				Str("hub_id", hubID).
				Msg("Skipping device with missing ID or type")
			continue
		}

		// Parse capabilities
		var capabilities []string
		if capData, exists := deviceMap["capabilities"]; exists {
			if capSlice, ok := capData.([]interface{}); ok {
				for _, cap := range capSlice {
					if capStr, ok := cap.(string); ok {
						capabilities = append(capabilities, capStr)
					}
				}
			}
		}

		bs.logger.Info().
			Str("hub_id", hubID).
			Str("device_id", deviceID).
			Str("device_type", deviceType).
			Str("device_name", deviceName).
			Str("device_model", deviceModel).
			Str("device_address", deviceAddress).
			Interface("capabilities", capabilities).
			Int("hub_db_id", hub.ID).
			Msg("[DEBUG] Gateway creating device in database")

		// Create device in database
		device, err := bs.database.CreateDevice(
			hub.ID,          // hub database ID
			deviceID,        // device ID from hub
			deviceType,      // device type
			deviceName,      // device name
			deviceModel,     // device model
			deviceAddress,   // device address
			capabilities,    // device capabilities
		)
		if err != nil {
			bs.logger.Error().
				Str("hub_id", hubID).
				Str("device_id", deviceID).
				Err(err).
				Msg("Failed to create device in database")
			continue
		}

		bs.logger.Info().
			Str("hub_id", hubID).
			Str("device_id", deviceID).
			Int("device_db_id", device.ID).
			Msg("[DEBUG] Gateway successfully created device in database")

		// Update device status - default to online since hub is connected
		finalStatus := "online"
		if deviceStatus != "" && deviceStatus != "unknown" {
			finalStatus = deviceStatus
		}
		
		if err := bs.database.UpdateDeviceStatus(deviceID, finalStatus); err != nil {
			bs.logger.Warn().
				Str("hub_id", hubID).
				Str("device_id", deviceID).
				Str("status", finalStatus).
				Err(err).
				Msg("Failed to update device status")
		} else {
			bs.logger.Debug().
				Str("hub_id", hubID).
				Str("device_id", deviceID).
				Str("status", finalStatus).
				Msg("Device status updated")
		}

		bs.logger.Info().
			Str("hub_id", hubID).
			Str("device_id", deviceID).
			Str("device_type", deviceType).
			Int("device_db_id", device.ID).
			Msg("Device registered successfully")

		deviceCount++
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Int("devices_registered", deviceCount).
		Int("devices_total", len(devicesSlice)).
		Msg("Device registration completed")
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

// getKeys returns the keys of a map as a slice for debugging
func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// updateHubDevicesStatus updates the status of all devices belonging to a hub
func (bs *BrokerService) updateHubDevicesStatus(hubID string, status string) error {
	bs.logger.Debug().
		Str("hub_id", hubID).
		Str("status", status).
		Msg("Updating all devices status for hub")

	// Get hub record from database
	hub, err := bs.database.GetHubByHubID(hubID)
	if err != nil {
		return fmt.Errorf("failed to get hub record: %w", err)
	}

	// Get all devices for this hub
	devices, err := bs.database.GetHubDevices(hub.ID)
	if err != nil {
		return fmt.Errorf("failed to get devices for hub: %w", err)
	}

	// Update status for each device
	updatedCount := 0
	for _, device := range devices {
		if err := bs.database.UpdateDeviceStatus(device.DeviceID, status); err != nil {
			bs.logger.Warn().
				Str("hub_id", hubID).
				Str("device_id", device.DeviceID).
				Str("status", status).
				Err(err).
				Msg("Failed to update device status")
		} else {
			updatedCount++
			bs.logger.Debug().
				Str("hub_id", hubID).
				Str("device_id", device.DeviceID).
				Str("status", status).
				Msg("Device status updated")
		}
	}

	bs.logger.Info().
		Str("hub_id", hubID).
		Str("status", status).
		Int("devices_updated", updatedCount).
		Int("devices_total", len(devices)).
		Msg("Hub devices status update completed")

	return nil
}