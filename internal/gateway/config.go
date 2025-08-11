package gateway

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// GatewayConfig represents the complete gateway configuration
type GatewayConfig struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Keys     KeysConfig     `yaml:"keys"`
	Logging  LoggingConfig  `yaml:"logging"`
	Security SecurityConfig `yaml:"security"`
}

// ServerConfig contains server-related settings
type ServerConfig struct {
	API APIConfig `yaml:"api"`
	ZMQ ZMQConfig `yaml:"zmq"`
}

// APIConfig contains HTTP API server settings
type APIConfig struct {
	Address string    `yaml:"address"`
	Timeout string    `yaml:"timeout"`
	TLS     TLSConfig `yaml:"tls"`
}

// TLSConfig contains TLS/SSL settings
type TLSConfig struct {
	Enabled  bool   `yaml:"enabled"`
	CertFile string `yaml:"cert_file"`
	KeyFile  string `yaml:"key_file"`
}

// ZMQConfig contains ZeroMQ server settings
type ZMQConfig struct {
	Address string `yaml:"address"`
	Timeout string `yaml:"timeout"`
}

// DatabaseConfig contains database settings
type DatabaseConfig struct {
	Path           string `yaml:"path"`
	MaxConnections int    `yaml:"max_connections"`
	Timeout        string `yaml:"timeout"`
}

// KeysConfig contains cryptographic key settings
type KeysConfig struct {
	File         string `yaml:"file"`
	AutoGenerate bool   `yaml:"auto_generate"`
	Format       string `yaml:"format"` // "yaml" or "json" for backward compatibility
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
	File   string `yaml:"file"`
}

// SecurityConfig contains security-related settings
type SecurityConfig struct {
	APIKeyRequired bool         `yaml:"api_key_required"`
	RateLimiting   RateLimiting `yaml:"rate_limiting"`
	JWT            JWTConfig    `yaml:"jwt"`
}

// JWTConfig contains JWT token settings
type JWTConfig struct {
	SecretKey   string `yaml:"secret_key"`
	Issuer      string `yaml:"issuer"`
	ExpiryHours int    `yaml:"expiry_hours"`
}

// RateLimiting contains rate limiting settings
type RateLimiting struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requests_per_minute"`
}

// LoadGatewayConfig loads configuration from a YAML file
func LoadGatewayConfig(filepath string) (*GatewayConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config GatewayConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate and set defaults
	if err := config.setDefaults(); err != nil {
		return nil, fmt.Errorf("failed to set defaults: %w", err)
	}

	if err := config.validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return &config, nil
}

// SaveGatewayConfig saves configuration to a YAML file
func SaveGatewayConfig(config *GatewayConfig, filepath string) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// NewDefaultGatewayConfig creates a default configuration
func NewDefaultGatewayConfig() *GatewayConfig {
	return &GatewayConfig{
		Server: ServerConfig{
			API: APIConfig{
				Address: ":8080",
				Timeout: "15s",
				TLS: TLSConfig{
					Enabled:  false,
					CertFile: "",
					KeyFile:  "",
				},
			},
			ZMQ: ZMQConfig{
				Address: "tcp://*:5555",
				Timeout: "30s",
			},
		},
		Database: DatabaseConfig{
			Path:           "gateway.db",
			MaxConnections: 10,
			Timeout:        "5s",
		},
		Keys: KeysConfig{
			File:         "gateway_keys.yml",
			AutoGenerate: true,
			Format:       "yaml",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
			File:   "gateway.log",
		},
		Security: SecurityConfig{
			APIKeyRequired: false,
			RateLimiting: RateLimiting{
				Enabled:           true,
				RequestsPerMinute: 100,
			},
			JWT: JWTConfig{
				SecretKey:   "your-super-secret-jwt-key-change-this-in-production",
				Issuer:      "lucas-gateway",
				ExpiryHours: 24,
			},
		},
	}
}

