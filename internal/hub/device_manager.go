package hub

import (
	"fmt"
	"lucas/internal"
	"sync"
	"time"

	"lucas/internal/bravia"
	"lucas/internal/device"
	"lucas/internal/logger"

	"github.com/rs/zerolog"
)

// DeviceManager manages the lifecycle and access to devices
type DeviceManager struct {
	devices    map[string]device.Device
	config     *Config
	mutex      sync.RWMutex
	logger     zerolog.Logger
	nonceCache *NonceCache
}

// NewDeviceManager creates a new device manager
func NewDeviceManager(config *Config) *DeviceManager {
	return &DeviceManager{
		devices:    make(map[string]device.Device),
		config:     config,
		logger:     logger.New(),
		nonceCache: NewNonceCache(50, time.Hour), // 50 nonces per device, 1 hour expiration
	}
}

// Initialize loads and initializes all devices from configuration
func (dm *DeviceManager) Initialize(debug, testMode bool) error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.logger.Info().
		Int("device_count", len(dm.config.Devices)).
		Msg("Initializing devices")

	for _, deviceConfig := range dm.config.Devices {
		device, err := dm.createDevice(deviceConfig, debug, testMode)
		if err != nil {
			dm.logger.Error().
				Str("device_id", deviceConfig.ID).
				Err(err).
				Msg("Failed to create device")
			return fmt.Errorf("failed to create device %s: %w", deviceConfig.ID, err)
		}

		dm.devices[deviceConfig.ID] = device
		dm.logger.Info().
			Str("device_id", deviceConfig.ID).
			Str("device_type", deviceConfig.Type).
			Str("device_address", deviceConfig.Address).
			Msg("Device initialized successfully")
	}

	dm.logger.Info().
		Int("initialized_count", len(dm.devices)).
		Msg("All devices initialized")

	return nil
}

// createDevice creates a device instance based on its configuration
func (dm *DeviceManager) createDevice(config DeviceConfig, debug, testMode bool) (device.Device, error) {
	switch config.Type {
	case "bravia":
		if config.Credential == "" {
			return nil, fmt.Errorf("credential is required for bravia device")
		}
		return bravia.NewBraviaRemote(config.Address, config.Credential, internal.NewModeOptions(internal.WithDebug(debug), internal.WithTest(testMode))), nil

	default:
		return nil, fmt.Errorf("unsupported device type: %s", config.Type)
	}
}

// GetDevice returns a device by ID
func (dm *DeviceManager) GetDevice(id string) (device.Device, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	device, exists := dm.devices[id]
	if !exists {
		return nil, fmt.Errorf("device not found: %s", id)
	}

	return device, nil
}

// GetAllDevices returns all managed devices
func (dm *DeviceManager) GetAllDevices() map[string]device.Device {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	// Return a copy to prevent external modification
	devices := make(map[string]device.Device)
	for id, dev := range dm.devices {
		devices[id] = dev
	}

	return devices
}

// GetDeviceInfo returns device information for a specific device
func (dm *DeviceManager) GetDeviceInfo(id string) (*device.DeviceInfo, error) {
	dev, err := dm.GetDevice(id)
	if err != nil {
		return nil, err
	}

	info := dev.GetDeviceInfo()
	return &info, nil
}

// GetAllDeviceInfo returns information for all devices
func (dm *DeviceManager) GetAllDeviceInfo() map[string]device.DeviceInfo {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()

	deviceInfos := make(map[string]device.DeviceInfo)
	for id, dev := range dm.devices {
		deviceInfos[id] = dev.GetDeviceInfo()
	}

	return deviceInfos
}

// ProcessDeviceAction processes an action for a specific device with nonce deduplication
func (dm *DeviceManager) ProcessDeviceAction(deviceID string, actionJSON []byte) (*device.ActionResponse, error) {
	dev, err := dm.GetDevice(deviceID)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("Device not found: %s", deviceID),
		}, nil
	}

	dm.logger.Debug().
		Str("device_id", deviceID).
		RawJSON("action", actionJSON).
		Msg("Processing device action")

	response, err := dev.Process(actionJSON)
	if err != nil {
		dm.logger.Error().
			Str("device_id", deviceID).
			Err(err).
			Msg("Device action processing failed")
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("Action processing failed: %v", err),
		}, nil
	}

	dm.logger.Info().
		Str("device_id", deviceID).
		Bool("success", response.Success).
		Msg("Device action processed")

	return response, nil
}

// ProcessDeviceActionWithNonce processes an action for a specific device with nonce-based deduplication
func (dm *DeviceManager) ProcessDeviceActionWithNonce(deviceID, nonce string, actionJSON []byte) (*device.ActionResponse, error) {
	// Check if we've seen this nonce before for this device
	if cachedResponse, found := dm.nonceCache.CheckNonce(deviceID, nonce); found {
		dm.logger.Info().
			Str("device_id", deviceID).
			Str("nonce", nonce).
			Msg("Returning cached response for duplicate nonce")
		return cachedResponse, nil
	}

	// Validate nonce format if provided
	if nonce != "" && !ValidateNonce(nonce) {
		dm.logger.Warn().
			Str("device_id", deviceID).
			Str("nonce", nonce).
			Msg("Invalid nonce format")
		return &device.ActionResponse{
			Success: false,
			Error:   "Invalid nonce format",
		}, nil
	}

	// Process the action normally
	response, err := dm.ProcessDeviceAction(deviceID, actionJSON)
	if err != nil {
		return response, err
	}

	// Cache the response with the nonce (only if nonce is provided)
	if nonce != "" {
		dm.nonceCache.StoreResponse(deviceID, nonce, response)
		dm.logger.Debug().
			Str("device_id", deviceID).
			Str("nonce", nonce).
			Bool("success", response.Success).
			Msg("Cached response for nonce")
	}

	return response, nil
}

// Shutdown gracefully shuts down all devices
func (dm *DeviceManager) Shutdown() {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()

	dm.logger.Info().
		Int("device_count", len(dm.devices)).
		Msg("Shutting down device manager")

	// Shutdown nonce cache
	if dm.nonceCache != nil {
		dm.nonceCache.Shutdown()
	}

	// For now, we just clear the devices map
	// In the future, we might want to add cleanup logic for each device type
	dm.devices = make(map[string]device.Device)

	dm.logger.Info().Msg("Device manager shutdown complete")
}

// Reload reloads devices from the configuration
func (dm *DeviceManager) Reload(newConfig *Config, debug, testMode bool) error {
	dm.logger.Info().Msg("Reloading device manager with new configuration")

	// Shutdown existing devices
	dm.Shutdown()

	// Update configuration
	dm.config = newConfig

	// Initialize with new configuration
	return dm.Initialize(debug, testMode)
}

// GetDeviceCount returns the number of managed devices
func (dm *DeviceManager) GetDeviceCount() int {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	return len(dm.devices)
}

// GetNonceStats returns nonce cache statistics
func (dm *DeviceManager) GetNonceStats() map[string]interface{} {
	if dm.nonceCache == nil {
		return map[string]interface{}{"enabled": false}
	}
	return dm.nonceCache.GetStats()
}

// ClearDeviceNonces clears all cached nonces for a specific device
func (dm *DeviceManager) ClearDeviceNonces(deviceID string) {
	if dm.nonceCache != nil {
		dm.nonceCache.ClearDevice(deviceID)
		dm.logger.Info().
			Str("device_id", deviceID).
			Msg("Cleared device nonce cache")
	}
}
