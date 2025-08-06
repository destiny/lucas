package hub

import (
	"encoding/json"
	"fmt"
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
	socket    *zmq4.Socket
	config    *Config
	logger    zerolog.Logger
	connected bool
}

// NewGatewayClient creates a new gateway client
func NewGatewayClient(config *Config) *GatewayClient {
	return &GatewayClient{
		config: config,
		logger: logger.New(),
	}
}

// Connect establishes connection to the gateway with CurveZMQ encryption
func (gc *GatewayClient) Connect() error {
	gc.logger.Info().
		Str("endpoint", gc.config.Gateway.Endpoint).
		Msg("Connecting to gateway")

	// Create REQ socket for synchronous request-reply with ROUTER
	socket, err := zmq4.NewSocket(zmq4.REQ)
	if err != nil {
		return fmt.Errorf("failed to create ZMQ socket: %w", err)
	}

	// Configure CurveZMQ client (temporarily disabled for testing)
	// TODO: Re-enable after basic communication is verified
	/*
	err = socket.ClientAuthCurve(gc.config.Gateway.PublicKey, gc.config.Hub.PublicKey, gc.config.Hub.PrivateKey)
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to configure CurveZMQ client: %w", err)
	}
	*/
	gc.logger.Info().Msg("CurveZMQ disabled for testing - using plain socket")

	// Set socket options
	err = socket.SetLinger(1000) // 1 second linger time
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to set socket linger: %w", err)
	}

	err = socket.SetRcvtimeo(30 * time.Second) // 30 second receive timeout
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to set receive timeout: %w", err)
	}

	err = socket.SetSndtimeo(30 * time.Second) // 30 second send timeout
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to set send timeout: %w", err)
	}

	// Connect to gateway
	err = socket.Connect(gc.config.Gateway.Endpoint)
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to connect to gateway: %w", err)
	}

	gc.socket = socket
	gc.connected = true

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
	// Convert devices from config to interface map for JSON serialization
	devices := make(map[string]interface{})
	for _, device := range gc.config.Devices {
		devices[device.ID] = map[string]interface{}{
			"id":           device.ID,
			"type":         device.Type,
			"model":        device.Model,
			"address":      device.Address,
			"capabilities": device.Capabilities,
		}
	}

	return gc.SendDeviceList(devices)
}

// Listen starts listening for messages from the gateway (device commands)
func (gc *GatewayClient) Listen(messageHandler func(*GatewayMessage) *HubResponse) error {
	if !gc.connected {
		return fmt.Errorf("not connected to gateway")
	}

	gc.logger.Info().Msg("Starting to listen for gateway messages")

	for {
		// Receive message from gateway - REQ socket gets single-part messages
		msgBytes, err := gc.socket.RecvBytes(0)
		if err != nil {
			gc.logger.Error().Err(err).Msg("Failed to receive message from gateway")
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

	_, err = gc.socket.SendBytes(responseJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send response: %w", err)
	}

	return nil
}

// Disconnect closes the connection to the gateway
func (gc *GatewayClient) Disconnect() error {
	if !gc.connected {
		return nil
	}

	gc.logger.Info().Msg("Disconnecting from gateway")

	if gc.socket != nil {
		err := gc.socket.Close()
		gc.socket = nil
		gc.connected = false
		
		if err != nil {
			return fmt.Errorf("failed to close socket: %w", err)
		}
	}

	gc.logger.Info().Msg("Disconnected from gateway")
	return nil
}

// IsConnected returns whether the client is connected to the gateway
func (gc *GatewayClient) IsConnected() bool {
	return gc.connected
}

// SendHubOnline sends a message to the gateway indicating the hub is online and waits for response
func (gc *GatewayClient) SendHubOnline() error {
	if !gc.connected {
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

	_, err = gc.socket.SendBytes(onlineJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send hub online message: %w", err)
	}

	// Wait for gateway_ready response
	response, err := gc.socket.RecvBytes(0)
	if err != nil {
		return fmt.Errorf("failed to receive gateway_ready response: %w", err)
	}

	var readyResponse map[string]interface{}
	if err := json.Unmarshal(response, &readyResponse); err != nil {
		gc.logger.Error().
			Err(err).
			Int("response_size", len(response)).
			Str("response_hex", fmt.Sprintf("%x", response[:min(50, len(response))])).
			Msg("Failed to parse gateway_ready response")
		return fmt.Errorf("failed to parse gateway_ready response: %w", err)
	}

	if msgType, ok := readyResponse["type"].(string); !ok || msgType != "gateway_ready" {
		return fmt.Errorf("unexpected response from gateway: %v", readyResponse)
	}

	gc.logger.Info().
		Str("hub_id", gc.config.Hub.ID).
		Msg("Gateway is ready")
	return nil
}

// SendDeviceList sends the list of devices to the gateway and waits for response
func (gc *GatewayClient) SendDeviceList(devices map[string]interface{}) error {
	if !gc.connected {
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
	if !gc.connected {
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

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
