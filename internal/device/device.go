package device

import (
	"encoding/json"
	"fmt"
)

// Device represents a generic device that can process commands
type Device interface {
	// Process handles a JSON-encoded action and executes the corresponding operation
	Process(actionJSON []byte) (*ActionResponse, error)

	// GetDeviceInfo returns basic information about the device
	GetDeviceInfo() DeviceInfo
}

// DeviceInfo contains basic information about a device
type DeviceInfo struct {
	Type         string   `json:"type"`
	Model        string   `json:"model"`
	Address      string   `json:"address"`
	Capabilities []string `json:"capabilities"`
}

// ActionType represents the type of action to perform
type ActionType string

const (
	ActionTypeRemote  ActionType = "remote"
	ActionTypeControl ActionType = "control"
)

// ActionRequest represents a JSON action request
type ActionRequest struct {
	Type       ActionType             `json:"type"`       // "remote" or "control"
	Action     string                 `json:"action"`     // specific action name
	Parameters map[string]interface{} `json:"parameters"` // optional parameters
}

// ActionResponse represents the response from processing an action
type ActionResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// RemoteAction represents available remote control actions
type RemoteAction string

const (
	RemoteActionPower       RemoteAction = "power"
	RemoteActionPowerOn     RemoteAction = "power_on"
	RemoteActionPowerOff    RemoteAction = "power_off"
	RemoteActionVolumeUp    RemoteAction = "volume_up"
	RemoteActionVolumeDown  RemoteAction = "volume_down"
	RemoteActionMute        RemoteAction = "mute"
	RemoteActionChannelUp   RemoteAction = "channel_up"
	RemoteActionChannelDown RemoteAction = "channel_down"
	RemoteActionUp          RemoteAction = "up"
	RemoteActionDown        RemoteAction = "down"
	RemoteActionLeft        RemoteAction = "left"
	RemoteActionRight       RemoteAction = "right"
	RemoteActionConfirm     RemoteAction = "confirm"
	RemoteActionHome        RemoteAction = "home"
	RemoteActionMenu        RemoteAction = "menu"
	RemoteActionBack        RemoteAction = "back"
	RemoteActionInput       RemoteAction = "input"
	RemoteActionHDMI1       RemoteAction = "hdmi1"
	RemoteActionHDMI2       RemoteAction = "hdmi2"
	RemoteActionHDMI3       RemoteAction = "hdmi3"
	RemoteActionHDMI4       RemoteAction = "hdmi4"
)

// ControlAction represents available control API actions
type ControlAction string

const (
	ControlActionPowerStatus    ControlAction = "power_status"
	ControlActionSystemInfo     ControlAction = "system_info"
	ControlActionVolumeInfo     ControlAction = "volume_info"
	ControlActionPlayingContent ControlAction = "playing_content"
	ControlActionAppList        ControlAction = "app_list"
	ControlActionContentList    ControlAction = "content_list"
	ControlActionSetVolume      ControlAction = "set_volume"
	ControlActionSetMute        ControlAction = "set_mute"
)

// parseActionRequest parses JSON input into ActionRequest
func parseActionRequest(actionJSON []byte) (*ActionRequest, error) {
	var request ActionRequest
	if err := json.Unmarshal(actionJSON, &request); err != nil {
		return nil, fmt.Errorf("failed to parse action request: %w", err)
	}

	// Validate required fields
	if request.Type == "" {
		return nil, fmt.Errorf("action type is required")
	}

	if request.Action == "" {
		return nil, fmt.Errorf("action is required")
	}

	return &request, nil
}
