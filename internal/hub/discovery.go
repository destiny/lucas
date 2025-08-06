package hub

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// GatewayInfo represents information about a discovered gateway
type GatewayInfo struct {
	APIEndpoint   string `json:"api_endpoint"`
	ZMQEndpoint   string `json:"zmq_endpoint"`
	PublicKey     string `json:"public_key"`
	Version       string `json:"version"`
	Online        bool   `json:"online"`
}

// GatewayDiscovery handles gateway discovery and API communication
type GatewayDiscovery struct {
	client *http.Client
}

// NewGatewayDiscovery creates a new gateway discovery client
func NewGatewayDiscovery() *GatewayDiscovery {
	return &GatewayDiscovery{
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// DiscoverGateway attempts to discover a gateway at common locations
func (gd *GatewayDiscovery) DiscoverGateway() (*GatewayInfo, error) {
	// Common gateway locations to try
	candidates := []string{
		"http://localhost:8080",
		"http://127.0.0.1:8080",
		"http://gateway:8080",
		"http://gateway.local:8080",
	}

	var lastErr error
	for _, baseURL := range candidates {
		info, err := gd.CheckGateway(baseURL)
		if err == nil {
			return info, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("no gateway found at common locations: %w", lastErr)
}

// CheckGateway checks if a gateway is available at the specified URL
func (gd *GatewayDiscovery) CheckGateway(baseURL string) (*GatewayInfo, error) {
	// Ensure baseURL has http:// prefix
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "http://" + baseURL
	}

	// Try health endpoint first
	healthURL := baseURL + "/api/v1/health"
	healthResp, err := gd.makeRequest(healthURL)
	if err != nil {
		return nil, fmt.Errorf("gateway health check failed: %w", err)
	}

	// Check if it's a healthy response
	if status, ok := healthResp["status"].(string); !ok || status != "healthy" {
		return nil, fmt.Errorf("gateway not healthy")
	}

	// Get gateway status for more info
	statusURL := baseURL + "/api/v1/gateway/status"
	statusResp, err := gd.makeRequest(statusURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway status: %w", err)
	}

	// Get gateway keys
	keysURL := baseURL + "/api/v1/gateway/keys/info"
	keysResp, err := gd.makeRequest(keysURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get gateway keys: %w", err)
	}

	// Extract gateway information
	info := &GatewayInfo{
		APIEndpoint: baseURL,
		Online:      true,
	}

	// Extract version from status response
	if version, ok := statusResp["version"].(string); ok {
		info.Version = version
	}

	// Extract public key from keys response
	if publicKey, ok := keysResp["public_key"].(string); ok {
		info.PublicKey = publicKey
	}

	// Try to determine ZMQ endpoint (this might need to be configured)
	// For now, assume standard ZMQ port based on API URL
	zmqEndpoint := strings.Replace(baseURL, ":8080", ":5555", 1)
	zmqEndpoint = strings.Replace(zmqEndpoint, "http://", "tcp://", 1)
	zmqEndpoint = strings.Replace(zmqEndpoint, "https://", "tcp://", 1)
	info.ZMQEndpoint = zmqEndpoint

	return info, nil
}

// RegisterWithGateway registers the hub with the gateway
func (gd *GatewayDiscovery) RegisterWithGateway(gatewayURL, hubID, hubPublicKey, productKey string) error {
	registerURL := gatewayURL + "/api/v1/hub/register"
	
	// Prepare registration payload
	payload := map[string]interface{}{
		"hub_id":      hubID,
		"public_key":  hubPublicKey,
		"name":        hubID,
		"product_key": productKey,
		"timestamp":   time.Now().UTC().Format(time.RFC3339),
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	// Make POST request
	resp, err := gd.client.Post(registerURL, "application/json", strings.NewReader(string(payloadJSON)))
	if err != nil {
		return fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("registration failed with status: %s", resp.Status)
	}

	// Parse response
	var regResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&regResponse); err != nil {
		return fmt.Errorf("failed to parse registration response: %w", err)
	}

	// Check if registration was successful
	if success, ok := regResponse["success"].(bool); !ok || !success {
		errorMsg := "unknown error"
		if errStr, ok := regResponse["message"].(string); ok {
			errorMsg = errStr
		}
		return fmt.Errorf("registration failed: %s", errorMsg)
	}

	return nil
}

// makeRequest makes an HTTP GET request and returns the parsed JSON response
func (gd *GatewayDiscovery) makeRequest(url string) (map[string]interface{}, error) {
	resp, err := gd.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	return result, nil
}

// GetGatewayInfo retrieves gateway information from a specific URL
func (gd *GatewayDiscovery) GetGatewayInfo(gatewayURL string) (*GatewayInfo, error) {
	return gd.CheckGateway(gatewayURL)
}

// TestGatewayConnection tests connectivity to a gateway
func (gd *GatewayDiscovery) TestGatewayConnection(gatewayURL string) error {
	_, err := gd.CheckGateway(gatewayURL)
	return err
}