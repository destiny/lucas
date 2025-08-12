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
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// CoAPWorkerSimple provides simplified CoAP-based communication for hubs
// This is a minimal implementation for initial CoAP support
type CoAPWorkerSimple struct {
	config        *Config
	deviceMgr     *DeviceManager
	logger        zerolog.Logger
	isRunning     bool
	mutex         sync.RWMutex
	
	// Service handling
	serviceName   string
	workerID      string
	stats         *ServiceHandlerStats
}

// NewCoAPWorker creates a new CoAP worker for hub communication (simplified)
func NewCoAPWorker(config *Config, deviceMgr *DeviceManager, serviceName, workerID string) *CoAPWorkerSimple {
	return &CoAPWorkerSimple{
		config:      config,
		deviceMgr:   deviceMgr,
		logger:      logger.New(),
		serviceName: serviceName,
		workerID:    workerID,
		stats:       &ServiceHandlerStats{},
	}
}

// Start connects the CoAP worker to the gateway (simplified)
func (w *CoAPWorkerSimple) Start() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if w.isRunning {
		return fmt.Errorf("CoAP worker already running")
	}

	// For now, just log that we're starting CoAP worker
	// Full CoAP client implementation will be added incrementally
	w.logger.Info().
		Str("service", w.serviceName).
		Str("worker_id", w.workerID).
		Str("gateway_endpoint", w.config.Gateway.Endpoint).
		Str("transport", w.config.GetTransport()).
		Msg("CoAP worker started (simplified mode)")

	w.isRunning = true
	return nil
}

// Stop disconnects the CoAP worker
func (w *CoAPWorkerSimple) Stop() error {
	w.mutex.Lock()
	defer w.mutex.Unlock()

	if !w.isRunning {
		return nil
	}

	w.isRunning = false
	w.logger.Info().Msg("CoAP worker stopped")
	return nil
}

// IsConnected returns whether the CoAP worker is connected (simplified)
func (w *CoAPWorkerSimple) IsConnected() bool {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	
	// In simplified mode, always return true if running
	return w.isRunning
}

// GetStats returns worker statistics
func (w *CoAPWorkerSimple) GetStats() *ServiceHandlerStats {
	w.mutex.RLock()
	defer w.mutex.RUnlock()
	
	// Return a copy of stats
	statsCopy := *w.stats
	return &statsCopy
}