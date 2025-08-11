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

package hermes

import (
	"fmt"
	"testing"
	"time"
)

// MockRequestHandler implements RequestHandler for testing
type MockRequestHandler struct {
	responses map[string][]byte
	errors    map[string]error
	callCount int
}

func NewMockRequestHandler() *MockRequestHandler {
	return &MockRequestHandler{
		responses: make(map[string][]byte),
		errors:    make(map[string]error),
	}
}

func (m *MockRequestHandler) Handle(request []byte) ([]byte, error) {
	m.callCount++
	requestStr := string(request)
	
	if err, exists := m.errors[requestStr]; exists {
		return nil, err
	}
	
	if response, exists := m.responses[requestStr]; exists {
		return response, nil
	}
	
	// Default response
	return []byte(fmt.Sprintf("processed: %s", requestStr)), nil
}

func (m *MockRequestHandler) SetResponse(request string, response []byte) {
	m.responses[request] = response
}

func (m *MockRequestHandler) SetError(request string, err error) {
	m.errors[request] = err
}

func (m *MockRequestHandler) GetCallCount() int {
	return m.callCount
}

func TestBrokerBasicOperations(t *testing.T) {
	// Note: These tests would require actual ZMQ sockets for full integration testing
	// Here we test the data structures and logic

	t.Run("NewBroker", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		if broker == nil {
			t.Fatal("Expected non-nil broker")
		}
		if broker.address != "tcp://localhost:5555" {
			t.Errorf("Expected address 'tcp://localhost:5555', got %s", broker.address)
		}
		if broker.services == nil {
			t.Error("Expected non-nil services map")
		}
		if broker.workers == nil {
			t.Error("Expected non-nil workers map")
		}
		if broker.heartbeat != 45*time.Second {
			t.Errorf("Expected default heartbeat 45s, got %v", broker.heartbeat)
		}
	})

	t.Run("BrokerStats", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		stats := broker.GetStats()
		if stats == nil {
			t.Fatal("Expected non-nil stats")
		}
		if stats.Services != 0 {
			t.Errorf("Expected 0 services, got %d", stats.Services)
		}
		if stats.Workers != 0 {
			t.Errorf("Expected 0 workers, got %d", stats.Workers)
		}
		if stats.StartTime.IsZero() {
			t.Error("Expected non-zero start time")
		}
	})

	t.Run("ServiceManagement", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Initially no services
		services := broker.GetServices()
		if len(services) != 0 {
			t.Errorf("Expected 0 services, got %d", len(services))
		}
		
		// Simulate adding a worker (this would normally happen via handleWorkerReady)
		err := broker.handleWorkerReady("worker1", "test.service")
		if err != nil {
			t.Errorf("Failed to handle worker ready: %v", err)
		}
		
		// Check that service was created
		services = broker.GetServices()
		if len(services) != 1 {
			t.Errorf("Expected 1 service, got %d", len(services))
		}
		
		if service, exists := services["test.service"]; exists {
			if service.Name != "test.service" {
				t.Errorf("Expected service name 'test.service', got %s", service.Name)
			}
			if len(service.Workers) != 1 {
				t.Errorf("Expected 1 worker, got %d", len(service.Workers))
			}
		} else {
			t.Error("Expected service 'test.service' to exist")
		}
		
		// Check workers
		workers := broker.GetWorkers()
		if len(workers) != 1 {
			t.Errorf("Expected 1 worker, got %d", len(workers))
		}
		
		if worker, exists := workers["worker1"]; exists {
			if worker.Identity != "worker1" {
				t.Errorf("Expected worker identity 'worker1', got %s", worker.Identity)
			}
			if worker.Service != "test.service" {
				t.Errorf("Expected worker service 'test.service', got %s", worker.Service)
			}
		} else {
			t.Error("Expected worker 'worker1' to exist")
		}
	})

	t.Run("WorkerHeartbeat", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Add a worker
		err := broker.handleWorkerReady("worker1", "test.service")
		if err != nil {
			t.Errorf("Failed to handle worker ready: %v", err)
		}
		
		// Check initial liveness
		workers := broker.GetWorkers()
		worker := workers["worker1"]
		if worker.Liveness != 10 {
			t.Errorf("Expected initial liveness 10, got %d", worker.Liveness)
		}
		
		// Handle heartbeat
		err = broker.handleWorkerHeartbeat("worker1")
		if err != nil {
			t.Errorf("Failed to handle worker heartbeat: %v", err)
		}
		
		// Liveness should remain 10 (reset to full)
		workers = broker.GetWorkers()
		worker = workers["worker1"]
		if worker.Liveness != 10 {
			t.Errorf("Expected liveness 10 after heartbeat, got %d", worker.Liveness)
		}
	})

	t.Run("WorkerRemoval", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Add a worker
		err := broker.handleWorkerReady("worker1", "test.service")
		if err != nil {
			t.Errorf("Failed to handle worker ready: %v", err)
		}
		
		// Verify worker exists
		workers := broker.GetWorkers()
		if len(workers) != 1 {
			t.Errorf("Expected 1 worker, got %d", len(workers))
		}
		
		// Remove worker
		err = broker.removeWorker("worker1")
		if err != nil {
			t.Errorf("Failed to remove worker: %v", err)
		}
		
		// Verify worker is gone
		workers = broker.GetWorkers()
		if len(workers) != 0 {
			t.Errorf("Expected 0 workers, got %d", len(workers))
		}
		
		// Service should still exist but with no workers
		services := broker.GetServices()
		if service, exists := services["test.service"]; exists {
			if len(service.Workers) != 0 {
				t.Errorf("Expected 0 workers in service, got %d", len(service.Workers))
			}
		}
	})
}

