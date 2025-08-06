package hub

import (
	"fmt"
	"os"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Config represents the hub configuration structure
type Config struct {
	Gateway GatewayConfig `yaml:"gateway"`
	Hub     HubConfig     `yaml:"hub"`
	Devices []DeviceConfig `yaml:"devices"`
}

// GatewayConfig contains gateway connection settings
type GatewayConfig struct {
	Endpoint  string `yaml:"endpoint"`
	PublicKey string `yaml:"public_key"`
}

// HubConfig contains hub identity and keys
type HubConfig struct {
	ID         string `yaml:"id"`
	PublicKey  string `yaml:"public_key"`
	PrivateKey string `yaml:"private_key"`
	ProductKey string `yaml:"product_key"`
}

// DeviceConfig represents a single device configuration
type DeviceConfig struct {
	ID           string   `yaml:"id"`
	Type         string   `yaml:"type"`
	Model        string   `yaml:"model"`
	Address      string   `yaml:"address"`
	Credential   string   `yaml:"credential"`
	Capabilities []string `yaml:"capabilities"`
}

// LoadConfig loads configuration from a YAML file
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate required fields
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	// Validate gateway config
	if c.Gateway.Endpoint == "" {
		return fmt.Errorf("gateway.endpoint is required")
	}
	if c.Gateway.PublicKey == "" {
		return fmt.Errorf("gateway.public_key is required")
	}

	// Validate hub config
	if c.Hub.PublicKey == "" {
		return fmt.Errorf("hub.public_key is required")
	}
	if c.Hub.PrivateKey == "" {
		return fmt.Errorf("hub.private_key is required")
	}
	if c.Hub.ID == "" {
		return fmt.Errorf("hub.id is required")
	}
	if c.Hub.ProductKey == "" {
		return fmt.Errorf("hub.product_key is required")
	}

	// Validate devices
	if len(c.Devices) == 0 {
		return fmt.Errorf("at least one device must be configured")
	}

	deviceIDs := make(map[string]bool)
	for i, device := range c.Devices {
		if device.ID == "" {
			return fmt.Errorf("device[%d].id is required", i)
		}
		if deviceIDs[device.ID] {
			return fmt.Errorf("duplicate device ID: %s", device.ID)
		}
		deviceIDs[device.ID] = true

		if device.Type == "" {
			return fmt.Errorf("device[%d].type is required", i)
		}
		if device.Address == "" {
			return fmt.Errorf("device[%d].address is required", i)
		}
	}

	return nil
}

// GetDevice returns a device configuration by ID
func (c *Config) GetDevice(id string) (*DeviceConfig, error) {
	for _, device := range c.Devices {
		if device.ID == id {
			return &device, nil
		}
	}
	return nil, fmt.Errorf("device not found: %s", id)
}

// SaveConfig saves configuration to a YAML file
func SaveConfig(config *Config, filepath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// NewConfigWithKeys creates a new configuration with actual generated keys
func NewConfigWithKeys(gatewayEndpoint, gatewayPublicKey string) (*Config, error) {
	// Generate hub keypair
	hubKeys, err := GenerateHubKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate hub keys: %w", err)
	}

	// Use provided gateway info or defaults
	if gatewayEndpoint == "" {
		gatewayEndpoint = "tcp://localhost:5555"
	}
	if gatewayPublicKey == "" {
		gatewayPublicKey = "gateway_public_key_here"
	}

	return &Config{
		Gateway: GatewayConfig{
			Endpoint:  gatewayEndpoint,
			PublicKey: gatewayPublicKey,
		},
		Hub: HubConfig{
			ID:         generateHubIDString(),
			PublicKey:  hubKeys.PublicKey,
			PrivateKey: hubKeys.PrivateKey,
			ProductKey: uuid.New().String(),
		},
		Devices: []DeviceConfig{
			{
				ID:         "living_room_tv",
				Type:       "bravia",
				Model:      "Sony Bravia",
				Address:    "192.168.1.100",
				Credential: "psk_key_here",
				Capabilities: []string{
					"remote_control",
					"system_control",
					"audio_control",
					"content_control",
				},
			},
		},
	}, nil
}

// generateHubIDString generates a hub ID string for use in configuration
func generateHubIDString() string {
	hubID, err := GenerateHubID()
	if err != nil {
		// Fallback to uuid if hub ID generation fails
		return uuid.New().String()
	}
	return hubID
}

// HasValidKeys returns true if the configuration contains actual keys (not placeholders)
func (c *Config) HasValidKeys() bool {
	return c.Hub.PublicKey != "hub_public_key_here" && 
		   c.Hub.PrivateKey != "hub_private_key_here" &&
		   c.Gateway.PublicKey != "gateway_public_key_here"
}

// HasValidHubKeys returns true if the hub has valid keys (not placeholders)
func (c *Config) HasValidHubKeys() bool {
	return c.Hub.PublicKey != "hub_public_key_here" && 
		   c.Hub.PrivateKey != "hub_private_key_here" &&
		   c.Hub.ProductKey != ""
}

// HasValidGatewayKey returns true if gateway key is not a placeholder
func (c *Config) HasValidGatewayKey() bool {
	return c.Gateway.PublicKey != "gateway_public_key_here" && c.Gateway.PublicKey != ""
}

// UpdateGatewayInfo updates the gateway configuration with discovered information
func (c *Config) UpdateGatewayInfo(endpoint, publicKey string) {
	if endpoint != "" {
		c.Gateway.Endpoint = endpoint
	}
	if publicKey != "" {
		c.Gateway.PublicKey = publicKey
	}
}

// NewDefaultConfig creates a default configuration template
func NewDefaultConfig() *Config {
	return &Config{
		Gateway: GatewayConfig{
			Endpoint:  "tcp://gateway.example.com:5555",
			PublicKey: "gateway_public_key_here",
		},
		Hub: HubConfig{
			ID:         "lucas_hub",
			PublicKey:  "hub_public_key_here",
			PrivateKey: "hub_private_key_here",
			ProductKey: "product_key_here",
		},
		Devices: []DeviceConfig{
			{
				ID:         "living_room_tv",
				Type:       "bravia",
				Model:      "Sony Bravia",
				Address:    "192.168.1.100",
				Credential: "psk_key_here",
				Capabilities: []string{
					"remote_control",
					"system_control",
					"audio_control",
					"content_control",
				},
			},
		},
	}
}