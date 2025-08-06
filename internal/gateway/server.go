package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// HubConnection represents an active connection to a hub
type HubConnection struct {
	HubID      string
	Identity   string // ZMQ identity
	PublicKey  string
	LastPing   time.Time
	Status     string
	UserID     int
}

// ZMQServer handles ZMQ ROUTER communication with hubs
type ZMQServer struct {
	socket      *zmq4.Socket
	address     string
	keys        *GatewayKeys
	database    *Database
	logger      zerolog.Logger
	connections map[string]*HubConnection // identity -> connection
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	running     bool
}

// NewZMQServer creates a new ZMQ server for hub communication
func NewZMQServer(address string, keys *GatewayKeys, database *Database) *ZMQServer {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &ZMQServer{
		address:     address,
		keys:        keys,
		database:    database,
		logger:      logger.New(),
		connections: make(map[string]*HubConnection),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// Start starts the ZMQ server
func (s *ZMQServer) Start() error {
	s.logger.Info().
		Str("address", s.address).
		Msg("Starting ZMQ server for hub connections")

	// Create ROUTER socket
	socket, err := zmq4.NewSocket(zmq4.ROUTER)
	if err != nil {
		return fmt.Errorf("failed to create ROUTER socket: %w", err)
	}

	// Configure CurveZMQ server (temporarily disabled for testing)
	// TODO: Re-enable after basic communication is verified
	/*
	err = socket.ServerAuthCurve("*", s.keys.GetServerPrivateKey())
	if err != nil {
		socket.Close()
		return fmt.Errorf("failed to configure CurveZMQ server: %w", err)
	}
	*/
	s.logger.Info().Msg("CurveZMQ disabled for testing - using plain socket")

	// Set socket options
	if err := socket.SetLinger(1000); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set linger: %w", err)
	}

	if err := socket.SetRcvhwm(1000); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set receive high watermark: %w", err)
	}

	if err := socket.SetSndhwm(1000); err != nil {
		socket.Close()
		return fmt.Errorf("failed to set send high watermark: %w", err)
	}

	// Bind to address
	if err := socket.Bind(s.address); err != nil {
		socket.Close()
		return fmt.Errorf("failed to bind to address: %w", err)
	}

	s.socket = socket
	s.running = true

	s.logger.Info().Msg("ZMQ server started successfully")

	// Start message processing loop
	go s.messageLoop()

	// Start connection monitoring
	go s.monitorConnections()

	return nil
}

// Stop stops the ZMQ server
func (s *ZMQServer) Stop() error {
	if !s.running {
		return nil
	}

	s.logger.Info().Msg("Stopping ZMQ server")
	
	s.running = false
	s.cancel()

	if s.socket != nil {
		if err := s.socket.Close(); err != nil {
			s.logger.Error().Err(err).Msg("Error closing ZMQ socket")
		}
		s.socket = nil
	}

	s.logger.Info().Msg("ZMQ server stopped")
	return nil
}

// messageLoop processes incoming messages from hubs
func (s *ZMQServer) messageLoop() {
	s.logger.Info().Msg("Starting message processing loop")

	for s.running {
		// Receive multipart message
		msg, err := s.socket.RecvMessageBytes(0)
		if err != nil {
			if s.running {
				s.logger.Error().Err(err).Msg("Failed to receive message")
			}
			continue
		}

		if len(msg) < 2 {
			s.logger.Warn().
				Int("parts_count", len(msg)).
				Msg("Received malformed message (too few parts)")
			continue
		}

		// First part is the identity, second part is the actual message
		identity := string(msg[0])
		messageData := msg[1]

		s.logger.Debug().
			Str("identity", identity).
			Str("identity_hex", fmt.Sprintf("%x", msg[0])).
			Int("identity_size", len(msg[0])).
			Int("message_size", len(messageData)).
			Str("message_hex", s.getMessageHex(messageData)).
			Int("parts_count", len(msg)).
			Msg("Received message from hub")

		// Log all message parts for debugging
		for i, part := range msg {
			s.logger.Debug().
				Int("part_index", i).
				Int("part_size", len(part)).
				Str("part_hex", fmt.Sprintf("%x", part[:min(20, len(part))])).
				Str("part_preview", s.getMessagePreview(part)).
				Msg("Message part details")
		}

		// Add validation for empty message data
		if len(messageData) == 0 {
			s.logger.Warn().
				Str("identity", identity).
				Msg("Received empty message data")
			response := s.createErrorResponse("empty_message", "Message data is empty")
			s.sendResponse(identity, response)
			continue
		}

		// Process message
		response := s.processMessage(identity, messageData)
		
		// Send response back to hub
		if err := s.sendResponse(identity, response); err != nil {
			s.logger.Error().
				Str("identity", identity).
				Err(err).
				Msg("Failed to send response to hub")
		}
	}

	s.logger.Info().Msg("Message processing loop stopped")
}

