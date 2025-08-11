package hermes_test

import (
	"testing"

	"lucas/internal/hermes"
)

func TestNewBroker(t *testing.T) {
	t.Run("creates broker successfully", func(t *testing.T) {
		broker := hermes.NewBroker("tcp://localhost:5555")
		if broker == nil {
			t.Error("Expected broker to be created")
		}
	})

	t.Run("creates broker with different addresses", func(t *testing.T) {
		addresses := []string{
			"tcp://*:5555",
			"tcp://127.0.0.1:5556", 
			"tcp://0.0.0.0:5557",
		}

		for _, addr := range addresses {
			broker := hermes.NewBroker(addr)
			if broker == nil {
				t.Errorf("Expected broker to be created for address %s", addr)
			}
		}
	})
}

func TestBrokerLifecycle(t *testing.T) {
	t.Run("start and stop broker", func(t *testing.T) {
		broker := hermes.NewBroker("tcp://localhost:5558")
		
		// Test starting the broker
		if err := broker.Start(); err != nil {
			t.Fatalf("Failed to start broker: %v", err)
		}

		// Test stopping the broker
		if err := broker.Stop(); err != nil {
			t.Fatalf("Failed to stop broker: %v", err)
		}
	})
}