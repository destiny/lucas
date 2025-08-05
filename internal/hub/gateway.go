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

	// Create REQ socket for request-reply pattern
	socket, err := zmq4.NewSocket(zmq4.REQ)
	if err != nil {
		return fmt.Errorf("failed to create ZMQ socket: %w", err)
	}

	// Configure CurveZMQ client
	err = socket.ClientAuthCurve(gc.config.Gateway.PublicKey, gc.config.Hub.PublicKey, gc.config.Hub.PrivateKey)
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to configure CurveZMQ client: %w", err)
	}

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

	// Perform key exchange/registration
	if err := gc.registerHub(); err != nil {
		gc.Disconnect()
		return fmt.Errorf("failed to register with gateway: %w", err)
	}

	return nil
}

// registerHub performs initial registration/key exchange with the gateway
func (gc *GatewayClient) registerHub() error {
	gc.logger.Info().Msg("Registering hub with gateway")

	// Create registration message
	regMessage := map[string]interface{}{
		"type":      "register",
		"hub_id":    "lucas_hub", // Could be configurable
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"public_key": gc.config.Hub.PublicKey,
	}

	// Send registration message
	regJSON, err := json.Marshal(regMessage)
	if err != nil {
		return fmt.Errorf("failed to marshal registration message: %w", err)
	}

	_, err = gc.socket.SendBytes(regJSON, 0)
	if err != nil {
		return fmt.Errorf("failed to send registration message: %w", err)
	}

	// Wait for response
	response, err := gc.socket.RecvBytes(0)
	if err != nil {
		return fmt.Errorf("failed to receive registration response: %w", err)
	}

	// Parse response
	var regResponse map[string]interface{}
	if err := json.Unmarshal(response, &regResponse); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	// Check if registration was successful
	if success, ok := regResponse["success"].(bool); !ok || !success {
		errorMsg := "unknown error"
		if errStr, ok := regResponse["error"].(string); ok {
			errorMsg = errStr
		}
		return fmt.Errorf("registration failed: %s", errorMsg)
	}

	gc.logger.Info().Msg("Hub registration successful")
	return nil
}

// Listen starts listening for messages from the gateway
func (gc *GatewayClient) Listen(messageHandler func(*GatewayMessage) *HubResponse) error {
	if !gc.connected {
		return fmt.Errorf("not connected to gateway")
	}

	gc.logger.Info().Msg("Starting to listen for gateway messages")

	for {
		// Receive message from gateway
		msgBytes, err := gc.socket.RecvBytes(0)
		if err != nil {
			gc.logger.Error().Err(err).Msg("Failed to receive message from gateway")
			continue
		}

		gc.logger.Debug().
			Bytes("message", msgBytes).
			Msg("Received message from gateway")

		// Parse message
		var gatewayMsg GatewayMessage
		if err := json.Unmarshal(msgBytes, &gatewayMsg); err != nil {
			gc.logger.Error().
				Err(err).
				Bytes("message", msgBytes).
				Msg("Failed to parse gateway message")
			
			// Send error response
			errorResponse := &HubResponse{
				ID:        "unknown",
				Nonce:     "", // No nonce available if parsing failed
				Timestamp: time.Now().UTC().Format(time.RFC3339),
				Success:   false,
				Error:     "Failed to parse message",
			}
			gc.sendResponse(errorResponse)
			continue
		}

		// Process message through handler
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

// Ping sends a ping message to test connectivity
func (gc *GatewayClient) Ping() error {
	if !gc.connected {
		return fmt.Errorf("not connected to gateway")
	}

	pingMessage := map[string]interface{}{
		"type":      "ping",
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
		return fmt.Errorf("failed to parse pong response: %w", err)
	}

	if msgType, ok := pongResponse["type"].(string); !ok || msgType != "pong" {
		return fmt.Errorf("unexpected ping response: %v", pongResponse)
	}

	gc.logger.Debug().Msg("Ping successful")
	return nil
}