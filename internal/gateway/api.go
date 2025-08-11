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

package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// APIServer handles REST API requests
type APIServer struct {
	database        *Database
	brokerService   *BrokerService
	keys            *GatewayKeys
	logger          zerolog.Logger
	server          *http.Server
	jwtService      *JWTService
	passwordService *PasswordService
	authMiddleware  *AuthMiddleware
}

// NewAPIServer creates a new API server
func NewAPIServer(database *Database, brokerService *BrokerService, keys *GatewayKeys, config *GatewayConfig) *APIServer {
	jwtService := NewJWTService(config.Security.JWT.SecretKey, config.Security.JWT.Issuer, config.Security.JWT.ExpiryHours)
	passwordService := NewPasswordService()
	authMiddleware := NewAuthMiddleware(jwtService, database)

	return &APIServer{
		database:        database,
		brokerService:   brokerService,
		keys:            keys,
		logger:          logger.New(),
		jwtService:      jwtService,
		passwordService: passwordService,
		authMiddleware:  authMiddleware,
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

	// Note: Hub claiming is now handled via JWT-protected /user/hubs/claim endpoint

	// User endpoints (protected with JWT authentication)
	// Note: All /user/* endpoints use JWT tokens to identify the user - no user_id in URL needed
	apiRouter.HandleFunc("/users", api.handleCreateUser).Methods("POST") // Keep public for admin/demo purposes
	apiRouter.Handle("/user/hubs", api.authMiddleware.RequireAuth(http.HandlerFunc(api.handleGetUserHubs))).Methods("GET")
	apiRouter.Handle("/user/hubs/claim", api.authMiddleware.RequireAuth(http.HandlerFunc(api.handleUserHubClaim))).Methods("POST")
	apiRouter.Handle("/user/devices", api.authMiddleware.RequireAuth(http.HandlerFunc(api.handleGetUserDevices))).Methods("GET")
	apiRouter.Handle("/user/devices/{device_id}/action", api.authMiddleware.RequireAuth(http.HandlerFunc(api.handleDeviceAction))).Methods("POST")

	// Debug logging for route registration
	api.logger.Info().Msg("User hub claim endpoint registered at /api/v1/user/hubs/claim")

	// Admin endpoints (no auth for demo)
	apiRouter.HandleFunc("/admin/users", api.handleListUsers).Methods("GET")
	apiRouter.HandleFunc("/admin/hubs", api.handleListHubs).Methods("GET")
	apiRouter.HandleFunc("/admin/devices", api.handleListDevices).Methods("GET")

	// Authentication endpoints
	apiRouter.HandleFunc("/auth/register", api.handleRegister).Methods("POST")
	apiRouter.HandleFunc("/auth/login", api.handleLogin).Methods("POST")
	apiRouter.Handle("/auth/me", api.authMiddleware.RequireAuth(http.HandlerFunc(api.handleGetCurrentUser))).Methods("GET")

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
		"status":        "running",
		"active_hubs":   getActiveHubCount(stats),
		"uptime":        "N/A", // Could add uptime tracking
		"version":       "1.0.0",
		"timestamp":     time.Now().UTC().Format(time.RFC3339),
		"service_stats": stats,
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
	if req.ProductKey == "" {
		api.sendError(w, http.StatusBadRequest, "Product key is required")
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
			Str("product_key", req.ProductKey).
			Err(err).
			Msg("Failed to register hub")

		// Provide specific error messages for common failures
		if strings.Contains(err.Error(), "already registered") {
			api.sendError(w, http.StatusConflict, err.Error())
		} else if strings.Contains(err.Error(), "product key") && strings.Contains(err.Error(), "required") {
			api.sendError(w, http.StatusBadRequest, err.Error())
		} else {
			api.sendError(w, http.StatusInternalServerError, "Failed to register hub")
		}
		return
	}

	// Return success response with gateway information
	response := map[string]interface{}{
		"success": true,
		"message": "Hub registered successfully",
		"hub":     hub,
		"gateway_info": map[string]interface{}{
			"public_key":   api.keys.GetServerPublicKey(),
			"zmq_endpoint": "tcp://localhost:5555", // Should be configurable
			"api_endpoint": r.Host,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusCreated, response)
}

// handleHubClaim is deprecated - use /user/hubs/claim with JWT authentication instead
// This endpoint was removed for security reasons to prevent unauthorized hub claiming
// All hub claiming should go through the JWT-protected user endpoints

// handleUserHubClaim handles hub claiming for authenticated users (JWT protected)
func (api *APIServer) handleUserHubClaim(w http.ResponseWriter, r *http.Request) {
	api.logger.Info().Msg("User hub claim request received")

	var req struct {
		ProductKey string `json:"product_key"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate required fields
	if req.ProductKey == "" {
		api.sendError(w, http.StatusBadRequest, "Product key is required")
		return
	}

	// Validate product key format (should not be empty or just whitespace)
	if strings.TrimSpace(req.ProductKey) == "" {
		api.sendError(w, http.StatusBadRequest, "Product key cannot be empty or whitespace")
		return
	}

	// Get user from JWT context (set by auth middleware)
	user, ok := r.Context().Value("user").(*User)
	if !ok {
		api.sendError(w, http.StatusUnauthorized, "Invalid authentication context")
		return
	}

	// Log the claim attempt for debugging
	api.logger.Info().
		Str("username", user.Username).
		Int("user_id", user.ID).
		Str("product_key", req.ProductKey).
		Msg("User attempting to claim hub")

	// Find hub by product key
	hub, err := api.database.GetHubByProductKey(req.ProductKey)
	if err != nil {
		api.logger.Warn().
			Str("username", user.Username).
			Int("user_id", user.ID).
			Str("product_key", req.ProductKey).
			Err(err).
			Msg("Hub not found for product key during claim attempt")
		api.sendError(w, http.StatusNotFound, "Hub not found with provided product key. Please check the product key and ensure the hub is registered with the gateway.")
		return
	}

	// Check if hub is already claimed by another user
	if hub.UserID.Valid && hub.UserID.Int32 != 0 {
		if int(hub.UserID.Int32) == user.ID {
			// User is trying to claim their own hub - return success
			api.logger.Info().
				Str("hub_id", hub.HubID).
				Int("user_id", user.ID).
				Str("username", user.Username).
				Msg("User attempted to claim already owned hub")

			response := map[string]interface{}{
				"success":   true,
				"message":   "Hub is already claimed by you",
				"hub_id":    hub.HubID,
				"name":      hub.Name,
				"id":        hub.ID,
				"user":      user.Username,
				"timestamp": time.Now().UTC().Format(time.RFC3339),
			}
			api.sendJSON(w, http.StatusOK, response)
			return
		} else if !hub.AutoRegistered {
			// Hub is claimed by another user and it's a permanent claim
			api.logger.Warn().
				Str("hub_id", hub.HubID).
				Int("requesting_user_id", user.ID).
				Int("owner_user_id", int(hub.UserID.Int32)).
				Str("product_key", req.ProductKey).
				Msg("User attempted to claim hub owned by another user")
			api.sendError(w, http.StatusConflict, "This hub is already claimed by another user. If you believe this is incorrect, please contact support.")
			return
		}
		// If hub.AutoRegistered is true, allow the claim to proceed (re-claiming from auto registration)
	}

	// Claim the hub - update user_id and set auto_registered to false
	api.logger.Info().
		Str("hub_id", hub.HubID).
		Int("user_id", user.ID).
		Str("username", user.Username).
		Str("hub_name", hub.Name).
		Bool("was_auto_registered", hub.AutoRegistered).
		Msg("Proceeding to claim hub")

	// Claim the hub - update user_id and set auto_registered to false
	if err := api.database.ClaimHub(hub.HubID, user.ID); err != nil {
		api.logger.Error().
			Str("hub_id", hub.HubID).
			Int("user_id", user.ID).
			Str("username", user.Username).
			Err(err).
			Msg("Database error while claiming hub")
		api.sendError(w, http.StatusInternalServerError, "Failed to claim hub due to database error. Please try again.")
		return
	}

	// Update devices to link to the user
	if err := api.database.UpdateDevicesUserID(hub.ID, user.ID); err != nil {
		api.logger.Error().
			Str("hub_id", hub.HubID).
			Int("hub_db_id", hub.ID).
			Int("user_id", user.ID).
			Str("username", user.Username).
			Err(err).
			Msg("Failed to update device ownership after hub claim - hub claimed successfully but device ownership not updated")
		// Don't fail the request, just log the warning as this is not critical
	}

	api.logger.Info().
		Str("hub_id", hub.HubID).
		Int("user_id", user.ID).
		Str("username", user.Username).
		Str("hub_name", hub.Name).
		Str("product_key", req.ProductKey).
		Msg("Hub claimed successfully via user endpoint")

	response := map[string]interface{}{
		"success":     true,
		"message":     fmt.Sprintf("Hub '%s' has been successfully claimed and linked to your account", hub.Name),
		"hub_id":      hub.HubID,
		"name":        hub.Name,
		"id":          hub.ID,
		"user":        user.Username,
		"product_key": req.ProductKey,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
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
	// Get authenticated user from context
	authUser, ok := GetUserFromContext(r)
	if !ok {
		api.sendError(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	hubs, err := api.database.GetUserHubs(authUser.ID)
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
	// Get authenticated user from context
	authUser, ok := GetUserFromContext(r)
	if !ok {
		api.sendError(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	devices, err := api.database.GetUserDevices(authUser.ID)
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
	deviceID := vars["device_id"]

	// Get authenticated user from context
	authUser, ok := GetUserFromContext(r)
	if !ok {
		api.sendError(w, http.StatusUnauthorized, "User not found in context")
		return
	}

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

	// Verify device belongs to authenticated user
	if !deviceHub.UserID.Valid || int(deviceHub.UserID.Int32) != authUser.ID {
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
		"status":    "healthy",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
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

// Authentication endpoints
func (api *APIServer) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate required fields
	if req.Username == "" {
		api.sendError(w, http.StatusBadRequest, "Username is required")
		return
	}
	if req.Email == "" {
		api.sendError(w, http.StatusBadRequest, "Email is required")
		return
	}
	if req.Password == "" {
		api.sendError(w, http.StatusBadRequest, "Password is required")
		return
	}
	if len(req.Password) < 6 {
		api.sendError(w, http.StatusBadRequest, "Password must be at least 6 characters long")
		return
	}

	// Hash the password
	hashedPassword, err := api.passwordService.HashPassword(req.Password)
	if err != nil {
		api.logger.Error().Err(err).Msg("Failed to hash password")
		api.sendError(w, http.StatusInternalServerError, "Failed to process password")
		return
	}

	// Create user
	user, err := api.database.CreateUserWithPassword(req.Username, req.Email, hashedPassword)
	if err != nil {
		api.logger.Error().Err(err).Str("username", req.Username).Msg("Failed to create user")
		api.sendError(w, http.StatusConflict, "Username already exists or registration failed")
		return
	}

	// Generate JWT token
	token, err := api.jwtService.GenerateToken(user)
	if err != nil {
		api.logger.Error().Err(err).Int("user_id", user.ID).Msg("Failed to generate token")
		api.sendError(w, http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	api.logger.Info().
		Int("user_id", user.ID).
		Str("username", user.Username).
		Str("email", user.Email).
		Msg("User registered successfully")

	response := map[string]interface{}{
		"success":   true,
		"message":   "User registered successfully",
		"user":      user,
		"token":     token,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusCreated, response)
}

func (api *APIServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		api.sendError(w, http.StatusBadRequest, "Invalid JSON")
		return
	}

	// Validate required fields
	if req.Username == "" && req.Email == "" {
		api.sendError(w, http.StatusBadRequest, "Username or email is required")
		return
	}
	if req.Password == "" {
		api.sendError(w, http.StatusBadRequest, "Password is required")
		return
	}

	// Find user by username or email
	var user *User
	var err error
	if req.Username != "" {
		user, err = api.database.GetUserByUsername(req.Username)
	} else {
		user, err = api.database.GetUserByEmail(req.Email)
	}

	if err != nil {
		api.logger.Debug().
			Str("username", req.Username).
			Str("email", req.Email).
			Err(err).
			Msg("User not found during login attempt")
		api.sendError(w, http.StatusUnauthorized, "Invalid username/email or password")
		return
	}

	// Verify password
	if user.PasswordHash == "" {
		api.logger.Warn().
			Int("user_id", user.ID).
			Str("username", user.Username).
			Msg("User has no password hash set")
		api.sendError(w, http.StatusUnauthorized, "Invalid username/email or password")
		return
	}

	valid, err := api.passwordService.VerifyPassword(req.Password, user.PasswordHash)
	if err != nil {
		api.logger.Error().Err(err).Int("user_id", user.ID).Msg("Failed to verify password")
		api.sendError(w, http.StatusInternalServerError, "Authentication failed")
		return
	}

	if !valid {
		api.logger.Debug().
			Int("user_id", user.ID).
			Str("username", user.Username).
			Msg("Invalid password during login attempt")
		api.sendError(w, http.StatusUnauthorized, "Invalid username/email or password")
		return
	}

	// Generate JWT token
	token, err := api.jwtService.GenerateToken(user)
	if err != nil {
		api.logger.Error().Err(err).Int("user_id", user.ID).Msg("Failed to generate token")
		api.sendError(w, http.StatusInternalServerError, "Failed to generate authentication token")
		return
	}

	api.logger.Info().
		Int("user_id", user.ID).
		Str("username", user.Username).
		Msg("User logged in successfully")

	response := map[string]interface{}{
		"success":   true,
		"message":   "Login successful",
		"user":      user,
		"token":     token,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusOK, response)
}

func (api *APIServer) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user, ok := GetUserFromContext(r)
	if !ok {
		api.sendError(w, http.StatusUnauthorized, "User not found in context")
		return
	}

	response := map[string]interface{}{
		"success":   true,
		"user":      user,
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	}

	api.sendJSON(w, http.StatusOK, response)
}
