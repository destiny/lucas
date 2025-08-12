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

package zmq

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"
	"lucas/internal/hermes"
	"lucas/internal/logger"
	"lucas/internal/network"
)

// ZMQProvider wraps Hermes ZMQ functionality as a NetworkProvider
type ZMQProvider struct {
	broker     *hermes.Broker
	client     *hermes.HermesClient
	logger     zerolog.Logger
	isRunning  bool
	mutex      sync.RWMutex
	
	// ZMQ configuration
	endpoint   string
	keys       *ZMQKeys // Curve encryption keys
	
	// Hub tracking
	connectedHubs map[string]bool
	hubMutex      sync.RWMutex
}

// ZMQKeys holds Curve encryption keys for ZMQ
type ZMQKeys struct {
	PublicKey  string
	PrivateKey string
}

// NewZMQProvider creates a new ZMQ network provider
func NewZMQProvider(endpoint string, keys *ZMQKeys) *ZMQProvider {
	return &ZMQProvider{
		endpoint:      endpoint,
		keys:          keys,
		logger:        logger.GetLogger("network.zmq"),
		connectedHubs: make(map[string]bool),
	}
}

// Name returns the provider name
func (z *ZMQProvider) Name() string {
	return "zmq"
}

// Start initializes and starts the ZMQ broker and client
func (z *ZMQProvider) Start(ctx context.Context) error {
	z.mutex.Lock()
	defer z.mutex.Unlock()

	if z.isRunning {
		return fmt.Errorf("ZMQ provider already running")
	}

	// Start Hermes broker
	broker := hermes.NewBroker(z.endpoint)
	if z.keys != nil {
		broker.SetCurveKeys(z.keys.PublicKey, z.keys.PrivateKey)
	}

	if err := broker.Start(ctx); err != nil {
		return fmt.Errorf("failed to start ZMQ broker: %w", err)
	}
	z.broker = broker

	// Create persistent client for gateway communication
	client := hermes.NewHermesClient("gateway_main")
	if z.keys != nil {
		client.SetCurveKeys(z.keys.PublicKey)
	}

	if err := client.Connect(ctx, z.endpoint); err != nil {
		z.broker.Stop()
		return fmt.Errorf("failed to connect ZMQ client: %w", err)
	}
	z.client = client

	z.isRunning = true
	z.logger.Info().Str("endpoint", z.endpoint).Msg("ZMQ provider started")
	return nil
}

// Stop gracefully shuts down the ZMQ provider
func (z *ZMQProvider) Stop() error {
	z.mutex.Lock()
	defer z.mutex.Unlock()

	if !z.isRunning {
		return nil
	}

	var lastErr error

	// Stop client
	if z.client != nil {
		if err := z.client.Close(); err != nil {
			z.logger.Error().Err(err).Msg("Failed to close ZMQ client")
			lastErr = err
		}
		z.client = nil
	}

	// Stop broker
	if z.broker != nil {
		if err := z.broker.Stop(); err != nil {
			z.logger.Error().Err(err).Msg("Failed to stop ZMQ broker")
			lastErr = err
		}
		z.broker = nil
	}

	z.isRunning = false
	z.logger.Info().Msg("ZMQ provider stopped")
	return lastErr
}

// IsRunning returns whether the provider is currently active
func (z *ZMQProvider) IsRunning() bool {
	z.mutex.RLock()
	defer z.mutex.RUnlock()
	return z.isRunning
}

// Send sends a message to a specific hub and waits for response
func (z *ZMQProvider) Send(ctx context.Context, hubID string, message []byte) ([]byte, error) {
	z.mutex.RLock()
	client := z.client
	z.mutex.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("ZMQ client not available")
	}

	// Parse the message to extract service information
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(message, &serviceReq); err != nil {
		return nil, fmt.Errorf("failed to parse service request: %w", err)
	}

	// Set timeout from context or use default
	timeout := 30 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		timeout = time.Until(deadline)
	}

	// Send via Hermes client
	response, err := client.Request(ctx, serviceReq.Service, message, timeout)
	if err != nil {
		return nil, fmt.Errorf("ZMQ request failed for hub %s: %w", hubID, err)
	}

	z.logger.Debug().
		Str("hub_id", hubID).
		Str("service", serviceReq.Service).
		Msg("ZMQ message sent successfully")

	return response, nil
}

// SendFireAndForget sends a message to a hub without waiting for response
func (z *ZMQProvider) SendFireAndForget(hubID string, message []byte) error {
	z.mutex.RLock()
	client := z.client
	z.mutex.RUnlock()

	if client == nil {
		return fmt.Errorf("ZMQ client not available")
	}

	// Parse the message to extract service information
	var serviceReq hermes.ServiceRequest
	if err := json.Unmarshal(message, &serviceReq); err != nil {
		return fmt.Errorf("failed to parse service request: %w", err)
	}

	// Send fire-and-forget via Hermes client
	err := client.FireAndForget(serviceReq.Service, message)
	if err != nil {
		return fmt.Errorf("ZMQ fire-and-forget failed for hub %s: %w", hubID, err)
	}

	z.logger.Debug().
		Str("hub_id", hubID).
		Str("service", serviceReq.Service).
		Msg("ZMQ fire-and-forget message sent")

	return nil
}

// RegisterHub registers that a hub is using this provider
func (z *ZMQProvider) RegisterHub(hubID string) error {
	z.hubMutex.Lock()
	defer z.hubMutex.Unlock()

	z.connectedHubs[hubID] = true
	z.logger.Info().Str("hub_id", hubID).Msg("Hub registered with ZMQ provider")
	return nil
}

// UnregisterHub removes a hub from this provider
func (z *ZMQProvider) UnregisterHub(hubID string) error {
	z.hubMutex.Lock()
	defer z.hubMutex.Unlock()

	delete(z.connectedHubs, hubID)
	z.logger.Info().Str("hub_id", hubID).Msg("Hub unregistered from ZMQ provider")
	return nil
}

// GetConnectedHubs returns all hubs connected via this provider
func (z *ZMQProvider) GetConnectedHubs() []string {
	z.hubMutex.RLock()
	defer z.hubMutex.RUnlock()

	hubs := make([]string, 0, len(z.connectedHubs))
	for hubID := range z.connectedHubs {
		hubs = append(hubs, hubID)
	}
	return hubs
}

// Compile-time check that ZMQProvider implements NetworkProvider
var _ network.NetworkProvider = (*ZMQProvider)(nil)