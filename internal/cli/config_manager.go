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

package cli

import (
	"fmt"
	"os"

	"lucas/internal/hub"
)

// ConfigManager handles hub configuration file operations
type ConfigManager struct {
	configPath string
}

// NewConfigManager creates a new config manager
func NewConfigManager(configPath string) *ConfigManager {
	return &ConfigManager{
		configPath: configPath,
	}
}

// LoadConfig loads the hub configuration
func (cm *ConfigManager) LoadConfig() (*hub.Config, error) {
	// Check if file exists
	if _, err := os.Stat(cm.configPath); os.IsNotExist(err) {
		// Create default config if it doesn't exist
		defaultConfig := hub.NewDefaultConfig()
		if err := cm.SaveConfig(defaultConfig); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return defaultConfig, nil
	}

	// Load existing config
	config, err := hub.LoadConfig(cm.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return config, nil
}

// SaveConfig saves the hub configuration
func (cm *ConfigManager) SaveConfig(config *hub.Config) error {
	if err := hub.SaveConfig(config, cm.configPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}
	return nil
}

// AddDevice adds a new device to the configuration
func (cm *ConfigManager) AddDevice(device hub.DeviceConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	// Check if device ID already exists
	for _, existingDevice := range config.Devices {
		if existingDevice.ID == device.ID {
			return fmt.Errorf("device with ID '%s' already exists", device.ID)
		}
	}

	// Add the new device
	config.Devices = append(config.Devices, device)

	return cm.SaveConfig(config)
}

// UpdateDevice updates an existing device in the configuration
func (cm *ConfigManager) UpdateDevice(deviceID string, updatedDevice hub.DeviceConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	// Find and update the device
	for i, device := range config.Devices {
		if device.ID == deviceID {
			// Keep the same ID
			updatedDevice.ID = deviceID
			config.Devices[i] = updatedDevice
			return cm.SaveConfig(config)
		}
	}

	return fmt.Errorf("device with ID '%s' not found", deviceID)
}

// RemoveDevice removes a device from the configuration
func (cm *ConfigManager) RemoveDevice(deviceID string) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	// Find and remove the device
	for i, device := range config.Devices {
		if device.ID == deviceID {
			// Remove device by slicing
			config.Devices = append(config.Devices[:i], config.Devices[i+1:]...)
			return cm.SaveConfig(config)
		}
	}

	return fmt.Errorf("device with ID '%s' not found", deviceID)
}

// GetDevice gets a specific device from the configuration
func (cm *ConfigManager) GetDevice(deviceID string) (*hub.DeviceConfig, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	for _, device := range config.Devices {
		if device.ID == deviceID {
			return &device, nil
		}
	}

	return nil, fmt.Errorf("device with ID '%s' not found", deviceID)
}

// ListDevices returns all devices from the configuration
func (cm *ConfigManager) ListDevices() ([]hub.DeviceConfig, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	return config.Devices, nil
}

// ValidateConfig validates the configuration
func (cm *ConfigManager) ValidateConfig() error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	return config.Validate()
}

// GetGatewayConfig returns the gateway configuration
func (cm *ConfigManager) GetGatewayConfig() (*hub.GatewayConfig, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	return &config.Gateway, nil
}

// UpdateGatewayConfig updates the gateway configuration
func (cm *ConfigManager) UpdateGatewayConfig(gateway hub.GatewayConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	config.Gateway = gateway
	return cm.SaveConfig(config)
}

// GetHubConfig returns the hub configuration
func (cm *ConfigManager) GetHubConfig() (*hub.HubConfig, error) {
	config, err := cm.LoadConfig()
	if err != nil {
		return nil, err
	}

	return &config.Hub, nil
}

// UpdateHubConfig updates the hub configuration
func (cm *ConfigManager) UpdateHubConfig(hubConfig hub.HubConfig) error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	config.Hub = hubConfig
	return cm.SaveConfig(config)
}

// GetConfigPath returns the configuration file path
func (cm *ConfigManager) GetConfigPath() string {
	return cm.configPath
}

// SetConfigPath sets the configuration file path
func (cm *ConfigManager) SetConfigPath(path string) {
	cm.configPath = path
}

// BackupConfig creates a backup of the current configuration
func (cm *ConfigManager) BackupConfig() error {
	config, err := cm.LoadConfig()
	if err != nil {
		return err
	}

	backupPath := cm.configPath + ".backup"
	return hub.SaveConfig(config, backupPath)
}

// RestoreFromBackup restores configuration from backup
func (cm *ConfigManager) RestoreFromBackup() error {
	backupPath := cm.configPath + ".backup"
	
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		return fmt.Errorf("backup file does not exist: %s", backupPath)
	}

	config, err := hub.LoadConfig(backupPath)
	if err != nil {
		return fmt.Errorf("failed to load backup: %w", err)
	}

	return cm.SaveConfig(config)
}

// DeviceExists checks if a device with the given ID exists
func (cm *ConfigManager) DeviceExists(deviceID string) bool {
	_, err := cm.GetDevice(deviceID)
	return err == nil
}

// GetDeviceCount returns the number of devices in the configuration
func (cm *ConfigManager) GetDeviceCount() (int, error) {
	devices, err := cm.ListDevices()
	if err != nil {
		return 0, err
	}
	return len(devices), nil
}

// GetDevicesByType returns devices of a specific type
func (cm *ConfigManager) GetDevicesByType(deviceType string) ([]hub.DeviceConfig, error) {
	devices, err := cm.ListDevices()
	if err != nil {
		return nil, err
	}

	var filtered []hub.DeviceConfig
	for _, device := range devices {
		if device.Type == deviceType {
			filtered = append(filtered, device)
		}
	}

	return filtered, nil
}

// GetSupportedDeviceTypes returns a list of supported device types
func (cm *ConfigManager) GetSupportedDeviceTypes() []string {
	return []string{
		"bravia", // Sony Bravia TV
		// Add more device types as they are implemented
	}
}

// CreateDeviceTemplate creates a template device configuration
func (cm *ConfigManager) CreateDeviceTemplate(deviceType string) hub.DeviceConfig {
	switch deviceType {
	case "bravia":
		return hub.DeviceConfig{
			ID:         "",
			Type:       "bravia",
			Model:      "Sony Bravia",
			Address:    "192.168.1.100",
			Credential: "",
			Capabilities: []string{
				"remote_control",
				"system_control",
				"audio_control",
				"content_control",
			},
		}
	default:
		return hub.DeviceConfig{
			ID:           "",
			Type:         deviceType,
			Model:        "",
			Address:      "",
			Credential:   "",
			Capabilities: []string{},
		}
	}
}