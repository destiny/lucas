package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"lucas/internal/hub"
	"lucas/internal/logger"
)

// APIServer handles REST API requests
type APIServer struct {
	database  *Database
	zmqServer *ZMQServer
	logger    zerolog.Logger
	server    *http.Server
}

// NewAPIServer creates a new API server
func NewAPIServer(database *Database, zmqServer *ZMQServer) *APIServer {
	return &APIServer{
		database:  database,
		zmqServer: zmqServer,
		logger:    logger.New(),
	}
}

// Start starts the HTTP API server
func (api *APIServer) Start(address string) error {
	router := mux.NewRouter()

	// Add middleware
	router.Use(api.loggingMiddleware)
	router.Use(api.corsMiddleware)

	// API routes
	apiRouter := router.PathPrefix("/api/v1").Subrouter()
	
	// Gateway endpoints
	apiRouter.HandleFunc("/gateway/status", api.handleGatewayStatus).Methods("GET")
	apiRouter.HandleFunc("/gateway/keys/info", api.handleKeyInfo).Methods("GET")
	apiRouter.HandleFunc("/gateway/connections", api.handleConnections).Methods("GET")

	// User endpoints
	apiRouter.HandleFunc("/users", api.handleCreateUser).Methods("POST")
	apiRouter.HandleFunc("/users/{user_id}/hubs", api.handleGetUserHubs).Methods("GET")
	apiRouter.HandleFunc("/users/{user_id}/devices", api.handleGetUserDevices).Methods("GET")
	apiRouter.HandleFunc("/users/{user_id}/devices/{device_id}/action", api.handleDeviceAction).Methods("POST")

	// Admin endpoints (no auth for demo)
	apiRouter.HandleFunc("/admin/users", api.handleListUsers).Methods("GET")
	apiRouter.HandleFunc("/admin/hubs", api.handleListHubs).Methods("GET")
	apiRouter.HandleFunc("/admin/devices", api.handleListDevices).Methods("GET")

	// Health check
	router.HandleFunc("/health", api.handleHealth).Methods("GET")

	api.server = &http.Server{
		Addr:         address,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	api.logger.Info().
		Str("address", address).
		Msg("Starting API server")

	return api.server.ListenAndServe()
}

// Stop stops the API server
func (api *APIServer) Stop() error {
	if api.server != nil {
		return api.server.Close()
	}
	return nil
}

// Middleware
func (api *APIServer) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		api.logger.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Dur("duration", time.Since(start)).
			Msg("API request")
	})
}

func (api *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Response helpers
func (api *APIServer) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (api *APIServer) sendError(w http.ResponseWriter, status int, message string) {
	api.sendJSON(w, status, map[string]interface{}{
		"error":     true,
		"message":   message,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Gateway endpoints
func (api *APIServer) handleGatewayStatus(w http.ResponseWriter, r *http.Request) {
	connections := api.zmqServer.GetActiveConnections()
	
	status := map[string]interface{}{
		"status":            "running",
		"active_hubs":       len(connections),
		"uptime":           "N/A", // Could add uptime tracking
		"version":          "1.0.0",
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"hub_connections":  connections,
	}

	api.sendJSON(w, http.StatusOK, status)
}

func (api *APIServer) handleKeyInfo(w http.ResponseWriter, r *http.Request) {
	// This would normally require authentication
	keyInfo := map[string]interface{}{
		"public_key": "gateway_public_key_here", // Would get from actual keys
		"key_type":   "curve25519",
		"algorithm":  "CurveZMQ",
	}

	api.sendJSON(w, http.StatusOK, keyInfo)
}

func (api *APIServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	connections := api.zmqServer.GetActiveConnections()
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"connections": connections,
		"count":       len(connections),
	})
}

// User endpoints
func (api *APIServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	if req.Username == "" {
		api.sendError(w, http.StatusBadRequest, "Username is required")
		return
	}

	user, err := api.database.CreateUser(req.Username, req.Email)
	if err != nil {
		api.logger.Error().Err(err).Msg("Failed to create user")
		api.sendError(w, http.StatusInternalServerError, "Failed to create user")
		return
	}

	api.sendJSON(w, http.StatusCreated, user)
}

func (api *APIServer) handleGetUserHubs(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["user_id"])
	if err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	hubs, err := api.database.GetUserHubs(userID)
	if err != nil {
		api.logger.Error().Err(err).Msg("Failed to get user hubs")
		api.sendError(w, http.StatusInternalServerError, "Failed to get hubs")
		return
	}

	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"hubs":  hubs,
		"count": len(hubs),
	})
}