// processMessage processes a message from a hub
func (s *ZMQServer) processMessage(identity string, messageData []byte) []byte {
	// Parse message
	var message map[string]interface{}
	if err := json.Unmarshal(messageData, &message); err != nil {
		s.logger.Error().
			Str("identity", identity).
			Err(err).
			Int("message_length", len(messageData)).
			Str("message_preview", s.getMessagePreview(messageData)).
			Str("message_hex", s.getMessageHex(messageData)).
			Msg("Failed to parse message JSON")
		return s.createErrorResponse("invalid_json", "Failed to parse message")
	}

	// Check message type
	msgType, ok := message["type"].(string)
	if !ok {
		s.logger.Error().
			Str("identity", identity).
			Msg("Message missing type field")
		return s.createErrorResponse("missing_type", "Message must have a type field")
	}

	s.logger.Info().
		Str("identity", identity).
		Str("type", msgType).
		Msg("Processing hub message")

	// Route message based on type
	switch msgType {
	case "hub_online":
		return s.handleHubOnline(identity, message)
	case "device_list":
		return s.handleDeviceList(identity, message)
	case "ping":
		return s.handlePing(identity, message)
	default:
		s.logger.Warn().
			Str("identity", identity).
			Str("type", msgType).
			Msg("Unknown message type")
		return s.createErrorResponse("unknown_type", fmt.Sprintf("Unknown message type: %s", msgType))
	}
}

