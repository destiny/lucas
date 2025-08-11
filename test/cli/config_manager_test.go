package cli_test

import (
	"os"
	"path/filepath"
	"testing"

	"lucas/internal/cli"
	"lucas/internal/hub"
)

func setupTestConfigManager(t *testing.T) (*cli.ConfigManager, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	cm := cli.NewConfigManager(configPath)

	cleanup := func() {
		os.Remove(configPath)
		os.Remove(configPath + ".backup")
	}

	return cm, cleanup
}

func createValidConfig() *hub.Config {
	return &hub.Config{
		Gateway: hub.GatewayConfig{
			Endpoint:  "tcp://localhost:5555",
			PublicKey: "gateway_public_key",
		},
		Hub: hub.HubConfig{
			ID:         "test-hub-id",
			PublicKey:  "hub_public_key",
			PrivateKey: "hub_private_key",
			ProductKey: "product_key_123",
		},
		Devices: []hub.DeviceConfig{
			{
				ID:           "device1",
				Type:         "bravia",
				Model:        "Sony Bravia",
				Address:      "192.168.1.100",
				Credential:   "credential1",
				Capabilities: []string{"remote_control", "system_control"},
			},
		},
	}
}

func TestNewConfigManager(t *testing.T) {
	configPath := "/tmp/test_config.yaml"
	cm := cli.NewConfigManager(configPath)

	if cm.GetConfigPath() != configPath {
		t.Errorf("Expected config path %s, got %s", configPath, cm.GetConfigPath())
	}
}

func TestLoadConfig(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	t.Run("load nonexistent config creates default", func(t *testing.T) {
		config, err := cm.LoadConfig()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if config == nil {
			t.Error("Expected config to be created")
		}

		// Verify file was created
		if _, err := os.Stat(cm.GetConfigPath()); os.IsNotExist(err) {
			t.Error("Expected config file to be created")
		}
	})

	t.Run("load existing config", func(t *testing.T) {
		// First create a valid config
		validConfig := createValidConfig()
		err := cm.SaveConfig(validConfig)
		if err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Now load it
		config, err := cm.LoadConfig()
		if err != nil {
			t.Fatalf("Failed to load config: %v", err)
		}

		if config.Hub.ID != validConfig.Hub.ID {
			t.Errorf("Expected hub ID %s, got %s", validConfig.Hub.ID, config.Hub.ID)
		}
	})
}

func TestSaveConfig(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	config := createValidConfig()

	err := cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cm.GetConfigPath()); os.IsNotExist(err) {
		t.Error("Expected config file to exist after save")
	}
}

func TestDeviceOperations(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	// Initialize with a valid config
	config := createValidConfig()
	err := cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save initial config: %v", err)
	}

	newDevice := hub.DeviceConfig{
		ID:           "device2",
		Type:         "bravia",
		Model:        "Sony Bravia X900H",
		Address:      "192.168.1.101",
		Credential:   "credential2",
		Capabilities: []string{"remote_control", "audio_control"},
	}

	t.Run("AddDevice", func(t *testing.T) {
		err := cm.AddDevice(newDevice)
		if err != nil {
			t.Fatalf("Failed to add device: %v", err)
		}

		// Verify device was added
		device, err := cm.GetDevice("device2")
		if err != nil {
			t.Fatalf("Failed to get added device: %v", err)
		}

		if device.ID != newDevice.ID {
			t.Errorf("Expected device ID %s, got %s", newDevice.ID, device.ID)
		}
	})

	t.Run("AddDevice duplicate ID", func(t *testing.T) {
		err := cm.AddDevice(newDevice) // Same device again
		if err == nil {
			t.Error("Expected error when adding device with duplicate ID")
		}
	})

	t.Run("GetDevice", func(t *testing.T) {
		device, err := cm.GetDevice("device1")
		if err != nil {
			t.Fatalf("Failed to get device: %v", err)
		}

		if device.Type != "bravia" {
			t.Errorf("Expected device type 'bravia', got %s", device.Type)
		}
	})

	t.Run("GetDevice nonexistent", func(t *testing.T) {
		_, err := cm.GetDevice("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent device")
		}
	})

	t.Run("UpdateDevice", func(t *testing.T) {
		updatedDevice := hub.DeviceConfig{
			ID:           "device1", // Will be overridden by UpdateDevice
			Type:         "bravia",
			Model:        "Updated Model",
			Address:      "192.168.1.150",
			Credential:   "updated_credential",
			Capabilities: []string{"remote_control", "system_control", "audio_control"},
		}

		err := cm.UpdateDevice("device1", updatedDevice)
		if err != nil {
			t.Fatalf("Failed to update device: %v", err)
		}

		// Verify device was updated
		device, err := cm.GetDevice("device1")
		if err != nil {
			t.Fatalf("Failed to get updated device: %v", err)
		}

		if device.Model != "Updated Model" {
			t.Errorf("Expected model 'Updated Model', got %s", device.Model)
		}
		if len(device.Capabilities) != 3 {
			t.Errorf("Expected 3 capabilities, got %d", len(device.Capabilities))
		}
	})

	t.Run("UpdateDevice nonexistent", func(t *testing.T) {
		err := cm.UpdateDevice("nonexistent", newDevice)
		if err == nil {
			t.Error("Expected error when updating nonexistent device")
		}
	})

	t.Run("ListDevices", func(t *testing.T) {
		devices, err := cm.ListDevices()
		if err != nil {
			t.Fatalf("Failed to list devices: %v", err)
		}

		// Should have 2 devices (original + added one)
		if len(devices) != 2 {
			t.Errorf("Expected 2 devices, got %d", len(devices))
		}
	})

	t.Run("RemoveDevice", func(t *testing.T) {
		err := cm.RemoveDevice("device2")
		if err != nil {
			t.Fatalf("Failed to remove device: %v", err)
		}

		// Verify device was removed
		_, err = cm.GetDevice("device2")
		if err == nil {
			t.Error("Expected error when getting removed device")
		}

		// Verify only 1 device remains
		devices, err := cm.ListDevices()
		if err != nil {
			t.Fatalf("Failed to list devices: %v", err)
		}
		if len(devices) != 1 {
			t.Errorf("Expected 1 device after removal, got %d", len(devices))
		}
	})

	t.Run("RemoveDevice nonexistent", func(t *testing.T) {
		err := cm.RemoveDevice("nonexistent")
		if err == nil {
			t.Error("Expected error when removing nonexistent device")
		}
	})
}

