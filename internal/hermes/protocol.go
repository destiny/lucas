package hermes

import (
	"encoding/json"
	"time"
)

// Hermes Protocol Constants
const (
	// Protocol versions
	HERMES_CLIENT = "HERMES01"
	HERMES_WORKER = "HERMESW01"
	
	// Worker commands
	HERMES_READY      = "READY"
	HERMES_REQUEST    = "REQUEST"
	HERMES_REPLY      = "REPLY"
	HERMES_HEARTBEAT  = "HEARTBEAT"
	HERMES_DISCONNECT = "DISCONNECT"
	
	// Client commands
	HERMES_REQ = "REQ"
	HERMES_REP = "REP"
	
	// Service lifecycle
	HERMES_SERVICE_UP   = "SERVICE_UP"
	HERMES_SERVICE_DOWN = "SERVICE_DOWN"
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
	Protocol  string `json:"protocol"`
	Command   string `json:"command"`
	Service   string `json:"service,omitempty"`
	Body      []byte `json:"body,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
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
	Identity    string    `json:"identity"`
	Service     string    `json:"service"`
	Address     string    `json:"address,omitempty"`
	Expiry      time.Time `json:"expiry"`
	LastPing    time.Time `json:"last_ping"`
	Status      string    `json:"status"`
	Liveness    int       `json:"liveness"`
	Requests    int       `json:"requests"`
}

// BrokerStats represents broker statistics
type BrokerStats struct {
	Services          int       `json:"services"`
	Workers           int       `json:"workers"`
	Requests          int       `json:"requests"`
	Responses         int       `json:"responses"`
	HeartbeatsReceived int       `json:"heartbeats_received"`
	HeartbeatsSent    int       `json:"heartbeats_sent"`
	StartTime         time.Time `json:"start_time"`
	LastRequest       time.Time `json:"last_request"`
	LastHeartbeat     time.Time `json:"last_heartbeat"`
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