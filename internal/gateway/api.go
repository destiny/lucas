package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// APIServer handles REST API requests
type APIServer struct {
	database      *Database
	brokerService *BrokerService
	keys          *GatewayKeys
	logger        zerolog.Logger
	server        *http.Server
}

// NewAPIServer creates a new API server
func NewAPIServer(database *Database, brokerService *BrokerService, keys *GatewayKeys) *APIServer {
	return &APIServer{
		database:      database,
		brokerService: brokerService,
		keys:          keys,
		logger:        logger.New(),
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

	// Hub registration endpoint
	apiRouter.HandleFunc("/hub/register", api.handleHubRegister).Methods("POST")
	
	// Hub claiming endpoint
	apiRouter.HandleFunc("/hub/claim", api.handleHubClaim).Methods("POST")

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
	apiRouter.HandleFunc("/health", api.handleHealth).Methods("GET")

	// Setup web app serving (must be last to catch all non-API routes)
	api.SetupWebApp(router)

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
	stats := api.brokerService.GetServiceStats()
	
	status := map[string]interface{}{
		"status":            "running",
		"active_hubs":       getActiveHubCount(stats),
		"uptime":           "N/A", // Could add uptime tracking
		"version":          "1.0.0",
		"timestamp":        time.Now().UTC().Format(time.RFC3339),
		"service_stats":    stats,
	}

	api.sendJSON(w, http.StatusOK, status)
}

func (api *APIServer) handleKeyInfo(w http.ResponseWriter, r *http.Request) {
	// This would normally require authentication
	keyInfo := map[string]interface{}{
		"public_key": api.keys.GetServerPublicKey(),
		"key_type":   "curve25519",
		"algorithm":  "CurveZMQ",
	}

	api.sendJSON(w, http.StatusOK, keyInfo)
}

func (api *APIServer) handleConnections(w http.ResponseWriter, r *http.Request) {
	stats := api.brokerService.GetServiceStats()
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"service_stats": stats,
		"active_hubs":   getActiveHubCount(stats),
	})
}

func (api *APIServer) handleHubRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		HubID      string `json:"hub_id"`
		PublicKey  string `json:"public_key"`
		Name       string `json:"name"`
		ProductKey string `json:"product_key"`
		Timestamp  string `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate required fields
	if req.HubID == "" {
		api.sendError(w, http.StatusBadRequest, "Hub ID is required")
		return
	}
	if req.PublicKey == "" {
		api.sendError(w, http.StatusBadRequest, "Public key is required")
		return
	}

	// Validate public key format (CurveZMQ keys are 40 characters)
	if len(req.PublicKey) != 40 {
		api.sendError(w, http.StatusBadRequest, "Invalid public key format")
		return
	}

	// Register hub in database
	hub, err := api.database.RegisterHub(req.HubID, req.PublicKey, req.Name, req.ProductKey)
	if err != nil {
		api.logger.Error().
			Str("hub_id", req.HubID).
			Err(err).
			Msg("Failed to register hub")
		api.sendError(w, http.StatusInternalServerError, "Failed to register hub")
		return
	}

	// Return success response with gateway information
	response := map[string]interface{}{
		"success":         true,
		"message":         "Hub registered successfully",
		"hub":             hub,
		"gateway_info": map[string]interface{}{
			"public_key":    api.keys.GetServerPublicKey(),
			"zmq_endpoint":  "tcp://localhost:5555", // Should be configurable
			"api_endpoint":  r.Host,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusCreated, response)
}

func (api *APIServer) handleHubClaim(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID     int    `json:"user_id"`
		ProductKey string `json:"product_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate required fields
	if req.UserID == 0 {
		api.sendError(w, http.StatusBadRequest, "User ID is required")
		return
	}
	if req.ProductKey == "" {
		api.sendError(w, http.StatusBadRequest, "Product key is required")
		return
	}

	// Find hub by product key
	hub, err := api.database.GetHubByProductKey(req.ProductKey)
	if err != nil {
		api.sendError(w, http.StatusNotFound, "Hub not found with provided product key")
		return
	}

	// Check if hub is already claimed
	if hub.UserID.Valid && hub.UserID.Int32 != 0 && !hub.AutoRegistered {
		api.sendError(w, http.StatusConflict, "Hub is already claimed by another user")
		return
	}

	// Verify user exists
	user, err := api.database.GetUser(req.UserID)
	if err != nil {
		api.sendError(w, http.StatusNotFound, "User not found")
		return
	}

	// Claim the hub - update user_id and set auto_registered to false
	if err := api.database.ClaimHub(hub.HubID, req.UserID); err != nil {
		api.logger.Error().
			Str("hub_id", hub.HubID).
			Int("user_id", req.UserID).
			Err(err).
			Msg("Failed to claim hub")
		api.sendError(w, http.StatusInternalServerError, "Failed to claim hub")
		return
	}

	// Update devices to link to the user
	if err := api.database.UpdateDevicesUserID(hub.ID, req.UserID); err != nil {
		api.logger.Error().
			Str("hub_id", hub.HubID).
			Int("user_id", req.UserID).
			Err(err).
			Msg("Failed to update device ownership")
		// Don't fail the request, just log the warning
	}

	api.logger.Info().
		Str("hub_id", hub.HubID).
		Int("user_id", req.UserID).
		Str("username", user.Username).
		Msg("Hub claimed successfully")

	response := map[string]interface{}{
		"success":   true,
		"message":   "Hub claimed successfully",
		"hub_id":    hub.HubID,
		"user":      user.Username,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusOK, response)
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
	if !deviceHub.UserID.Valid || int(deviceHub.UserID.Int32) != userID {
		api.sendError(w, http.StatusForbidden, "Device not accessible by user")
		return
	}

	// Create device action using BrokerService
	deviceAction := json.RawMessage(fmt.Sprintf(`{"type":"%s","action":"%s","parameters":%s}`, 
		actionReq.Type, actionReq.Action, mustMarshal(actionReq.Parameters)))

	// Send device command via Hermes BrokerService
	response, err := api.brokerService.SendDeviceCommand(deviceHub.HubID, deviceID, deviceAction)
	if err != nil {
		api.logger.Error().
			Str("hub_id", deviceHub.HubID).
			Str("device_id", deviceID).
			Err(err).
			Msg("Failed to send device command via broker service")
		api.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to send command to device: %v", err))
		return
	}

	// Return response from BrokerService
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"success":   true,
		"message":   "Device command executed successfully",
		"response":  response,
		"device":    device,
		"hub":       deviceHub.HubID,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
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
	stats := api.brokerService.GetServiceStats()
	activeHubs := getActiveHubCount(stats)
	api.sendJSON(w, http.StatusOK, map[string]interface{}{
		"active_hubs":   activeHubs,
		"count":         activeHubs,
		"service_stats": stats,
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
			"database":       "healthy",
			"broker_service": "healthy",
		},
	}

	api.sendJSON(w, http.StatusOK, health)
}

// Helper functions
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

// getActiveHubCount extracts the number of active hubs from service stats
func getActiveHubCount(stats map[string]interface{}) int {
	if workers, ok := stats["workers"].(map[string]interface{}); ok {
		return len(workers)
	}
	if brokerStats, ok := stats["broker"].(map[string]interface{}); ok {
		if workers, ok := brokerStats["workers"].(int); ok {
			return workers
		}
	}
	return 0
}