func TestDeviceUtilities(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	// Initialize with a valid config with multiple device types
	config := createValidConfig()
	config.Devices = append(config.Devices, hub.DeviceConfig{
		ID:           "speaker1",
		Type:         "speaker",
		Model:        "Sonos One",
		Address:      "192.168.1.102",
		Credential:   "speaker_cred",
		Capabilities: []string{"audio_control"},
	})

	err := cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Run("DeviceExists", func(t *testing.T) {
		if !cm.DeviceExists("device1") {
			t.Error("Expected device1 to exist")
		}

		if cm.DeviceExists("nonexistent") {
			t.Error("Expected nonexistent device to not exist")
		}
	})

	t.Run("GetDeviceCount", func(t *testing.T) {
		count, err := cm.GetDeviceCount()
		if err != nil {
			t.Fatalf("Failed to get device count: %v", err)
		}

		if count != 2 {
			t.Errorf("Expected 2 devices, got %d", count)
		}
	})

	t.Run("GetDevicesByType", func(t *testing.T) {
		braviaDevices, err := cm.GetDevicesByType("bravia")
		if err != nil {
			t.Fatalf("Failed to get bravia devices: %v", err)
		}

		if len(braviaDevices) != 1 {
			t.Errorf("Expected 1 bravia device, got %d", len(braviaDevices))
		}

		speakerDevices, err := cm.GetDevicesByType("speaker")
		if err != nil {
			t.Fatalf("Failed to get speaker devices: %v", err)
		}

		if len(speakerDevices) != 1 {
			t.Errorf("Expected 1 speaker device, got %d", len(speakerDevices))
		}

		// Test non-existent type
		nonExistentDevices, err := cm.GetDevicesByType("nonexistent")
		if err != nil {
			t.Fatalf("Failed to get nonexistent type devices: %v", err)
		}

		if len(nonExistentDevices) != 0 {
			t.Errorf("Expected 0 nonexistent devices, got %d", len(nonExistentDevices))
		}
	})

	t.Run("GetSupportedDeviceTypes", func(t *testing.T) {
		types := cm.GetSupportedDeviceTypes()

		if len(types) == 0 {
			t.Error("Expected at least one supported device type")
		}

		// Should include bravia
		found := false
		for _, deviceType := range types {
			if deviceType == "bravia" {
				found = true
				break
			}
		}
		if !found {
			t.Error("Expected 'bravia' to be in supported device types")
		}
	})

	t.Run("CreateDeviceTemplate", func(t *testing.T) {
		template := cm.CreateDeviceTemplate("bravia")

		if template.Type != "bravia" {
			t.Errorf("Expected template type 'bravia', got %s", template.Type)
		}
		if template.Model != "Sony Bravia" {
			t.Errorf("Expected template model 'Sony Bravia', got %s", template.Model)
		}
		if len(template.Capabilities) == 0 {
			t.Error("Expected template to have capabilities")
		}

		// Test unknown type
		unknownTemplate := cm.CreateDeviceTemplate("unknown")
		if unknownTemplate.Type != "unknown" {
			t.Errorf("Expected template type 'unknown', got %s", unknownTemplate.Type)
		}
		if unknownTemplate.Model != "" {
			t.Errorf("Expected empty model for unknown type, got %s", unknownTemplate.Model)
		}
	})
}