// handleHubOnline handles the hub online message
func (s *ZMQServer) handleHubOnline(identity string, message map[string]interface{}) []byte {
	s.logger.Info().
		Str("identity", identity).
		Msg("Processing hub online message")

	// Extract hub_id
	hubID, ok := message["hub_id"].(string)
	if !ok {
		return s.createErrorResponse("missing_hub_id", "Hub online message must include hub_id")
	}

	// Update hub status in database
	if err := s.database.UpdateHubStatus(hubID, "online"); err != nil {
		s.logger.Error().
			Str("hub_id", hubID).
			Err(err).
			Msg("Failed to update hub status")
	}

	// Store connection info
	s.mutex.Lock()
	s.connections[identity] = &HubConnection{
		HubID:     hubID,
		Identity:  identity,
		LastPing:  time.Now(),
		Status:    "online",
	}
	s.mutex.Unlock()

	// Send gateway ready message
	response := map[string]interface{}{
		"type":      "gateway_ready",
		"success":   true,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// handleDeviceList handles the device list message from a hub
func (s *ZMQServer) handleDeviceList(identity string, message map[string]interface{}) []byte {
	s.mutex.RLock()
	conn, exists := s.connections[identity]
	s.mutex.RUnlock()

	if !exists {
		return s.createErrorResponse("not_registered", "Hub not registered")
	}

	devices, ok := message["devices"].([]interface{})
	if !ok {
		return s.createErrorResponse("invalid_devices", "Devices must be an array")
	}

	// Get hub from database
	hub, err := s.database.GetHubByHubID(conn.HubID)
	if err != nil {
		s.logger.Error().
			Str("hub_id", conn.HubID).
			Err(err).
			Msg("Failed to get hub from database")
		return s.createErrorResponse("database_error", "Failed to get hub")
	}

	// Process each device
	for _, deviceData := range devices {
		deviceMap, ok := deviceData.(map[string]interface{})
		if !ok {
			continue
		}

		deviceID, _ := deviceMap["id"].(string)
		deviceType, _ := deviceMap["type"].(string)
		name, _ := deviceMap["name"].(string)
		model, _ := deviceMap["model"].(string)
		address, _ := deviceMap["address"].(string)

		capabilities := []string{}
		if caps, ok := deviceMap["capabilities"].([]interface{}); ok {
			for _, cap := range caps {
				if capStr, ok := cap.(string); ok {
					capabilities = append(capabilities, capStr)
				}
			}
		}

		if deviceID == "" || deviceType == "" || name == "" {
			s.logger.Warn().
				Str("hub_id", conn.HubID).
				Msg("Skipping device with missing required fields")
			continue
		}

		// Create or update device
		_, err := s.database.CreateDevice(hub.ID, deviceID, deviceType, name, model, address, capabilities)
		if err != nil {
			s.logger.Error().
				Str("hub_id", conn.HubID).
				Str("device_id", deviceID).
				Err(err).
				Msg("Failed to create/update device")
		} else {
			s.logger.Info().
				Str("hub_id", conn.HubID).
				Str("device_id", deviceID).
				Msg("Registered device")
		}
	}

	response := map[string]interface{}{
		"type":         "devices_registered",
		"success":      true,
		"device_count": len(devices),
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// handlePing handles ping messages from hubs
func (s *ZMQServer) handlePing(identity string, message map[string]interface{}) []byte {
	s.mutex.Lock()
	if conn, exists := s.connections[identity]; exists {
		conn.LastPing = time.Now()
		s.database.UpdateHubStatus(conn.HubID, "online")
	}
	s.mutex.Unlock()

	response := map[string]interface{}{
		"type":      "pong",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// sendResponse sends a response back to a hub
func (s *ZMQServer) sendResponse(identity string, response []byte) error {
	if s.socket == nil {
		return fmt.Errorf("socket not initialized")
	}

	_, err := s.socket.SendMessage(identity, response)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	s.logger.Debug().
		Str("identity", identity).
		Int("response_size", len(response)).
		Msg("Sent response to hub")

	return nil
}

// createErrorResponse creates a JSON error response
func (s *ZMQServer) createErrorResponse(errorCode, errorMessage string) []byte {
	response := map[string]interface{}{
		"type":      "error",
		"success":   false,
		"error":     errorCode,
		"message":   errorMessage,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	responseJSON, _ := json.Marshal(response)
	return responseJSON
}

// monitorConnections monitors hub connections and handles timeouts
func (s *ZMQServer) monitorConnections() {
	ticker := time.NewTicker(30 * time.Second) // Check every 30 seconds
	defer ticker.Stop()

	s.logger.Info().Msg("Starting connection monitor")

	for {
		select {
		case <-ticker.C:
			s.checkConnectionTimeouts()
		case <-s.ctx.Done():
			s.logger.Info().Msg("Connection monitor stopping")
			return
		}
	}
}

// checkConnectionTimeouts checks for timed out connections
func (s *ZMQServer) checkConnectionTimeouts() {
	timeout := 2 * time.Minute // 2 minute timeout
	now := time.Now()

	s.mutex.Lock()
	for identity, conn := range s.connections {
		if now.Sub(conn.LastPing) > timeout {
			s.logger.Warn().
				Str("identity", identity).
				Str("hub_id", conn.HubID).
				Msg("Hub connection timed out")
			
			// Update database status
			s.database.UpdateHubStatus(conn.HubID, "offline")
			
			// Remove from active connections
			delete(s.connections, identity)
		}
	}
	s.mutex.Unlock()
}

// GetActiveConnections returns information about active hub connections
func (s *ZMQServer) GetActiveConnections() map[string]*HubConnection {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Return a copy to prevent external modification
	connections := make(map[string]*HubConnection)
	for identity, conn := range s.connections {
		connCopy := *conn
		connections[identity] = &connCopy
	}

	return connections
}

// SendMessageToHub sends a message to a specific hub
func (s *ZMQServer) SendMessageToHub(hubID string, message []byte) error {
	s.mutex.RLock()
	var targetIdentity string
	for identity, conn := range s.connections {
		if conn.HubID == hubID {
			targetIdentity = identity
			break
		}
	}
	s.mutex.RUnlock()

	if targetIdentity == "" {
		return fmt.Errorf("hub %s not connected", hubID)
	}

	return s.sendResponse(targetIdentity, message)
}

// IsHubConnected checks if a hub is currently connected
func (s *ZMQServer) IsHubConnected(hubID string) bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	for _, conn := range s.connections {
		if conn.HubID == hubID {
			return true
		}
	}
	return false
}

// GetConnectionCount returns the number of active connections
func (s *ZMQServer) GetConnectionCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.connections)
}

// getMessagePreview returns a safe string preview of message data
func (s *ZMQServer) getMessagePreview(data []byte) string {
	maxLen := 100
	if len(data) > maxLen {
		return string(data[:maxLen]) + "..."
	}
	return string(data)
}

// getMessageHex returns hex dump of first 50 bytes for debugging
func (s *ZMQServer) getMessageHex(data []byte) string {
	maxLen := 50
	if len(data) > maxLen {
		return fmt.Sprintf("%x...", data[:maxLen])
	}
	return fmt.Sprintf("%x", data)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

