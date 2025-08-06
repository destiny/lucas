package hub

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
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

// GatewayClient handles ZMQ communication with the gateway
type GatewayClient struct {
	socket          *zmq4.Socket
	config          *Config
	logger          zerolog.Logger
	connected       bool
	reconnectDelay  time.Duration
	maxReconnectDelay time.Duration
	mutex           sync.RWMutex // Protects socket access from multiple goroutines
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(config *Config) *GatewayClient {
	return &GatewayClient{
		config:          config,
		logger:          logger.New(),
		reconnectDelay:  1 * time.Second,   // Initial delay
		maxReconnectDelay: 30 * time.Second, // Maximum delay for internet connections
	}
}

// Connect establishes connection to the gateway with CurveZMQ encryption
func (gc *GatewayClient) Connect() error {
	// Validate configuration before attempting connection
	if err := gc.validateConfig(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}
	
	gc.logger.Info().
		Str("endpoint", gc.config.Gateway.Endpoint).
		Msg("Connecting to gateway")

	// Create DEALER socket for asynchronous communication with ROUTER
	socket, err := zmq4.NewSocket(zmq4.DEALER)
	if err != nil {
		return fmt.Errorf("failed to create ZMQ socket: %w", err)
	}
	
	// Ensure socket is cleaned up on any error
	defer func() {
		if err != nil {
			socket.Close()
		}
	}()

	// Configure CurveZMQ client
	err = socket.ClientAuthCurve(gc.config.Gateway.PublicKey, gc.config.Hub.PublicKey, gc.config.Hub.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to configure CurveZMQ client: %w", err)
	}
	gc.logger.Info().Msg("CurveZMQ client authentication enabled")

	// Set socket options
	err = socket.SetLinger(1000) // 1 second linger time
	if err != nil {
		return fmt.Errorf("failed to set socket linger: %w", err)
	}

	err = socket.SetRcvtimeo(30 * time.Second) // 30 second receive timeout
	if err != nil {
		return fmt.Errorf("failed to set receive timeout: %w", err)
	}

	err = socket.SetSndtimeo(30 * time.Second) // 30 second send timeout
	if err != nil {
		return fmt.Errorf("failed to set send timeout: %w", err)
	}

	// Connect to gateway
	err = socket.Connect(gc.config.Gateway.Endpoint)
	if err != nil {
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}

	// Set socket and connection state atomically
	gc.mutex.Lock()
	gc.socket = socket
	gc.connected = true
	gc.mutex.Unlock()

	gc.logger.Info().Msg("Successfully connected to gateway")

	return nil
}

// PerformHandshake performs the ZMQ handshake sequence with the gateway
func (gc *GatewayClient) PerformHandshake() error {
	if !gc.connected {
		return fmt.Errorf("not connected to gateway")
	}

	gc.logger.Info().Msg("Starting ZMQ handshake with gateway")

	// Step 1: Send hub_online message and wait for response
	if err := gc.SendHubOnline(); err != nil {
		return fmt.Errorf("failed to send hub_online message: %w", err)
	}

	// Step 2: Send device list to gateway for registration and wait for response
	if err := gc.registerDevices(); err != nil {
		return fmt.Errorf("failed to register devices: %w", err)
	}

	gc.logger.Info().Msg("ZMQ handshake completed successfully")
	return nil
}

// registerDevices sends device list to gateway during handshake
func (gc *GatewayClient) registerDevices() error {
	// Convert devices from config to array for JSON serialization
	devices := make([]interface{}, 0, len(gc.config.Devices))
	for _, device := range gc.config.Devices {
		// Use device model as name if available, otherwise use device ID
		deviceName := device.Model
		if deviceName == "" {
			deviceName = device.ID
		}
		
		devices = append(devices, map[string]interface{}{
			"id":           device.ID,
			"type":         device.Type,
			"name":         deviceName,
			"model":        device.Model,
			"address":      device.Address,
			"capabilities": device.Capabilities,
		})
	}

	return gc.SendDeviceListArray(devices)
}

// ConnectWithRetry connects to gateway with exponential backoff retry
func (gc *GatewayClient) ConnectWithRetry() error {
	attempt := 1
	delay := gc.reconnectDelay
	
	for {
		gc.logger.Info().
			Int("attempt", attempt).
			Dur("delay", delay).
			Msg("Attempting to connect to gateway")
			
		err := gc.Connect()
		if err == nil {
			// Reset delay on successful connection
			gc.reconnectDelay = 1 * time.Second
			return nil
		}
		
		gc.logger.Warn().
			Err(err).
			Int("attempt", attempt).
			Dur("next_retry_delay", delay).
			Msg("Failed to connect to gateway, retrying...")
		
		time.Sleep(delay)
		
		// Exponential backoff with maximum delay
		delay = time.Duration(float64(delay) * 1.5)
		if delay > gc.maxReconnectDelay {
			delay = gc.maxReconnectDelay
		}
		
		attempt++
	}
}

// reconnect attempts to reconnect after connection loss
func (gc *GatewayClient) reconnect() error {
	// Use exclusive lock to make reconnection atomic and prevent race conditions
	gc.mutex.Lock()
	defer gc.mutex.Unlock()
	
	gc.logger.Warn().Msg("Connection lost, attempting to reconnect")
	
	// Clean up existing connection (already holding lock)
	if gc.socket != nil {
		gc.socket.Close()
		gc.socket = nil
	}
	gc.connected = false
	
	// Temporarily unlock during connection attempts to avoid blocking other operations too long
	gc.mutex.Unlock()
	
	// Attempt reconnection with backoff
	err := gc.ConnectWithRetry()
	if err != nil {
		gc.mutex.Lock() // Re-lock before returning
		return fmt.Errorf("failed to reconnect: %w", err)
	}
	
	// Re-perform handshake
	err = gc.PerformHandshake()
	
	// Re-lock for final state update
	gc.mutex.Lock()
	
	if err != nil {
		gc.logger.Error().Err(err).Msg("Failed to re-establish handshake after reconnection")
		// Clean up on handshake failure
		if gc.socket != nil {
			gc.socket.Close()
			gc.socket = nil
		}
		gc.connected = false
		return fmt.Errorf("handshake failed after reconnection: %w", err)
	}
	
	gc.logger.Info().Msg("Successfully reconnected and re-established handshake")
	return nil
}

// Listen starts listening for messages from the gateway (device commands)
func (gc *GatewayClient) Listen(messageHandler func(*GatewayMessage) *HubResponse) error {
	if !gc.connected || gc.socket == nil {
		return fmt.Errorf("not connected to gateway")
	}

	gc.logger.Info().Msg("Starting to listen for gateway messages")

	for {
		// Validate socket state before receiving (with read lock)
		gc.mutex.RLock()
		socket := gc.socket
		gc.mutex.RUnlock()
		
		if socket == nil {
			return fmt.Errorf("socket is nil, connection lost")
		}
		
		// Receive message from gateway - DEALER socket gets single-part messages
		msgBytes, err := socket.RecvBytes(0)
		if err != nil {
			gc.logger.Error().Err(err).Msg("Failed to receive message from gateway")
			
			// Attempt to reconnect on communication failure
			if reconnectErr := gc.reconnect(); reconnectErr != nil {
				gc.logger.Error().Err(reconnectErr).Msg("Failed to reconnect to gateway")
				return fmt.Errorf("connection lost and reconnection failed: %w", reconnectErr)
			}
			continue
		}
		
		gc.logger.Debug().
			Int("message_size", len(msgBytes)).
			Str("message_preview", gc.getMessagePreview(msgBytes)).
			Msg("Received message from gateway")

		// Parse as operational GatewayMessage (device commands)
		var gatewayMsg GatewayMessage
		if err := json.Unmarshal(msgBytes, &gatewayMsg); err != nil {
			gc.logger.Error().
				Err(err).
				Int("message_size", len(msgBytes)).
				Str("message_hex", fmt.Sprintf("%x", msgBytes[:min(50, len(msgBytes))])).
				Msg("Failed to parse gateway message")
			
			// Send error response
			errorResponse := &HubResponse{
				ID:        "unknown",
				Nonce:     "", 
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Success:   false,
				Error:     "Failed to parse message",
			}
			gc.sendResponse(errorResponse)
			continue
		}

		// Process operational message through handler
		response := messageHandler(&gatewayMsg)
		if response == nil {
			response = &HubResponse{
				ID:        gatewayMsg.ID,
				Nonce:     gatewayMsg.Nonce,
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Success:   false,
				Error:     "No response from message handler",
			}
		}

		// Ensure response has the correct ID, nonce, and timestamp
		response.ID = gatewayMsg.ID
		response.Nonce = gatewayMsg.Nonce
		if response.Timestamp == "" {
			response.Timestamp = time.Now().UTC().Format(time.RFC3339)
		}

		// Send response back to gateway
		if err := gc.sendResponse(response); err != nil {
			gc.logger.Error().
				Err(err).
				Str("message_id", gatewayMsg.ID).
				Msg("Failed to send response to gateway")
		}
	}
}

// sendResponse sends a response back to the gateway
func (gc *GatewayClient) sendResponse(response *HubResponse) error {
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	gc.logger.Debug().
		Str("message_id", response.ID).
		Bool("success", response.Success).
		Msg("Sending response to gateway")

	// Get socket with read lock for sending
	gc.mutex.RLock()
	socket := gc.socket
	gc.mutex.RUnlock()
	
	if socket == nil {
		return fmt.Errorf("socket is nil, cannot send response")
	}

	_, err = socket.SendBytes(responseJSON, 0)
	if err != nil {
		gc.logger.Warn().Err(err).Msg("Failed to send response, attempting reconnection")
		
		// Attempt to reconnect and retry send
		if reconnectErr := gc.reconnect(); reconnectErr != nil {
			return fmt.Errorf("failed to send response and reconnection failed: %w", err)
		}
		
		// Retry sending after reconnection
		if gc.socket != nil {
			_, retryErr := gc.socket.SendBytes(responseJSON, 0)
			if retryErr != nil {
				return fmt.Errorf("failed to send response after reconnection: %w", retryErr)
			}
			gc.logger.Info().Msg("Response sent successfully after reconnection")
		} else {
			return fmt.Errorf("socket still nil after reconnection")
		}
	}

	return nil
}

// Disconnect closes the connection to the gateway
func (gc *GatewayClient) Disconnect() error {
	gc.mutex.Lock()
	defer gc.mutex.Unlock()
	
	gc.logger.Info().Msg("Disconnecting from gateway")

	// Set disconnected state and clean up socket atomically
	wasConnected := gc.connected
	gc.connected = false
	
	if gc.socket != nil {
		err := gc.socket.Close()
		gc.socket = nil
		
		if err != nil {
			gc.logger.Error().Err(err).Msg("Error while closing socket")
			return fmt.Errorf("failed to close socket: %w", err)
		}
	}

	if wasConnected {
		gc.logger.Info().Msg("Disconnected from gateway")
	} else {
		gc.logger.Debug().Msg("Already disconnected from gateway")
	}
	
	// Reset reconnect delay for future connections
	gc.reconnectDelay = 1 * time.Second
	return nil
}

// IsConnected returns whether the client is connected to the gateway
func (gc *GatewayClient) IsConnected() bool {
	gc.mutex.RLock()
	defer gc.mutex.RUnlock()
	return gc.connected && gc.socket != nil
}

// SendHubOnline sends a message to the gateway indicating the hub is online and waits for response
func (gc *GatewayClient) SendHubOnline() error {
	gc.mutex.RLock()
	connected := gc.connected
	socket := gc.socket
	gc.mutex.RUnlock()
	
	if !connected || socket == nil {
		return fmt.Errorf("not connected to gateway")
	}

	onlineMessage := map[string]interface{}{
		"type":      "hub_online",
		"hub_id":    gc.config.Hub.ID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	onlineJSON, err := json.Marshal(onlineMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal hub online message: %w", err)
	}

	gc.logger.Debug().
		Str("hub_id", gc.config.Hub.ID).
		Int("message_size", len(onlineJSON)).
		Msg("Sending hub_online message")

	_, err = socket.SendBytes(onlineJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send hub online message: %w", err)
	}

	// Wait for device list request (heartbeat)
	response, err := socket.RecvBytes(0)
	if err != nil {
		return fmt.Errorf("failed to receive device list request: %w", err)
	}

	var requestResponse map[string]interface{}
	if err := json.Unmarshal(response, &requestResponse); err != nil {
		gc.logger.Error().
			Err(err).
			Int("response_size", len(response)).
			Str("response_hex", fmt.Sprintf("%x", response[:min(50, len(response))])).
			Msg("Failed to parse device list request")
		return fmt.Errorf("failed to parse device list request: %w", err)
	}

	if msgType, ok := requestResponse["type"].(string); !ok || msgType != "device_list_request" {
		return fmt.Errorf("unexpected response from gateway: %v", requestResponse)
	}

	gc.logger.Info().
		Str("hub_id", gc.config.Hub.ID).
		Msg("Received device list request - gateway heartbeat established")
	return nil
}

// SendDeviceListArray sends the list of devices as array to the gateway and waits for response
func (gc *GatewayClient) SendDeviceListArray(devices []interface{}) error {
	if !gc.connected || gc.socket == nil {
		return fmt.Errorf("not connected to gateway")
	}

	deviceListMessage := map[string]interface{}{
		"type":      "device_list",
		"hub_id":    gc.config.Hub.ID,
		"devices":   devices,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	deviceListJSON, err := json.Marshal(deviceListMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal device list message: %w", err)
	}

	gc.logger.Debug().
		Str("hub_id", gc.config.Hub.ID).
		Int("device_count", len(devices)).
		Int("message_size", len(deviceListJSON)).
		Msg("Sending device_list message")

	_, err = gc.socket.SendBytes(deviceListJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send device list message: %w", err)
	}

	// Wait for devices_registered response
	response, err := gc.socket.RecvBytes(0)
	if err != nil {
		return fmt.Errorf("failed to receive devices_registered response: %w", err)
	}

	var registeredResponse map[string]interface{}
	if err := json.Unmarshal(response, &registeredResponse); err != nil {
		gc.logger.Error().
			Err(err).
			Int("response_size", len(response)).
			Str("response_hex", fmt.Sprintf("%x", response[:min(50, len(response))])).
			Msg("Failed to parse devices_registered response")
		return fmt.Errorf("failed to parse devices_registered response: %w", err)
	}

	if msgType, ok := registeredResponse["type"].(string); !ok || msgType != "devices_registered" {
		return fmt.Errorf("unexpected response from gateway: %v", registeredResponse)
	}

	gc.logger.Info().
		Str("hub_id", gc.config.Hub.ID).
		Int("device_count", len(devices)).
		Msg("Devices registered successfully")
	return nil
}

// Ping sends a ping message to test connectivity and waits for pong
func (gc *GatewayClient) Ping() error {
	if !gc.connected || gc.socket == nil {
		return fmt.Errorf("not connected to gateway")
	}

	pingMessage := map[string]interface{}{
		"type":      "ping",
		"hub_id":    gc.config.Hub.ID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	pingJSON, err := json.Marshal(pingMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal ping message: %w", err)
	}

	_, err = gc.socket.SendBytes(pingJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}

	// Wait for pong response
	response, err := gc.socket.RecvBytes(0)
	if err != nil {
		return fmt.Errorf("failed to receive pong: %w", err)
	}

	var pongResponse map[string]interface{}
	if err := json.Unmarshal(response, &pongResponse); err != nil {
		gc.logger.Error().
			Err(err).
			Int("response_size", len(response)).
			Str("response_hex", fmt.Sprintf("%x", response[:min(50, len(response))])).
			Msg("Failed to parse pong response")
		return fmt.Errorf("failed to parse pong response: %w", err)
	}

	if msgType, ok := pongResponse["type"].(string); !ok || msgType != "pong" {
		return fmt.Errorf("unexpected ping response: %v", pongResponse)
	}

	gc.logger.Debug().Msg("Ping successful")
	return nil
}

// getMessagePreview returns a safe string preview of message data
func (gc *GatewayClient) getMessagePreview(data []byte) string {
	maxLen := 100
	if len(data) > maxLen {
		return string(data[:maxLen]) + "..."
	}
	return string(data)
}

// validateConfig validates that all required configuration for CurveZMQ is present
func (gc *GatewayClient) validateConfig() error {
	if gc.config.Gateway.Endpoint == "" {
		return fmt.Errorf("gateway endpoint is required")
	}
	
	if gc.config.Gateway.PublicKey == "" {
		return fmt.Errorf("gateway public key is required - hub may not be registered yet")
	}
	
	if gc.config.Hub.PublicKey == "" {
		return fmt.Errorf("hub public key is required")
	}
	
	if gc.config.Hub.PrivateKey == "" {
		return fmt.Errorf("hub private key is required")
	}
	
	// Validate key lengths (CurveZMQ keys are 40 characters)
	if len(gc.config.Gateway.PublicKey) != 40 {
		return fmt.Errorf("invalid gateway public key length: expected 40, got %d", len(gc.config.Gateway.PublicKey))
	}
	
	if len(gc.config.Hub.PublicKey) != 40 {
		return fmt.Errorf("invalid hub public key length: expected 40, got %d", len(gc.config.Hub.PublicKey))
	}
	
	if len(gc.config.Hub.PrivateKey) != 40 {
		return fmt.Errorf("invalid hub private key length: expected 40, got %d", len(gc.config.Hub.PrivateKey))
	}
	
	gc.logger.Debug().Msg("Configuration validation passed")
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