func TestConfigOperations(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	// Initialize with valid config
	config := createValidConfig()
	err := cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Run("ValidateConfig", func(t *testing.T) {
		err := cm.ValidateConfig()
		if err != nil {
			t.Errorf("Valid config should pass validation: %v", err)
		}
	})

	t.Run("GetGatewayConfig", func(t *testing.T) {
		gatewayConfig, err := cm.GetGatewayConfig()
		if err != nil {
			t.Fatalf("Failed to get gateway config: %v", err)
		}

		if gatewayConfig.Endpoint != config.Gateway.Endpoint {
			t.Errorf("Expected endpoint %s, got %s", config.Gateway.Endpoint, gatewayConfig.Endpoint)
		}
	})

	t.Run("UpdateGatewayConfig", func(t *testing.T) {
		newGateway := hub.GatewayConfig{
			Endpoint:  "tcp://newgateway:5555",
			PublicKey: "new_gateway_key",
		}

		err := cm.UpdateGatewayConfig(newGateway)
		if err != nil {
			t.Fatalf("Failed to update gateway config: %v", err)
		}

		// Verify update
		gatewayConfig, err := cm.GetGatewayConfig()
		if err != nil {
			t.Fatalf("Failed to get updated gateway config: %v", err)
		}

		if gatewayConfig.Endpoint != newGateway.Endpoint {
			t.Errorf("Expected updated endpoint %s, got %s", newGateway.Endpoint, gatewayConfig.Endpoint)
		}
	})

	t.Run("GetHubConfig", func(t *testing.T) {
		hubConfig, err := cm.GetHubConfig()
		if err != nil {
			t.Fatalf("Failed to get hub config: %v", err)
		}

		if hubConfig.ID != config.Hub.ID {
			t.Errorf("Expected hub ID %s, got %s", config.Hub.ID, hubConfig.ID)
		}
	})

	t.Run("UpdateHubConfig", func(t *testing.T) {
		newHub := hub.HubConfig{
			ID:         "new-hub-id",
			PublicKey:  "new_hub_public_key",
			PrivateKey: "new_hub_private_key",
			ProductKey: "new_product_key",
		}

		err := cm.UpdateHubConfig(newHub)
		if err != nil {
			t.Fatalf("Failed to update hub config: %v", err)
		}

		// Verify update
		hubConfig, err := cm.GetHubConfig()
		if err != nil {
			t.Fatalf("Failed to get updated hub config: %v", err)
		}

		if hubConfig.ID != newHub.ID {
			t.Errorf("Expected updated hub ID %s, got %s", newHub.ID, hubConfig.ID)
		}
	})
}

func TestConfigPath(t *testing.T) {
	originalPath := "/tmp/original.yaml"
	cm := cli.NewConfigManager(originalPath)

	if cm.GetConfigPath() != originalPath {
		t.Errorf("Expected config path %s, got %s", originalPath, cm.GetConfigPath())
	}

	newPath := "/tmp/new.yaml"
	cm.SetConfigPath(newPath)

	if cm.GetConfigPath() != newPath {
		t.Errorf("Expected updated config path %s, got %s", newPath, cm.GetConfigPath())
	}
}

func TestBackupAndRestore(t *testing.T) {
	cm, cleanup := setupTestConfigManager(t)
	defer cleanup()

	// Create and save a config
	config := createValidConfig()
	err := cm.SaveConfig(config)
	if err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	t.Run("BackupConfig", func(t *testing.T) {
		err := cm.BackupConfig()
		if err != nil {
			t.Fatalf("Failed to backup config: %v", err)
		}

		// Verify backup file exists
		backupPath := cm.GetConfigPath() + ".backup"
		if _, err := os.Stat(backupPath); os.IsNotExist(err) {
			t.Error("Expected backup file to exist")
		}
	})

	t.Run("RestoreFromBackup", func(t *testing.T) {
		// First backup the config
		err := cm.BackupConfig()
		if err != nil {
			t.Fatalf("Failed to backup config: %v", err)
		}

		// Modify the config
		newDevice := hub.DeviceConfig{
			ID:           "restore_test",
			Type:         "bravia",
			Model:        "Test Device",
			Address:      "192.168.1.200",
			Credential:   "test_cred",
			Capabilities: []string{"remote_control"},
		}
		err = cm.AddDevice(newDevice)
		if err != nil {
			t.Fatalf("Failed to add device: %v", err)
		}

		// Verify device was added
		if !cm.DeviceExists("restore_test") {
			t.Error("Device should exist before restore")
		}

		// Restore from backup
		err = cm.RestoreFromBackup()
		if err != nil {
			t.Fatalf("Failed to restore from backup: %v", err)
		}

		// Verify device is gone (restored to backup state)
		if cm.DeviceExists("restore_test") {
			t.Error("Device should not exist after restore")
		}
	})

	t.Run("RestoreFromBackup nonexistent", func(t *testing.T) {
		// Remove backup file
		backupPath := cm.GetConfigPath() + ".backup"
		os.Remove(backupPath)

		err := cm.RestoreFromBackup()
		if err == nil {
			t.Error("Expected error when restoring from nonexistent backup")
		}
	})
}
