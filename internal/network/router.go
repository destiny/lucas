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
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// Router coordinates message routing across multiple network providers
type Router struct {
	registry  *HubRegistry
	providers map[string]NetworkProvider
	logger    zerolog.Logger
	mutex     sync.RWMutex
}

// NewRouter creates a new multi-provider router
func NewRouter() *Router {
	return &Router{
		registry:  NewHubRegistry(),
		providers: make(map[string]NetworkProvider),
		logger:    logger.GetLogger("network.router"),
	}
}

// RegisterProvider adds a network provider to the router
func (r *Router) RegisterProvider(provider NetworkProvider) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	name := provider.Name()
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider
	r.logger.Info().Str("provider", name).Msg("Network provider registered")
	return nil
}

// StartAll starts all registered providers
func (r *Router) StartAll(ctx context.Context) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	for name, provider := range r.providers {
		if err := provider.Start(ctx); err != nil {
			r.logger.Error().Err(err).Str("provider", name).Msg("Failed to start provider")
			return fmt.Errorf("failed to start provider %s: %w", name, err)
		}
		r.logger.Info().Str("provider", name).Msg("Network provider started")
	}
	return nil
}

// StopAll stops all registered providers
func (r *Router) StopAll() error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var lastErr error
	for name, provider := range r.providers {
		if err := provider.Stop(); err != nil {
			r.logger.Error().Err(err).Str("provider", name).Msg("Failed to stop provider")
			lastErr = err
		} else {
			r.logger.Info().Str("provider", name).Msg("Network provider stopped")
		}
	}
	return lastErr
}

// SendToHub routes a message to the appropriate provider based on hub transport
func (r *Router) SendToHub(ctx context.Context, hubID string, message []byte) ([]byte, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Look up which transport this hub is using
	providerName, exists := r.registry.GetHubTransport(hubID)
	if !exists {
		return nil, fmt.Errorf("hub %s not registered with any transport", hubID)
	}

	// Get the appropriate provider
	provider, exists := r.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("transport provider %s not available", providerName)
	}

	// Route via the specific provider
	r.logger.Debug().
		Str("hub_id", hubID).
		Str("provider", providerName).
		Msg("Routing message to hub")

	return provider.Send(ctx, hubID, message)
}

// SendFireAndForgetToHub sends a message without waiting for response
func (r *Router) SendFireAndForgetToHub(hubID string, message []byte) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	// Look up which transport this hub is using
	providerName, exists := r.registry.GetHubTransport(hubID)
	if !exists {
		return fmt.Errorf("hub %s not registered with any transport", hubID)
	}

	// Get the appropriate provider
	provider, exists := r.providers[providerName]
	if !exists {
		return fmt.Errorf("transport provider %s not available", providerName)
	}

	// Route via the specific provider
	r.logger.Debug().
		Str("hub_id", hubID).
		Str("provider", providerName).
		Msg("Routing fire-and-forget message to hub")

	return provider.SendFireAndForget(hubID, message)
}

// RegisterHubTransport records which provider a hub is using
func (r *Router) RegisterHubTransport(hubID, providerName string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Verify provider exists
	provider, exists := r.providers[providerName]
	if !exists {
		return fmt.Errorf("provider %s not registered", providerName)
	}

	// Register hub with the provider
	if err := provider.RegisterHub(hubID); err != nil {
		return fmt.Errorf("failed to register hub with provider %s: %w", providerName, err)
	}

	// Update registry
	r.registry.RegisterHubTransport(hubID, providerName)
	
	r.logger.Info().
		Str("hub_id", hubID).
		Str("provider", providerName).
		Msg("Hub transport registered")

	return nil
}

// UnregisterHub removes a hub from all providers and registry
func (r *Router) UnregisterHub(hubID string) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	// Get current provider
	providerName, exists := r.registry.GetHubTransport(hubID)
	if !exists {
		return nil // Hub not registered
	}

	// Unregister from provider
	if provider, exists := r.providers[providerName]; exists {
		if err := provider.UnregisterHub(hubID); err != nil {
			r.logger.Warn().Err(err).
				Str("hub_id", hubID).
				Str("provider", providerName).
				Msg("Failed to unregister hub from provider")
		}
	}

	// Remove from registry
	r.registry.UnregisterHub(hubID)

	r.logger.Info().
		Str("hub_id", hubID).
		Str("provider", providerName).
		Msg("Hub transport unregistered")

	return nil
}

// GetHubTransports returns all hub transport mappings
func (r *Router) GetHubTransports() map[string]string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.registry.GetAllHubs()
}

// GetProviders returns all registered provider names
func (r *Router) GetProviders() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	providers := make([]string, 0, len(r.providers))
	for name := range r.providers {
		providers = append(providers, name)
	}
	return providers
}