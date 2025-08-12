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

package coap

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"sync"

	"github.com/rs/zerolog"
	"lucas/internal/hermes"
	"lucas/internal/logger"
	"lucas/internal/network"
)

// CoAPProviderSimple implements NetworkProvider interface for CoAP transport
// This is a simplified version for initial implementation
type CoAPProviderSimple struct {
	logger        zerolog.Logger
	isRunning     bool
	mutex         sync.RWMutex
	
	// CoAP configuration
	endpoint      string
	listenAddr    string
	
	// Hub tracking
	connectedHubs map[string]bool
	hubMutex      sync.RWMutex
	
	// UDP connection for simple implementation
	conn          *net.UDPConn
}

// NewCoAPProvider creates a new CoAP network provider (simplified version)
func NewCoAPProvider(endpoint string) *CoAPProviderSimple {
	// Parse endpoint to determine listen address
	listenAddr := "0.0.0.0:5683" // Default CoAP port
	if u, err := url.Parse(endpoint); err == nil && u.Host != "" {
		listenAddr = u.Host
	}
	
	return &CoAPProviderSimple{
		endpoint:      endpoint,
		listenAddr:    listenAddr,
		logger:        logger.New(),
		connectedHubs: make(map[string]bool),
	}
}

// Name returns the provider name
func (c *CoAPProviderSimple) Name() string {
	return "coap"
}

// Start initializes and starts the CoAP server (simplified)
func (c *CoAPProviderSimple) Start(ctx context.Context) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.isRunning {
		return fmt.Errorf("CoAP provider already running")
	}

	// For now, just log that we're starting CoAP
	// Full CoAP implementation will be added incrementally
	c.logger.Info().
		Str("endpoint", c.endpoint).
		Str("listen_addr", c.listenAddr).
		Msg("CoAP provider started (simplified mode)")

	c.isRunning = true
	return nil
}

// Stop gracefully shuts down the CoAP provider
func (c *CoAPProviderSimple) Stop() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if !c.isRunning {
		return nil
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}

	c.isRunning = false
	c.logger.Info().Msg("CoAP provider stopped")
	return nil
}

// IsRunning returns whether the provider is currently active
func (c *CoAPProviderSimple) IsRunning() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.isRunning
}

// Send sends a message to a specific hub and waits for response (simplified)
func (c *CoAPProviderSimple) Send(ctx context.Context, hubID string, message []byte) ([]byte, error) {
	c.hubMutex.RLock()
	_, exists := c.connectedHubs[hubID]
	c.hubMutex.RUnlock()

	if !exists {
		return nil, fmt.Errorf("hub %s not connected via CoAP", hubID)
	}

	// Parse the Hermes ServiceRequest for logging
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(message, &serviceReq); err != nil {
		return nil, fmt.Errorf("failed to parse service request: %w", err)
	}

	c.logger.Debug().
		Str("hub_id", hubID).
		Str("service", serviceReq.Service).
		Msg("CoAP message send requested (simplified - not implemented)")

	// For now, return a mock response
	mockResponse := hermes.CreateServiceResponseWithNonce(
		serviceReq.MessageID,
		serviceReq.Service,
		serviceReq.Nonce,
		true,
		map[string]interface{}{
			"status": "mock_response",
			"message": "CoAP implementation in progress",
		},
		nil,
	)

	return json.Marshal(mockResponse)
}

// SendFireAndForget sends a message to a hub without waiting for response (simplified)
func (c *CoAPProviderSimple) SendFireAndForget(hubID string, message []byte) error {
	c.hubMutex.RLock()
	_, exists := c.connectedHubs[hubID]
	c.hubMutex.RUnlock()

	if !exists {
		return fmt.Errorf("hub %s not connected via CoAP", hubID)
	}

	// Parse the Hermes ServiceRequest for logging
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(message, &serviceReq); err != nil {
		return fmt.Errorf("failed to parse service request: %w", err)
	}

	c.logger.Debug().
		Str("hub_id", hubID).
		Str("service", serviceReq.Service).
		Msg("CoAP fire-and-forget message sent (simplified - not implemented)")

	return nil
}

// RegisterHub registers that a hub is using this provider
func (c *CoAPProviderSimple) RegisterHub(hubID string) error {
	c.hubMutex.Lock()
	defer c.hubMutex.Unlock()

	c.connectedHubs[hubID] = true
	c.logger.Info().Str("hub_id", hubID).Msg("Hub registered with CoAP provider (simplified)")
	return nil
}

// UnregisterHub removes a hub from this provider
func (c *CoAPProviderSimple) UnregisterHub(hubID string) error {
	c.hubMutex.Lock()
	defer c.hubMutex.Unlock()

	delete(c.connectedHubs, hubID)
	c.logger.Info().Str("hub_id", hubID).Msg("Hub unregistered from CoAP provider")
	return nil
}

// GetConnectedHubs returns all hubs connected via this provider
func (c *CoAPProviderSimple) GetConnectedHubs() []string {
	c.hubMutex.RLock()
	defer c.hubMutex.RUnlock()

	hubs := make([]string, 0, len(c.connectedHubs))
	for hubID := range c.connectedHubs {
		hubs = append(hubs, hubID)
	}
	return hubs
}

// Compile-time check that CoAPProviderSimple implements NetworkProvider
var _ network.NetworkProvider = (*CoAPProviderSimple)(nil)