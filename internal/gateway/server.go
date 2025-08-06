package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pebbe/zmq4"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// HubConnection represents an active connection to a hub
type HubConnection struct {
	HubID     string
	Identity  string // ZMQ identity
	PublicKey string
	LastPing  time.Time
	Status    string
	UserID    int
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
	
	// Ensure socket is cleaned up on any error
	defer func() {
		if err != nil {
			socket.Close()
		}
	}()

	// Configure CurveZMQ server
	err = socket.ServerAuthCurve("*", s.keys.GetServerPrivateKey())
	if err != nil {
		return fmt.Errorf("failed to configure CurveZMQ server: %w", err)
	}
	s.logger.Info().Msg("CurveZMQ server authentication enabled")

	// Set socket options
	if err = socket.SetLinger(1000); err != nil {
		return fmt.Errorf("failed to set linger: %w", err)
	}

	if err = socket.SetRcvhwm(1000); err != nil {
		return fmt.Errorf("failed to set receive high watermark: %w", err)
	}

	if err = socket.SetSndhwm(1000); err != nil {
		return fmt.Errorf("failed to set send high watermark: %w", err)
	}

	// Set receive timeout to match hub timeout (30 seconds) for internet stability
	if err = socket.SetRcvtimeo(30 * time.Second); err != nil {
		return fmt.Errorf("failed to set receive timeout: %w", err)
	}

	// Set send timeout for internet connection reliability
	if err = socket.SetSndtimeo(30 * time.Second); err != nil {
		return fmt.Errorf("failed to set send timeout: %w", err)
	}

	// Bind to address
	if err = socket.Bind(s.address); err != nil {
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
				// Check if it's a timeout (normal for internet connections)
				if err.Error() == "resource temporarily unavailable" {
					// Timeout occurred, continue processing
					continue
				}
				s.logger.Error().Err(err).Msg("Failed to receive message")
			}
			continue
		}

		// Handle both DEALER (2-frame) and REQ (3-frame) message formats
		var identity string
		var messageData []byte

		if len(msg) == 2 {
			// DEALER format: [identity][message]
			identity = string(msg[0])
			messageData = msg[1]
		} else if len(msg) >= 3 {
			// REQ format: [identity][empty delimiter][message]
			identity = string(msg[0])
			messageData = msg[2]
		} else {
			s.logger.Warn().
				Int("parts_count", len(msg)).
				Msg("Received malformed message (invalid frame count)")
			continue
		}

		frameFormat := "DEALER"
		if len(msg) >= 3 {
			frameFormat = "REQ"
		}

		s.logger.Debug().
			Str("identity", identity).
			Str("identity_hex", fmt.Sprintf("%x", msg[0])).
			Int("identity_size", len(msg[0])).
			Int("message_size", len(messageData)).
			Str("message_hex", s.getMessageHex(messageData)).
			Int("parts_count", len(msg)).
			Str("frame_format", frameFormat).
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
	// Normalize hub_id by trimming whitespace
	hubID = strings.TrimSpace(hubID)

	s.logger.Debug().
		Str("identity", identity).
		Str("received_hub_id", hubID).
		Msg("Processing hub_online: received hub_id from message")

	// Update hub status in database
	if err := s.database.UpdateHubStatus(hubID, "online"); err != nil {
		s.logger.Warn().
			Str("hub_id", hubID).
			Err(err).
			Msg("Hub not found in database, attempting auto-registration")
		
		// Try to auto-register the hub
		name := fmt.Sprintf("Auto-registered Hub (%s)", hubID[:8])
		_, regErr := s.database.RegisterHub(hubID, "", name, "")
		if regErr != nil {
			s.logger.Error().
				Str("hub_id", hubID).
				Err(regErr).
				Msg("Failed to auto-register hub")
			return s.createErrorResponse("registration_failed", "Hub not registered and auto-registration failed")
		}
		
		s.logger.Info().
			Str("hub_id", hubID).
			Msg("Successfully auto-registered hub")
		
		// Now update the status
		if err := s.database.UpdateHubStatus(hubID, "online"); err != nil {
			s.logger.Error().
				Str("hub_id", hubID).
				Err(err).
				Msg("Failed to update hub status after auto-registration")
		}
	}

	// Store connection info
	s.mutex.Lock()
	s.connections[identity] = &HubConnection{
		HubID:    hubID,
		Identity: identity,
		LastPing: time.Now(),
		Status:   "online",
	}
	s.mutex.Unlock()
	
	s.logger.Debug().
		Str("identity", identity).
		Str("stored_hub_id", hubID).
		Int("stored_hub_id_len", len(hubID)).
		Str("stored_hub_id_hex", fmt.Sprintf("%x", []byte(hubID))).
		Msg("Stored hub connection in connection map")

	// Send device list request (heartbeat mechanism)
	response := map[string]interface{}{
		"type":         "device_list_request",
		"success":      true,
		"message":      "Please send current device status",
		"request_type": "heartbeat",
		"timestamp":    time.Now().UTC().Format(time.RFC3339),
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
	s.logger.Debug().
		Str("identity", identity).
		Str("conn_hub_id", conn.HubID).
		Int("conn_hub_id_len", len(conn.HubID)).
		Str("conn_hub_id_hex", fmt.Sprintf("%x", []byte(conn.HubID))).
		Msg("Looking up hub in database for device list processing")
		
	hub, err := s.database.GetHubByHubID(conn.HubID)
	if err != nil {
		s.logger.Error().
			Str("identity", identity).
			Str("conn_hub_id", conn.HubID).
			Err(err).
			Msg("Failed to get hub from database for device list")
		return s.createErrorResponse("database_error", "Failed to get hub")
	}
	
	s.logger.Debug().
		Str("identity", identity).
		Str("found_hub_id", hub.HubID).
		Int("hub_db_id", hub.ID).
		Msg("Successfully found hub in database")

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
				Str("device_id", deviceID).
				Str("device_type", deviceType).
				Str("name", name).
				Msg("Skipping device with missing required fields")
			continue
		}

		// Create or update device
		device, err := s.database.CreateDevice(hub.ID, deviceID, deviceType, name, model, address, capabilities)
		if err != nil {
			s.logger.Error().
				Str("hub_id", conn.HubID).
				Str("device_id", deviceID).
				Str("device_type", deviceType).
				Str("name", name).
				Err(err).
				Msg("Failed to create/update device")
		} else {
			s.logger.Info().
				Str("hub_id", conn.HubID).
				Str("device_id", deviceID).
				Str("device_type", deviceType).
				Str("name", name).
				Str("model", model).
				Str("address", address).
				Int("db_device_id", device.ID).
				Msg("Successfully registered/updated device")
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
	if s.socket == nil || !s.running {
		return fmt.Errorf("socket not initialized or server not running")
	}

	// ROUTER must send: [identity][response] to match DEALER socket expectations (2 frames)
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
	timedOutConnections := make([]string, 0)
	
	for identity, conn := range s.connections {
		if now.Sub(conn.LastPing) > timeout {
			s.logger.Warn().
				Str("identity", identity).
				Str("hub_id", conn.HubID).
				Dur("last_ping_age", now.Sub(conn.LastPing)).
				Msg("Hub connection timed out")

			timedOutConnections = append(timedOutConnections, identity)
		}
	}
	
	// Remove timed out connections and update database
	for _, identity := range timedOutConnections {
		conn := s.connections[identity]
		
		// Update database status
		if err := s.database.UpdateHubStatus(conn.HubID, "offline"); err != nil {
			s.logger.Error().
				Err(err).
				Str("hub_id", conn.HubID).
				Msg("Failed to update hub status to offline")
		}

		// Remove from active connections
		delete(s.connections, identity)
		
		s.logger.Info().
			Str("identity", identity).
			Str("hub_id", conn.HubID).
			Msg("Removed timed out hub connection")
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
	if s.socket == nil || !s.running {
		return fmt.Errorf("server not running or socket not initialized")
	}
	
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