func (api *APIServer) handleGetUserDevices(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["user_id"])
	if err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	devices, err := api.database.GetUserDevices(userID)
	if err != nil {
		api.logger.Error().Err(err).Msg("Failed to get user devices")
		api.sendError(w, http.StatusInternalServerError, "Failed to get devices")
		return
	}

	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"devices": devices,
		"count":   len(devices),
	})
}

func (api *APIServer) handleDeviceAction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, err := strconv.Atoi(vars["user_id"])
	if err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}
	deviceID := vars["device_id"]

	// Parse action request
	var actionReq struct {
		Type       string                 `json:"type"`
		Action     string                 `json:"action"`
		Parameters map[string]interface{} `json:"parameters"`
	}

	if err := json.NewDecoder(r.Body).Decode(&actionReq); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Find device and its hub
	device, deviceHub, err := api.database.FindDeviceByID(deviceID)
	if err != nil {
		api.sendError(w, http.StatusNotFound, "Device not found")
		return
	}

	// Verify device belongs to user
	if deviceHub.UserID != userID {
		api.sendError(w, http.StatusForbidden, "Device not accessible by user")
		return
	}

	// Check if hub is connected
	if !api.zmqServer.IsHubConnected(deviceHub.HubID) {
		api.sendError(w, http.StatusServiceUnavailable, "Hub not connected")
		return
	}

	// Create gateway message for hub
	gatewayMessage := hub.GatewayMessage{
		ID:        fmt.Sprintf("api-%d", time.Now().UnixNano()),
		Nonce:     hub.GenerateNonce(),
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		DeviceID:  deviceID,
		Action:    json.RawMessage(fmt.Sprintf(`{"type":"%s","action":"%s","parameters":%s}`, 
			actionReq.Type, actionReq.Action, mustMarshal(actionReq.Parameters))),
	}

	// Send message to hub
	messageJSON, err := json.Marshal(gatewayMessage)
	if err != nil {
		api.sendError(w, http.StatusInternalServerError, "Failed to create message")
		return
	}

	if err := api.zmqServer.SendMessageToHub(deviceHub.HubID, messageJSON); err != nil {
		api.logger.Error().
			Str("hub_id", deviceHub.HubID).
			Str("device_id", deviceID).
			Err(err).
			Msg("Failed to send message to hub")
		api.sendError(w, http.StatusInternalServerError, "Failed to send command to device")
		return
	}

	// For now, return immediate response (in production, might wait for hub response)
	api.sendJSON(w, http.StatusAccepted, map[string]interface{}{
		"success":    true,
		"message":    "Command sent to device",
		"message_id": gatewayMessage.ID,
		"nonce":      gatewayMessage.Nonce,
		"device":     device,
		"hub":        deviceHub.HubID,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

// Admin endpoints
func (api *APIServer) handleListUsers(w http.ResponseWriter, r *http.Request) {
	// This would normally require admin authentication
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Admin endpoint - would list all users",
	})
}

func (api *APIServer) handleListHubs(w http.ResponseWriter, r *http.Request) {
	// This would normally require admin authentication
	connections := api.zmqServer.GetActiveConnections()
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"active_hubs": connections,
		"count":       len(connections),
	})
}

func (api *APIServer) handleListDevices(w http.ResponseWriter, r *http.Request) {
	// This would normally require admin authentication
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"message": "Admin endpoint - would list all devices",
	})
}

// Health check
func (api *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":     "healthy",
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
		"components": map[string]string{
			"database":   "healthy",
			"zmq_server": "healthy",
		},
	}

	api.sendJSON(w, http.StatusOK, health)
}

// Helper function
func mustMarshal(v interface{}) string {
	if v == nil {
		return "{}"
	}
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}