func TestMessageRouting(t *testing.T) {
	// These tests focus on message parsing and routing logic
	
	t.Run("ParseWorkerMessage", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Create a worker message
		msg := &WorkerMessage{
			Protocol: HERMES_WORKER,
			Command:  HERMES_READY,
			Service:  "test.service",
		}
		
		msgBytes, err := SerializeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to serialize message: %v", err)
		}
		
		// Test routing (this would be called with actual ZMQ message parts)
		err = broker.routeMessage("worker1", [][]byte{msgBytes})
		if err != nil {
			t.Errorf("Failed to route worker message: %v", err)
		}
		
		// Check that worker was registered
		workers := broker.GetWorkers()
		if len(workers) != 1 {
			t.Errorf("Expected 1 worker, got %d", len(workers))
		}
	})

	t.Run("ParseClientMessage", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// First register a service
		err := broker.handleWorkerReady("worker1", "test.service")
		if err != nil {
			t.Errorf("Failed to handle worker ready: %v", err)
		}
		
		// Create a client message
		msg := &ClientMessage{
			Protocol:  HERMES_CLIENT,
			Command:   HERMES_REQ,
			Service:   "test.service",
			MessageID: "msg_123",
			Body:      []byte("test request"),
		}
		
		msgBytes, err := SerializeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to serialize message: %v", err)
		}
		
		// Test routing
		err = broker.routeMessage("client1", [][]byte{msgBytes})
		if err != nil {
			t.Errorf("Failed to route client message: %v", err)
		}
		
		// In a real scenario, this would result in a message being sent to the worker
		// Here we just verify the message was processed without error
	})

	t.Run("InvalidMessage", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Invalid JSON
		invalidJSON := []byte(`{"invalid": json}`)
		
		err := broker.routeMessage("sender1", [][]byte{invalidJSON})
		if err == nil {
			t.Error("Expected error for invalid message format")
		}
	})
}

func TestServiceOperations(t *testing.T) {
	t.Run("ServiceCreation", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Register multiple workers for the same service
		err := broker.handleWorkerReady("worker1", "test.service")
		if err != nil {
			t.Errorf("Failed to register worker1: %v", err)
		}
		
		err = broker.handleWorkerReady("worker2", "test.service")
		if err != nil {
			t.Errorf("Failed to register worker2: %v", err)
		}
		
		// Check that both workers are registered
		workers := broker.GetWorkers()
		if len(workers) != 2 {
			t.Errorf("Expected 2 workers, got %d", len(workers))
		}
		
		// Check that service has both workers
		services := broker.GetServices()
		service := services["test.service"]
		if len(service.Workers) != 2 {
			t.Errorf("Expected 2 workers in service, got %d", len(service.Workers))
		}
	})

	t.Run("MultipleServices", func(t *testing.T) {
		broker := NewBroker("tcp://localhost:5555")
		
		// Register workers for different services
		err := broker.handleWorkerReady("worker1", "service.a")
		if err != nil {
			t.Errorf("Failed to register worker1: %v", err)
		}
		
		err = broker.handleWorkerReady("worker2", "service.b")
		if err != nil {
			t.Errorf("Failed to register worker2: %v", err)
		}
		
		// Check services
		services := broker.GetServices()
		if len(services) != 2 {
			t.Errorf("Expected 2 services, got %d", len(services))
		}
		
		if _, exists := services["service.a"]; !exists {
			t.Error("Expected service.a to exist")
		}
		
		if _, exists := services["service.b"]; !exists {
			t.Error("Expected service.b to exist")
		}
	})
}

// Benchmark tests for broker operations
func BenchmarkWorkerRegistration(b *testing.B) {
	broker := NewBroker("tcp://localhost:5555")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workerID := fmt.Sprintf("worker%d", i)
		err := broker.handleWorkerReady(workerID, "test.service")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMessageRouting(b *testing.B) {
	broker := NewBroker("tcp://localhost:5555")
	
	// Create a serialized message
	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_HEARTBEAT,
	}
	msgBytes, _ := SerializeMessage(msg)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		broker.routeMessage("worker1", [][]byte{msgBytes})
	}
}