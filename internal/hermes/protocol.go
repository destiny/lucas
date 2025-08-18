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
	"encoding/json"
	"time"
)

// Hermes Protocol Constants - Enhanced RFC 7/MDP Compliance
const (
	// Protocol versions (RFC 7/MDP compliant)
	HERMES_CLIENT = "MDPC01"    // Majordomo Protocol Client v0.1
	HERMES_WORKER = "MDPW01"    // Majordomo Protocol Worker v0.1

	// Worker commands (RFC 7/MDP standard)
	HERMES_READY      = "\x01"  // Worker is ready for work
	HERMES_REQUEST    = "\x02"  // Request from broker to worker
	HERMES_REPLY      = "\x03"  // Reply from worker to broker
	HERMES_HEARTBEAT  = "\x04"  // Heartbeat between worker and broker
	HERMES_DISCONNECT = "\x05"  // Worker disconnecting

	// Client commands (RFC 7/MDP standard)
	HERMES_REQ = "\x01"  // Client request
	HERMES_REP = "\x02"  // Client reply

	// Service lifecycle (extended)
	HERMES_SERVICE_UP   = "SERVICE_UP"
	HERMES_SERVICE_DOWN = "SERVICE_DOWN"
	
	// Protocol frame markers (RFC 7/MDP compliance)
	MDP_CLIENT_HEADER = "MDPC01"
	MDP_WORKER_HEADER = "MDPW01"
	
	// Standard timing constants (RFC recommendations)
	MDP_HEARTBEAT_LIVENESS  = 3     // Heartbeats before considering worker dead
)

// Message represents a Hermes protocol message
type Message struct {
	Protocol  string    `json:"protocol"`
	Type      string    `json:"type"`
	Service   string    `json:"service,omitempty"`
	Body      []byte    `json:"body,omitempty"`
	ReplyTo   string    `json:"reply_to,omitempty"`
	MessageID string    `json:"message_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// WorkerMessage represents a message from worker to broker
type WorkerMessage struct {
	Protocol string `json:"protocol"`
	Command  string `json:"command"`
	Service  string `json:"service,omitempty"`
	Body     []byte `json:"body,omitempty"`
	ClientID string `json:"client_id,omitempty"`
}

// ClientMessage represents a message from client to broker
type ClientMessage struct {
	Protocol  string `json:"protocol"`
	Command   string `json:"command"`
	Service   string `json:"service"`
	Body      []byte `json:"body"`
	MessageID string `json:"message_id"`
}

// ServiceRequest represents a service request
type ServiceRequest struct {
	MessageID string          `json:"message_id"`
	Service   string          `json:"service"`
	Action    string          `json:"action"`
	Payload   json.RawMessage `json:"payload"`
	Nonce     string          `json:"nonce,omitempty"`
	Timeout   int             `json:"timeout,omitempty"` // seconds
}

// ServiceResponse represents a service response
type ServiceResponse struct {
	MessageID string      `json:"message_id"`
	Service   string      `json:"service"`
	Success   bool        `json:"success"`
	Data      interface{} `json:"data,omitempty"`
	Error     string      `json:"error,omitempty"`
	Nonce     string      `json:"nonce,omitempty"`
}

// ServiceInfo represents information about a service
type ServiceInfo struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	Capabilities []string  `json:"capabilities"`
	Workers      []string  `json:"workers"`
	LastSeen     time.Time `json:"last_seen"`
	Status       string    `json:"status"`
}

// WorkerInfo represents information about a worker
type WorkerInfo struct {
	Identity string    `json:"identity"`
	Service  string    `json:"service"`
	Address  string    `json:"address,omitempty"`
	Expiry   time.Time `json:"expiry"`
	LastPing time.Time `json:"last_ping"`
	Status   string    `json:"status"`
	Liveness int       `json:"liveness"`
	Requests int       `json:"requests"`
}

// BrokerStats represents broker statistics
type BrokerStats struct {
	Services           int       `json:"services"`
	Workers            int       `json:"workers"`
	Requests           int       `json:"requests"`
	Responses          int       `json:"responses"`
	HeartbeatsReceived int       `json:"heartbeats_received"`
	HeartbeatsSent     int       `json:"heartbeats_sent"`
	StartTime          time.Time `json:"start_time"`
	LastRequest        time.Time `json:"last_request"`
	LastHeartbeat      time.Time `json:"last_heartbeat"`
}

// RequestHandler interface for handling service requests
type RequestHandler interface {
	Handle(request []byte) ([]byte, error)
}

// ServiceRegistry interface for managing services
type ServiceRegistry interface {
	RegisterService(name string, info *ServiceInfo) error
	UnregisterService(name string) error
	GetService(name string) (*ServiceInfo, bool)
	ListServices() map[string]*ServiceInfo
}

// WorkerRegistry interface for managing workers
type WorkerRegistry interface {
	RegisterWorker(identity string, service string, info *WorkerInfo) error
	UnregisterWorker(identity string) error
	GetWorker(identity string) (*WorkerInfo, bool)
	GetWorkersForService(service string) []*WorkerInfo
	UpdateWorkerHeartbeat(identity string) error
}

// RFC 7/MDP Compliance Helper Functions

// GetMDPHeartbeatInterval returns the recommended heartbeat interval (2.5 seconds)
func GetMDPHeartbeatInterval() time.Duration {
	return 2500 * time.Millisecond
}

// GetMDPHeartbeatExpiry returns the heartbeat expiry duration (7.5 seconds = 3 * 2.5s)
func GetMDPHeartbeatExpiry() time.Duration {
	return GetMDPHeartbeatInterval() * MDP_HEARTBEAT_LIVENESS
}

// IsValidMDPWorkerCommand checks if a command is valid according to RFC 7/MDP
func IsValidMDPWorkerCommand(command string) bool {
	switch command {
	case HERMES_READY, HERMES_REQUEST, HERMES_REPLY, HERMES_HEARTBEAT, HERMES_DISCONNECT:
		return true
	default:
		return false
	}
}

// IsValidMDPClientCommand checks if a command is valid according to RFC 7/MDP
func IsValidMDPClientCommand(command string) bool {
	switch command {
	case HERMES_REQ, HERMES_REP:
		return true
	default:
		return false
	}
}

// FormatMDPWorkerFrame formats a worker message frame according to RFC 7/MDP
// Frame format: [empty, protocol, command, service?, body?]
func FormatMDPWorkerFrame(command, service string, body []byte) [][]byte {
	frames := [][]byte{
		[]byte(""),           // Empty frame
		[]byte(MDP_WORKER_HEADER), // Protocol header
		[]byte(command),      // Command
	}
	
	if service != "" {
		frames = append(frames, []byte(service))
	}
	
	if len(body) > 0 {
		frames = append(frames, body)
	}
	
	return frames
}

// FormatMDPClientFrame formats a client message frame according to RFC 7/MDP
// Frame format: [empty, protocol, command, service, body]
func FormatMDPClientFrame(command, service string, body []byte) [][]byte {
	return [][]byte{
		[]byte(""),           // Empty frame
		[]byte(MDP_CLIENT_HEADER), // Protocol header
		[]byte(command),      // Command
		[]byte(service),      // Service name
		body,                 // Request body
	}
}
