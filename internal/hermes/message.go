package hermes

import (
	"encoding/json"
	"fmt"
	"time"
)

// MessageBuilder helps create Hermes protocol messages
type MessageBuilder struct {
	protocol string
}

// NewMessageBuilder creates a new message builder
func NewMessageBuilder(protocol string) *MessageBuilder {
	return &MessageBuilder{protocol: protocol}
}

// BuildWorkerReady creates a READY message for worker registration
func (mb *MessageBuilder) BuildWorkerReady(service string) *WorkerMessage {
	return &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_READY,
		Service:  service,
	}
}

// BuildWorkerReply creates a REPLY message for worker responses
func (mb *MessageBuilder) BuildWorkerReply(clientID string, body []byte) *WorkerMessage {
	return &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_REPLY,
		Body:     body,
		ClientID: clientID,
	}
}

// BuildWorkerHeartbeat creates a HEARTBEAT message for worker liveness
func (mb *MessageBuilder) BuildWorkerHeartbeat() *WorkerMessage {
	return &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_HEARTBEAT,
	}
}

// BuildWorkerDisconnect creates a DISCONNECT message for graceful worker shutdown
func (mb *MessageBuilder) BuildWorkerDisconnect() *WorkerMessage {
	return &WorkerMessage{
		Protocol: HERMES_WORKER,
		Command:  HERMES_DISCONNECT,
	}
}

// BuildClientRequest creates a REQ message for client requests
func (mb *MessageBuilder) BuildClientRequest(service string, messageID string, body []byte) *ClientMessage {
	return &ClientMessage{
		Protocol:  HERMES_CLIENT,
		Command:   HERMES_REQ,
		Service:   service,
		MessageID: messageID,
		Body:      body,
	}
}

// SerializeMessage serializes a message to JSON bytes
func SerializeMessage(msg interface{}) ([]byte, error) {
	return json.Marshal(msg)
}

// DeserializeWorkerMessage deserializes JSON bytes to WorkerMessage
func DeserializeWorkerMessage(data []byte) (*WorkerMessage, error) {
	var msg WorkerMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to deserialize worker message: %w", err)
	}
	return &msg, nil
}

// DeserializeClientMessage deserializes JSON bytes to ClientMessage
func DeserializeClientMessage(data []byte) (*ClientMessage, error) {
	var msg ClientMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("failed to deserialize client message: %w", err)
	}
	return &msg, nil
}

// DeserializeServiceRequest deserializes JSON bytes to ServiceRequest
func DeserializeServiceRequest(data []byte) (*ServiceRequest, error) {
	var req ServiceRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to deserialize service request: %w", err)
	}
	return &req, nil
}

// SerializeServiceResponse serializes a ServiceResponse to JSON bytes
func SerializeServiceResponse(resp *ServiceResponse) ([]byte, error) {
	return json.Marshal(resp)
}

// CreateServiceRequest creates a new ServiceRequest
func CreateServiceRequest(service, action string, payload interface{}) (*ServiceRequest, error) {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	return &ServiceRequest{
		MessageID: GenerateMessageID(),
		Service:   service,
		Action:    action,
		Payload:   payloadBytes,
	}, nil
}

// CreateServiceResponse creates a new ServiceResponse
func CreateServiceResponse(messageID, service string, success bool, data interface{}, err error) *ServiceResponse {
	resp := &ServiceResponse{
		MessageID: messageID,
		Service:   service,
		Success:   success,
		Data:      data,
	}
	
	if err != nil {
		resp.Error = err.Error()
	}
	
	return resp
}

// GenerateMessageID generates a unique message ID
func GenerateMessageID() string {
	return fmt.Sprintf("msg_%d", time.Now().UnixNano())
}

// ValidateMessage validates a Hermes message
func ValidateMessage(msg interface{}) error {
	switch m := msg.(type) {
	case *WorkerMessage:
		return validateWorkerMessage(m)
	case *ClientMessage:
		return validateClientMessage(m)
	case *ServiceRequest:
		return validateServiceRequest(m)
	default:
		return fmt.Errorf("unknown message type")
	}
}

// validateWorkerMessage validates a worker message
func validateWorkerMessage(msg *WorkerMessage) error {
	if msg.Protocol != HERMES_WORKER {
		return fmt.Errorf("invalid protocol: %s", msg.Protocol)
	}
	
	switch msg.Command {
	case HERMES_READY:
		if msg.Service == "" {
			return fmt.Errorf("service required for READY command")
		}
	case HERMES_REPLY:
		if msg.ClientID == "" {
			return fmt.Errorf("client_id required for REPLY command")
		}
	case HERMES_HEARTBEAT, HERMES_DISCONNECT:
		// No additional validation required
	default:
		return fmt.Errorf("unknown worker command: %s", msg.Command)
	}
	
	return nil
}

// validateClientMessage validates a client message
func validateClientMessage(msg *ClientMessage) error {
	if msg.Protocol != HERMES_CLIENT {
		return fmt.Errorf("invalid protocol: %s", msg.Protocol)
	}
	
	switch msg.Command {
	case HERMES_REQ:
		if msg.Service == "" {
			return fmt.Errorf("service required for REQ command")
		}
		if msg.MessageID == "" {
			return fmt.Errorf("message_id required for REQ command")
		}
	default:
		return fmt.Errorf("unknown client command: %s", msg.Command)
	}
	
	return nil
}

// validateServiceRequest validates a service request
func validateServiceRequest(req *ServiceRequest) error {
	if req.MessageID == "" {
		return fmt.Errorf("message_id is required")
	}
	if req.Service == "" {
		return fmt.Errorf("service is required")
	}
	if req.Action == "" {
		return fmt.Errorf("action is required")
	}
	return nil
}