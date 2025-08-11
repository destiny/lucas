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

func TestMessageBuilder(t *testing.T) {
	mb := NewMessageBuilder(HERMES_WORKER)

	t.Run("BuildWorkerReady", func(t *testing.T) {
		msg := mb.BuildWorkerReady("test.service")
		
		if msg.Protocol != HERMES_WORKER {
			t.Errorf("Expected protocol %s, got %s", HERMES_WORKER, msg.Protocol)
		}
		if msg.Command != HERMES_READY {
			t.Errorf("Expected command %s, got %s", HERMES_READY, msg.Command)
		}
		if msg.Service != "test.service" {
			t.Errorf("Expected service 'test.service', got %s", msg.Service)
		}
	})

	t.Run("BuildWorkerReply", func(t *testing.T) {
		body := []byte("test response")
		msg := mb.BuildWorkerReply("client123", body)
		
		if msg.Protocol != HERMES_WORKER {
			t.Errorf("Expected protocol %s, got %s", HERMES_WORKER, msg.Protocol)
		}
		if msg.Command != HERMES_REPLY {
			t.Errorf("Expected command %s, got %s", HERMES_REPLY, msg.Command)
		}
		if msg.ClientID != "client123" {
			t.Errorf("Expected client_id 'client123', got %s", msg.ClientID)
		}
		if string(msg.Body) != "test response" {
			t.Errorf("Expected body 'test response', got %s", string(msg.Body))
		}
	})

	t.Run("BuildWorkerHeartbeat", func(t *testing.T) {
		msg := mb.BuildWorkerHeartbeat()
		
		if msg.Protocol != HERMES_WORKER {
			t.Errorf("Expected protocol %s, got %s", HERMES_WORKER, msg.Protocol)
		}
		if msg.Command != HERMES_HEARTBEAT {
			t.Errorf("Expected command %s, got %s", HERMES_HEARTBEAT, msg.Command)
		}
	})
}

func TestMessageValidation(t *testing.T) {
	t.Run("ValidWorkerMessage", func(t *testing.T) {
		msg := &WorkerMessage{
			Protocol: HERMES_WORKER,
			Command:  HERMES_READY,
			Service:  "test.service",
		}
		
		if err := ValidateMessage(msg); err != nil {
			t.Errorf("Expected valid message, got error: %v", err)
		}
	})

	t.Run("InvalidProtocol", func(t *testing.T) {
		msg := &WorkerMessage{
			Protocol: "INVALID",
			Command:  HERMES_READY,
			Service:  "test.service",
		}
		
		if err := ValidateMessage(msg); err == nil {
			t.Error("Expected validation error for invalid protocol")
		}
	})

	t.Run("MissingService", func(t *testing.T) {
		msg := &WorkerMessage{
			Protocol: HERMES_WORKER,
			Command:  HERMES_READY,
			// Service missing
		}
		
		if err := ValidateMessage(msg); err == nil {
			t.Error("Expected validation error for missing service")
		}
	})

	t.Run("ValidClientMessage", func(t *testing.T) {
		msg := &ClientMessage{
			Protocol:  HERMES_CLIENT,
			Command:   HERMES_REQ,
			Service:   "test.service",
			MessageID: "msg_123",
			Body:      []byte("test"),
		}
		
		if err := ValidateMessage(msg); err != nil {
			t.Errorf("Expected valid message, got error: %v", err)
		}
	})

	t.Run("ValidServiceRequest", func(t *testing.T) {
		req := &ServiceRequest{
			MessageID: "msg_123",
			Service:   "test.service",
			Action:    "execute",
			Payload:   []byte(`{"test": true}`),
		}
		
		if err := ValidateMessage(req); err != nil {
			t.Errorf("Expected valid message, got error: %v", err)
		}
	})
}

