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

package hub

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

// GatewayInfo represents information about a discovered gateway
type GatewayInfo struct {
	APIEndpoint string `json:"api_endpoint"`
	ZMQEndpoint string `json:"zmq_endpoint"`
	PublicKey   string `json:"public_key"`
	Version     string `json:"version"`
	Online      bool   `json:"online"`
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

// DiscoverHTTPFromConfig attempts to discover the HTTP endpoint using config's smart conversion
func (gd *GatewayDiscovery) DiscoverHTTPFromConfig(config *Config) (*GatewayInfo, error) {
	// Check if user explicitly configured HTTP endpoint
	if config.Gateway.HTTPEndpoint != "" {
		// User explicitly configured - ONLY try this endpoint, no fallbacks
		info, err := gd.CheckGateway(config.Gateway.HTTPEndpoint)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to configured HTTP endpoint %s: %w", 
				config.Gateway.HTTPEndpoint, err)
		}
		return info, nil
	}
	
	// No explicit HTTP endpoint config - try auto-derivation from ZMQ endpoint
	httpEndpoint := config.GetHTTPEndpoint()
	
	info, err := gd.CheckGateway(httpEndpoint)
	if err == nil {
		// Success! Save the discovered endpoint to config
		config.SetHTTPEndpoint(httpEndpoint)
		return info, nil
	}
	
	// If derived endpoint failed, try common alternatives
	candidateEndpoints := gd.generateCandidateEndpoints(config.Gateway.Endpoint)
	
	for _, endpoint := range candidateEndpoints {
		info, err := gd.CheckGateway(endpoint)
		if err == nil {
			// Success! Save the discovered endpoint to config
			config.SetHTTPEndpoint(endpoint)
			return info, nil
		}
	}
	
	// All automatic attempts failed, prompt user interactively
	return gd.promptForHTTPEndpoint(config)
}

// generateCandidateEndpoints generates alternative HTTP endpoints to try
func (gd *GatewayDiscovery) generateCandidateEndpoints(zmqEndpoint string) []string {
	candidates := []string{}
	
	if zmqEndpoint == "" {
		return []string{"http://localhost:8080", "http://127.0.0.1:8080"}
	}
	
	// Extract host from ZMQ endpoint
	host := extractHostFromEndpoint(zmqEndpoint)
	
	// Try different HTTP ports
	httpPorts := []string{"8080", "80", "8000", "3000"}
	for _, port := range httpPorts {
		candidates = append(candidates, fmt.Sprintf("http://%s:%s", host, port))
	}
	
	return candidates
}

// extractHostFromEndpoint extracts the host part from a ZMQ endpoint
func extractHostFromEndpoint(endpoint string) string {
	// Remove protocol
	host := strings.TrimPrefix(endpoint, "tcp://")
	
	// Handle wildcard addresses
	if strings.HasPrefix(host, "*:") || strings.HasPrefix(host, "0.0.0.0:") {
		return "localhost"
	}
	
	// Extract host part (before port)
	if idx := strings.LastIndex(host, ":"); idx != -1 {
		return host[:idx]
	}
	
	return host
}

// promptForHTTPEndpoint interactively prompts user for the correct HTTP endpoint
func (gd *GatewayDiscovery) promptForHTTPEndpoint(config *Config) (*GatewayInfo, error) {
	reader := bufio.NewReader(os.Stdin)
	
	fmt.Printf("\n🔍 Gateway HTTP endpoint discovery failed!\n")
	fmt.Printf("ZMQ endpoint: %s\n", config.Gateway.Endpoint)
	fmt.Printf("Tried: %s\n\n", config.GetHTTPEndpoint())
	
	for {
		fmt.Printf("Please enter the correct HTTP endpoint for the gateway API\n")
		fmt.Printf("(e.g., http://192.168.1.100:8080 or http://gateway.local:8080): ")
		
		userInput, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("failed to read user input: %w", err)
		}
		
		httpEndpoint := strings.TrimSpace(userInput)
		if httpEndpoint == "" {
			fmt.Printf("❌ Empty input. Please try again.\n\n")
			continue
		}
		
		// Ensure proper HTTP prefix
		if !strings.HasPrefix(httpEndpoint, "http://") && !strings.HasPrefix(httpEndpoint, "https://") {
			httpEndpoint = "http://" + httpEndpoint
		}
		
		fmt.Printf("🔗 Testing connection to: %s\n", httpEndpoint)
		
		info, err := gd.CheckGateway(httpEndpoint)
		if err == nil {
			fmt.Printf("✅ Connection successful!\n")
			// Save the working endpoint to config
			config.SetHTTPEndpoint(httpEndpoint)
			return info, nil
		}
		
		fmt.Printf("❌ Connection failed: %v\n", err)
		fmt.Printf("Please check the URL and try again.\n\n")
	}
}
