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
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"lucas/internal/device"
	"lucas/internal/logger"
	"github.com/rs/zerolog"
)

// GatewayMessage represents a message from the gateway
type GatewayMessage struct {
	ID        string          `json:"id"`
	Nonce     string          `json:"nonce"`     // Unique nonce for idempotency
	Timestamp string          `json:"timestamp"`
	DeviceID  string          `json:"device_id"`
	Action    json.RawMessage `json:"action"`
}

// HubResponse represents a response from the hub to the gateway
type HubResponse struct {
	ID        string      `json:"id"`
	Nonce     string      `json:"nonce"`     // Echo back the nonce
	Timestamp string      `json:"timestamp"`
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
}

// Daemon represents the hub daemon
type Daemon struct {
	config        *Config
	configPath    string
	deviceManager *DeviceManager
	workerService *WorkerService
	configAPI     *ConfigAPIServer
	logger        zerolog.Logger
	running       bool
	mutex         sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	debug         bool
	testMode      bool
}

// NewDaemon creates a new hub daemon
func NewDaemon(configPath string, debug, testMode bool) (*Daemon, error) {
	// Load configuration
	config, err := LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	daemon := &Daemon{
		config:     config,
		configPath: configPath,
		logger:     logger.New(),
		ctx:        ctx,
		cancel:     cancel,
		debug:      debug,
		testMode:   testMode,
	}

	// Initialize device manager
	daemon.deviceManager = NewDeviceManager(config)

	// Initialize worker service
	daemon.workerService = NewWorkerService(config, daemon.deviceManager)

	// Initialize configuration API server (port 8081)
	daemon.configAPI = NewConfigAPIServer(daemon, 8081)

	return daemon, nil
}

// needsRegistration checks if the hub needs to register with gateway
// by detecting if gateway public key is still a placeholder
func (d *Daemon) needsRegistration() bool {
	return d.config.Gateway.PublicKey == "" || 
		   d.config.Gateway.PublicKey == "gateway_public_key_here"
}

// autoRegister performs automatic registration with gateway using default locations
func (d *Daemon) autoRegister() error {
	d.logger.Info().Msg("Gateway not registered, performing auto-registration...")

	// Try common gateway URLs
	gatewayURLs := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080", 
		"http://gateway:8080",
		"http://gateway.local:8080",
	}

	discovery := NewGatewayDiscovery()
	var lastErr error

	for _, gatewayURL := range gatewayURLs {
		d.logger.Debug().
			Str("gateway_url", gatewayURL).
			Msg("Attempting auto-registration with gateway")

		// Try to get gateway info first to verify it's reachable
		gatewayInfo, err := discovery.GetGatewayInfo(gatewayURL)
		if err != nil {
			lastErr = err
			continue
		}

		// Perform registration
		err = discovery.RegisterWithGateway(
			gatewayURL,
			d.config.Hub.ID,
			d.config.Hub.PublicKey,
			d.config.Hub.ProductKey,
		)
		if err != nil {
			lastErr = err
			continue
		}

		// Update config with gateway information
		d.config.UpdateGatewayInfo(gatewayInfo.ZMQEndpoint, gatewayInfo.PublicKey)

		d.logger.Info().
			Str("gateway_url", gatewayURL).
			Str("gateway_endpoint", gatewayInfo.ZMQEndpoint).
			Msg("Auto-registration successful")

		return nil
	}

	return fmt.Errorf("failed to auto-register with any gateway: %w", lastErr)
}