func TestMessageSerialization(t *testing.T) {
	t.Run("SerializeWorkerMessage", func(t *testing.T) {
		msg := &WorkerMessage{
			Protocol: HERMES_WORKER,
			Command:  HERMES_READY,
			Service:  "test.service",
		}
		
		data, err := SerializeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to serialize message: %v", err)
		}
		
		// Deserialize and compare
		deserialized, err := DeserializeWorkerMessage(data)
		if err != nil {
			t.Fatalf("Failed to deserialize message: %v", err)
		}
		
		if deserialized.Protocol != msg.Protocol {
			t.Errorf("Protocol mismatch: expected %s, got %s", msg.Protocol, deserialized.Protocol)
		}
		if deserialized.Command != msg.Command {
			t.Errorf("Command mismatch: expected %s, got %s", msg.Command, deserialized.Command)
		}
		if deserialized.Service != msg.Service {
			t.Errorf("Service mismatch: expected %s, got %s", msg.Service, deserialized.Service)
		}
	})

	t.Run("SerializeClientMessage", func(t *testing.T) {
		msg := &ClientMessage{
			Protocol:  HERMES_CLIENT,
			Command:   HERMES_REQ,
			Service:   "test.service",
			MessageID: "msg_123",
			Body:      []byte("test body"),
		}
		
		data, err := SerializeMessage(msg)
		if err != nil {
			t.Fatalf("Failed to serialize message: %v", err)
		}
		
		// Deserialize and compare
		deserialized, err := DeserializeClientMessage(data)
		if err != nil {
			t.Fatalf("Failed to deserialize message: %v", err)
		}
		
		if deserialized.Protocol != msg.Protocol {
			t.Errorf("Protocol mismatch: expected %s, got %s", msg.Protocol, deserialized.Protocol)
		}
		if deserialized.MessageID != msg.MessageID {
			t.Errorf("MessageID mismatch: expected %s, got %s", msg.MessageID, deserialized.MessageID)
		}
		if string(deserialized.Body) != string(msg.Body) {
			t.Errorf("Body mismatch: expected %s, got %s", string(msg.Body), string(deserialized.Body))
		}
	})
}

func TestServiceRequestCreation(t *testing.T) {
	t.Run("CreateServiceRequest", func(t *testing.T) {
		payload := map[string]interface{}{
			"device_id": "test_device",
			"command":   "power_on",
		}
		
		req, err := CreateServiceRequest("device.bravia", "execute", payload)
		if err != nil {
			t.Fatalf("Failed to create service request: %v", err)
		}
		
		if req.Service != "device.bravia" {
			t.Errorf("Expected service 'device.bravia', got %s", req.Service)
		}
		if req.Action != "execute" {
			t.Errorf("Expected action 'execute', got %s", req.Action)
		}
		if req.MessageID == "" {
			t.Error("Expected non-empty message ID")
		}
		if len(req.Payload) == 0 {
			t.Error("Expected non-empty payload")
		}
	})
}

func TestServiceResponseCreation(t *testing.T) {
	t.Run("CreateSuccessResponse", func(t *testing.T) {
		data := map[string]interface{}{
			"result": "success",
			"value":  42,
		}
		
		resp := CreateServiceResponse("msg_123", "test.service", true, data, nil)
		
		if resp.MessageID != "msg_123" {
			t.Errorf("Expected message ID 'msg_123', got %s", resp.MessageID)
		}
		if resp.Service != "test.service" {
			t.Errorf("Expected service 'test.service', got %s", resp.Service)
		}
		if !resp.Success {
			t.Error("Expected success to be true")
		}
		if resp.Data == nil {
			t.Error("Expected data to be set")
		}
		if resp.Error != "" {
			t.Errorf("Expected empty error, got %s", resp.Error)
		}
	})

	t.Run("CreateErrorResponse", func(t *testing.T) {
		testErr := fmt.Errorf("test error")
		
		resp := CreateServiceResponse("msg_456", "test.service", false, nil, testErr)
		
		if resp.MessageID != "msg_456" {
			t.Errorf("Expected message ID 'msg_456', got %s", resp.MessageID)
		}
		if resp.Success {
			t.Error("Expected success to be false")
		}
		if resp.Error != testErr.Error() {
			t.Errorf("Expected error '%s', got %s", testErr.Error(), resp.Error)
		}
	})
}

func TestGenerateMessageID(t *testing.T) {
	t.Run("UniqueIDs", func(t *testing.T) {
		id1 := GenerateMessageID()
		time.Sleep(1 * time.Millisecond) // Ensure different timestamps
		id2 := GenerateMessageID()
		
		if id1 == id2 {
			t.Error("Expected unique message IDs")
		}
		if id1 == "" || id2 == "" {
			t.Error("Expected non-empty message IDs")
		}
	})
}

// Benchmark tests
func BenchmarkSerializeMessage(b *testing.B) {
	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_READY,
		Service:  "test.service",
		Body:     make([]byte, 1024), // 1KB payload
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := SerializeMessage(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDeserializeWorkerMessage(b *testing.B) {
	msg := &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_READY,
		Service:  "test.service",
		Body:     make([]byte, 1024), // 1KB payload
	}
	
	data, err := SerializeMessage(msg)
	if err != nil {
		b.Fatal(err)
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DeserializeWorkerMessage(data)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGenerateMessageID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateMessageID()
	}
}