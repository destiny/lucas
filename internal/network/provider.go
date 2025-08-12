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

package network

import (
	"context"
)

// NetworkProvider defines the interface for different network transport implementations
type NetworkProvider interface {
	// Name returns the provider name (e.g., "zmq", "coap", "http")
	Name() string

	// Send sends a message to a specific hub and waits for response
	Send(ctx context.Context, hubID string, message []byte) ([]byte, error)

	// SendFireAndForget sends a message to a hub without waiting for response
	SendFireAndForget(hubID string, message []byte) error

	// Start initializes and starts the provider
	Start(ctx context.Context) error

	// Stop gracefully shuts down the provider
	Stop() error

	// IsRunning returns whether the provider is currently active
	IsRunning() bool

	// RegisterHubConnection registers that a hub is using this provider
	RegisterHub(hubID string) error

	// UnregisterHubConnection removes a hub from this provider
	UnregisterHub(hubID string) error
}

// HubRegistry tracks which transport each hub is using
type HubRegistry struct {
	// hubTransports maps hub_id to provider name
	hubTransports map[string]string // "hub_abc123" -> "zmq" | "coap"
}

// NewHubRegistry creates a new hub transport registry
func NewHubRegistry() *HubRegistry {
	return &HubRegistry{
		hubTransports: make(map[string]string),
	}
}

// RegisterHubTransport records which provider a hub is using
func (r *HubRegistry) RegisterHubTransport(hubID, providerName string) {
	r.hubTransports[hubID] = providerName
}

// GetHubTransport returns the provider name for a hub
func (r *HubRegistry) GetHubTransport(hubID string) (string, bool) {
	provider, exists := r.hubTransports[hubID]
	return provider, exists
}

// UnregisterHub removes a hub from the registry
func (r *HubRegistry) UnregisterHub(hubID string) {
	delete(r.hubTransports, hubID)
}

// GetAllHubs returns all registered hubs and their transports
func (r *HubRegistry) GetAllHubs() map[string]string {
	result := make(map[string]string)
	for hubID, provider := range r.hubTransports {
		result[hubID] = provider
	}
	return result
}