// Start starts the hub daemon
func (d *Daemon) Start() error {
	d.mutex.Lock()
	if d.running {
		d.mutex.Unlock()
		return fmt.Errorf("daemon is already running")
	}
	d.running = true
	d.mutex.Unlock()

	d.logger.Info().
		Bool("debug", d.debug).
		Bool("test_mode", d.testMode).
		Msg("Starting Lucas Hub daemon")

	// Initialize devices
	if err := d.deviceManager.Initialize(d.debug, d.testMode); err != nil {
		return fmt.Errorf("failed to initialize devices: %w", err)
	}

	// Check if auto-registration is needed
	if d.needsRegistration() {
		if err := d.autoRegister(); err != nil {
			d.logger.Warn().
				Err(err).
				Msg("Auto-registration failed, continuing in offline mode")
			// Continue anyway - hub can still work offline
		} else {
			// Auto-registration succeeded, save updated config to file
			if err := SaveConfig(d.config, d.configPath); err != nil {
				d.logger.Warn().
					Err(err).
					Msg("Failed to save updated config after auto-registration")
			} else {
				d.logger.Info().Msg("Configuration updated with gateway information")
			}
		}
	}

	// Start configuration API server
	if err := d.configAPI.Start(); err != nil {
		return fmt.Errorf("failed to start configuration API: %w", err)
	}

	// Start worker service to connect to gateway via Hermes
	if err := d.workerService.Start(); err != nil {
		return fmt.Errorf("failed to start worker service: %w", err)
	}

	// Set up signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start health check routine
	go d.startHealthCheck()

	d.logger.Info().
		Int("device_count", d.deviceManager.GetDeviceCount()).
		Str("gateway_endpoint", d.config.Gateway.Endpoint).
		Msg("Hub daemon started successfully")

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		d.logger.Info().
			Str("signal", sig.String()).
			Msg("Received shutdown signal")
		return d.Stop()
	case <-d.ctx.Done():
		d.logger.Info().Msg("Context cancelled")
		return d.Stop()
	}
}

// Stop stops the hub daemon gracefully
func (d *Daemon) Stop() error {
	d.mutex.Lock()
	if !d.running {
		d.mutex.Unlock()
		return nil
	}
	d.running = false
	d.mutex.Unlock()

	d.logger.Info().Msg("Stopping hub daemon")

	// Cancel context to signal shutdown
	d.cancel()

	// Stop configuration API server
	if err := d.configAPI.Stop(); err != nil {
		d.logger.Error().Err(err).Msg("Error stopping configuration API")
	}

	// Stop worker service
	if err := d.workerService.Stop(); err != nil {
		d.logger.Error().Err(err).Msg("Error stopping worker service")
	}

	// Shutdown device manager
	d.deviceManager.Shutdown()

	d.logger.Info().Msg("Hub daemon stopped")
	return nil
}

// handleGatewayMessage processes messages received from the gateway with nonce-based deduplication
func (d *Daemon) handleGatewayMessage(msg *GatewayMessage) *HubResponse {
	d.logger.Info().
		Str("message_id", msg.ID).
		Str("nonce", msg.Nonce).
		Str("device_id", msg.DeviceID).
		Msg("Processing gateway message")

	// Process device action
	return d.processDeviceAction(msg, msg.Action)
}

func (d *Daemon) processDeviceAction(msg *GatewayMessage, action json.RawMessage) *HubResponse {
	// Validate message
	if msg.DeviceID == "" {
		return &HubResponse{
			ID:        msg.ID,
			Nonce:     msg.Nonce,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Success:   false,
			Error:     "device_id is required",
		}
	}

	// Process device action with nonce-based deduplication
	response, err := d.deviceManager.ProcessDeviceActionWithNonce(msg.DeviceID, msg.Nonce, action)
	if err != nil {
		d.logger.Error().
			Str("message_id", msg.ID).
			Str("nonce", msg.Nonce).
			Str("device_id", msg.DeviceID).
			Err(err).
			Msg("Failed to process device action")

		return &HubResponse{
			ID:        msg.ID,
			Nonce:     msg.Nonce,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Success:   false,
			Error:     fmt.Sprintf("Failed to process action: %v", err),
		}
	}

	// Convert device response to hub response
	hubResponse := &HubResponse{
		ID:        msg.ID,
		Nonce:     msg.Nonce,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Success:   response.Success,
		Data:      response.Data,
		Error:     response.Error,
	}

	d.logger.Info().
		Str("message_id", msg.ID).
		Str("nonce", msg.Nonce).
		Str("device_id", msg.DeviceID).
		Bool("success", response.Success).
		Msg("Gateway message processed")

	return hubResponse
}