// setDefaults ensures all required fields have default values
func (c *GatewayConfig) setDefaults() error {
	if c.Server.API.Address == "" {
		c.Server.API.Address = ":8080"
	}
	if c.Server.API.Timeout == "" {
		c.Server.API.Timeout = "15s"
	}
	if c.Server.ZMQ.Address == "" {
		c.Server.ZMQ.Address = "tcp://*:5555"
	}
	if c.Server.ZMQ.Timeout == "" {
		c.Server.ZMQ.Timeout = "30s"
	}

	if c.Database.Path == "" {
		c.Database.Path = "gateway.db"
	}
	if c.Database.MaxConnections == 0 {
		c.Database.MaxConnections = 10
	}
	if c.Database.Timeout == "" {
		c.Database.Timeout = "5s"
	}

	if c.Keys.File == "" {
		c.Keys.File = "gateway_keys.yml"
	}
	if c.Keys.Format == "" {
		c.Keys.Format = "yaml"
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.Format == "" {
		c.Logging.Format = "json"
	}

	if c.Security.RateLimiting.RequestsPerMinute == 0 {
		c.Security.RateLimiting.RequestsPerMinute = 100
	}

	if c.Security.JWT.SecretKey == "" {
		c.Security.JWT.SecretKey = "your-super-secret-jwt-key-change-this-in-production"
	}
	if c.Security.JWT.Issuer == "" {
		c.Security.JWT.Issuer = "lucas-gateway"
	}
	if c.Security.JWT.ExpiryHours == 0 {
		c.Security.JWT.ExpiryHours = 24
	}

	return nil
}

// validate checks if the configuration values are valid
func (c *GatewayConfig) validate() error {
	// Validate timeout durations
	if _, err := time.ParseDuration(c.Server.API.Timeout); err != nil {
		return fmt.Errorf("invalid API timeout format: %w", err)
	}
	if _, err := time.ParseDuration(c.Server.ZMQ.Timeout); err != nil {
		return fmt.Errorf("invalid ZMQ timeout format: %w", err)
	}
	if _, err := time.ParseDuration(c.Database.Timeout); err != nil {
		return fmt.Errorf("invalid database timeout format: %w", err)
	}

	// Validate TLS configuration
	if c.Server.API.TLS.Enabled {
		if c.Server.API.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert_file is required when TLS is enabled")
		}
		if c.Server.API.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key_file is required when TLS is enabled")
		}
	}

	// Validate key format
	if c.Keys.Format != "yaml" && c.Keys.Format != "json" {
		return fmt.Errorf("keys format must be 'yaml' or 'json'")
	}

	// Validate logging level
	validLevels := []string{"debug", "info", "warn", "error", "fatal", "panic"}
	levelValid := false
	for _, level := range validLevels {
		if c.Logging.Level == level {
			levelValid = true
			break
		}
	}
	if !levelValid {
		return fmt.Errorf("invalid logging level: %s (must be one of: %v)", c.Logging.Level, validLevels)
	}

	// Validate logging format
	if c.Logging.Format != "json" && c.Logging.Format != "text" {
		return fmt.Errorf("logging format must be 'json' or 'text'")
	}

	// Validate rate limiting
	if c.Security.RateLimiting.Enabled && c.Security.RateLimiting.RequestsPerMinute <= 0 {
		return fmt.Errorf("requests_per_minute must be greater than 0 when rate limiting is enabled")
	}

	// Validate JWT config
	if c.Security.JWT.SecretKey == "" {
		return fmt.Errorf("JWT secret_key cannot be empty")
	}
	if len(c.Security.JWT.SecretKey) < 32 {
		return fmt.Errorf("JWT secret_key must be at least 32 characters long for security")
	}
	if c.Security.JWT.Issuer == "" {
		return fmt.Errorf("JWT issuer cannot be empty")
	}
	if c.Security.JWT.ExpiryHours <= 0 {
		return fmt.Errorf("JWT expiry_hours must be greater than 0")
	}

	return nil
}

// GetAPITimeout returns the API timeout as a time.Duration
func (c *GatewayConfig) GetAPITimeout() time.Duration {
	duration, _ := time.ParseDuration(c.Server.API.Timeout)
	return duration
}

// GetZMQTimeout returns the ZMQ timeout as a time.Duration
func (c *GatewayConfig) GetZMQTimeout() time.Duration {
	duration, _ := time.ParseDuration(c.Server.ZMQ.Timeout)
	return duration
}

// GetDatabaseTimeout returns the database timeout as a time.Duration
func (c *GatewayConfig) GetDatabaseTimeout() time.Duration {
	duration, _ := time.ParseDuration(c.Database.Timeout)
	return duration
}
