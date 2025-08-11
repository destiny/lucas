package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog"
)

// ConfigAPIServer provides HTTP endpoints for device configuration management
type ConfigAPIServer struct {
	daemon *Daemon
	server *http.Server
	logger zerolog.Logger
}

// DeviceConfigRequest represents a device configuration request
type DeviceConfigRequest struct {
	Devices []DeviceConfig `json:"devices"`
}

// DeviceConfigResponse represents the response to device configuration requests
type DeviceConfigResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// NewConfigAPIServer creates a new configuration API server
func NewConfigAPIServer(daemon *Daemon, port int) *ConfigAPIServer {
	server := &ConfigAPIServer{
		daemon: daemon,
		logger: daemon.logger.With().Str("component", "config_api").Logger(),
	}

	router := mux.NewRouter()
	
	// Device configuration endpoints
	router.HandleFunc("/devices/configure", server.handleDeviceConfigure).Methods("POST")
	router.HandleFunc("/devices/list", server.handleDeviceList).Methods("GET")
	router.HandleFunc("/devices/reload", server.handleDeviceReload).Methods("POST")
	
	// Health check
	router.HandleFunc("/health", server.handleHealth).Methods("GET")

	server.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	return server
}

// Start starts the configuration API server
func (s *ConfigAPIServer) Start() error {
	s.logger.Info().
		Str("address", s.server.Addr).
		Msg("Starting hub configuration API server")

	go func() {
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error().Err(err).Msg("Configuration API server error")
		}
	}()

	return nil
}

// Stop stops the configuration API server
func (s *ConfigAPIServer) Stop() error {
	s.logger.Info().Msg("Stopping hub configuration API server")
	return s.server.Shutdown(s.daemon.ctx)
}

// handleDeviceConfigure handles device configuration requests
func (s *ConfigAPIServer) handleDeviceConfigure(w http.ResponseWriter, r *http.Request) {
	s.logger.Info().Msg("Received device configuration request")

	var req DeviceConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, http.StatusBadRequest, "Invalid JSON format", err)
		return
	}

	// Validate device configurations
	if len(req.Devices) == 0 {
		s.sendError(w, http.StatusBadRequest, "At least one device must be configured", nil)
		return
	}

	// Update the hub configuration
	s.daemon.config.Devices = req.Devices

	// Save configuration to file
	if err := s.daemon.config.Save(s.daemon.configPath); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to save configuration", err)
		return
	}

	// Reload device manager with new configuration
	if err := s.daemon.deviceManager.Reload(s.daemon.config, s.daemon.debug, s.daemon.testMode); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to reload devices", err)
		return
	}

	s.sendSuccess(w, "Device configuration updated successfully", map[string]interface{}{
		"devices_configured": len(req.Devices),
		"devices":           req.Devices,
	})

	s.logger.Info().
		Int("device_count", len(req.Devices)).
		Msg("Device configuration updated successfully")
}

// handleDeviceList returns the current device configuration
func (s *ConfigAPIServer) handleDeviceList(w http.ResponseWriter, r *http.Request) {
	s.logger.Debug().Msg("Device list requested")

	s.sendSuccess(w, "Device list retrieved successfully", map[string]interface{}{
		"devices": s.daemon.config.Devices,
		"count":   len(s.daemon.config.Devices),
	})
}

// handleDeviceReload reloads devices from current configuration
func (s *ConfigAPIServer) handleDeviceReload(w http.ResponseWriter, r *http.Request) {
	s.logger.Info().Msg("Device reload requested")

	if err := s.daemon.deviceManager.Reload(s.daemon.config, s.daemon.debug, s.daemon.testMode); err != nil {
		s.sendError(w, http.StatusInternalServerError, "Failed to reload devices", err)
		return
	}

	s.sendSuccess(w, "Devices reloaded successfully", map[string]interface{}{
		"devices_loaded": len(s.daemon.config.Devices),
	})
}

// handleHealth returns the health status of the hub
func (s *ConfigAPIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	s.sendSuccess(w, "Hub is healthy", map[string]interface{}{
		"status":       "healthy",
		"device_count": len(s.daemon.config.Devices),
		"hub_id":       s.daemon.config.Hub.ID,
	})
}

// sendSuccess sends a successful response
func (s *ConfigAPIServer) sendSuccess(w http.ResponseWriter, message string, data interface{}) {
	response := DeviceConfigResponse{
		Success: true,
		Message: message,
		Data:    data,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// sendError sends an error response
func (s *ConfigAPIServer) sendError(w http.ResponseWriter, statusCode int, message string, err error) {
	response := DeviceConfigResponse{
		Success: false,
		Message: message,
	}

	if err != nil {
		response.Error = err.Error()
		s.logger.Error().Err(err).Str("message", message).Msg("API error")
	} else {
		s.logger.Warn().Str("message", message).Msg("API client error")
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}