// startHealthCheck starts a periodic health check routine
func (d *Daemon) startHealthCheck() {
	ticker := time.NewTicker(60 * time.Second) // Health check every 60 seconds for less system noise
	defer ticker.Stop()

	d.logger.Info().Msg("Starting health check routine")

	for {
		select {
		case <-ticker.C:
			d.performHealthCheck()
		case <-d.ctx.Done():
			d.logger.Info().Msg("Health check routine stopping")
			return
		}
	}
}

// performHealthCheck performs a health check of the system
func (d *Daemon) performHealthCheck() {
	d.logger.Debug().Msg("Performing health check")

	// Test actual gateway connectivity by sending a test request
	gatewayReachable := d.testGatewayConnectivity()
	
	if gatewayReachable {
		d.logger.Debug().Msg("Gateway connectivity OK - test request successful")
	} else {
		d.logger.Warn().Msg("Gateway connectivity FAILED - test request failed")
	}

	// Check device manager status
	deviceCount := d.deviceManager.GetDeviceCount()
	d.logger.Debug().
		Int("device_count", deviceCount).
		Msg("Device manager status")

	// Log overall health status with actual connectivity
	d.logger.Info().
		Bool("gateway_reachable", gatewayReachable).
		Bool("worker_connected", d.workerService.IsConnected()).
		Int("device_count", deviceCount).
		Msg("Health check completed")
}

// testGatewayConnectivity tests gateway connectivity using existing worker infrastructure
func (d *Daemon) testGatewayConnectivity() bool {
	// Check if worker service has any connected workers
	if !d.workerService.IsConnected() {
		d.logger.Debug().Msg("No workers connected to gateway")
		return false
	}
	
	// Get recent heartbeat/connection health from worker service
	return d.workerService.IsGatewayReachable()
}

// IsRunning returns whether the daemon is currently running
func (d *Daemon) IsRunning() bool {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	return d.running
}

// GetStatus returns the current status of the daemon
func (d *Daemon) GetStatus() map[string]interface{} {
	d.mutex.RLock()
	defer d.mutex.RUnlock()

	return map[string]interface{}{
		"running":           d.running,
		"debug":             d.debug,
		"test_mode":         d.testMode,
		"gateway_connected": d.workerService.IsConnected(),
		"device_count":      d.deviceManager.GetDeviceCount(),
		"devices":           d.deviceManager.GetAllDeviceInfo(),
		"nonce_cache":       d.deviceManager.GetNonceStats(),
	}
}

// ReloadConfig reloads the configuration and reinitializes components
func (d *Daemon) ReloadConfig(configPath string) error {
	d.logger.Info().
		Str("config_path", configPath).
		Msg("Reloading configuration")

	// Load new configuration
	newConfig, err := LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load new config: %w", err)
	}

	// Update daemon configuration
	d.config = newConfig

	// Reload device manager
	if err := d.deviceManager.Reload(newConfig, d.debug, d.testMode); err != nil {
		return fmt.Errorf("failed to reload device manager: %w", err)
	}

	// Note: Gateway client would need to be reconnected if gateway config changed
	// For now, we'll just log that this would require a restart
	d.logger.Info().Msg("Configuration reloaded successfully (gateway reconnection requires restart)")

	return nil
}

// ProcessDeviceAction provides external access to device action processing
func (d *Daemon) ProcessDeviceAction(deviceID string, actionJSON []byte) (*device.ActionResponse, error) {
	return d.deviceManager.ProcessDeviceAction(deviceID, actionJSON)
}

// GetDevices returns information about all managed devices
func (d *Daemon) GetDevices() map[string]device.DeviceInfo {
	return d.deviceManager.GetAllDeviceInfo()
}

// GetDevice returns information about a specific device
func (d *Daemon) GetDevice(deviceID string) (*device.DeviceInfo, error) {
	return d.deviceManager.GetDeviceInfo(deviceID)
}