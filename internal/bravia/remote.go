package bravia

import (
	"encoding/json"
	"fmt"
	"io"
	"lucas/internal/device"
	"strconv"
)

// BraviaRemote implements the Device interface for Sony Bravia TVs
type BraviaRemote struct {
	client *BraviaClient
	info   device.DeviceInfo
}

// NewBraviaRemote creates a new BraviaRemote device
func NewBraviaRemote(address, credential string, debug bool) *BraviaRemote {
	client := NewBraviaClient(address, credential, debug)

	return &BraviaRemote{
		client: client,
		info: device.DeviceInfo{
			Type:    "bravia_tv",
			Model:   "Sony Bravia",
			Address: address,
			Capabilities: []string{
				"remote_control",
				"system_control",
				"audio_control",
				"content_control",
				"app_control",
			},
		},
	}
}

// GetDeviceInfo returns information about this Bravia device
func (br *BraviaRemote) GetDeviceInfo() device.DeviceInfo {
	return br.info
}

// Process handles JSON action requests and routes them to appropriate methods
func (br *BraviaRemote) Process(actionJSON []byte) (*device.ActionResponse, error) {
	// Parse the action request
	request, err := parseActionRequest(actionJSON)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Route based on action type
	switch request.Type {
	case device.ActionTypeRemote:
		return br.processRemoteAction(request)
	case device.ActionTypeControl:
		return br.processControlAction(request)
	default:
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("unsupported action type: %s", request.Type),
		}, nil
	}
}

// processRemoteAction handles remote control actions
func (br *BraviaRemote) processRemoteAction(request *device.ActionRequest) (*device.ActionResponse, error) {
	// Convert action string to RemoteAction
	remoteAction := device.RemoteAction(request.Action)

	// Look up the corresponding BraviaRemoteCode
	code, exists := remoteActionMap[remoteAction]
	if !exists {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("unsupported remote action: %s", request.Action),
		}, nil
	}

	// Execute the remote request
	err := br.client.RemoteRequest(code)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("remote request failed: %v", err),
		}, nil
	}

	return &device.ActionResponse{
		Success: true,
		Data:    fmt.Sprintf("Remote action '%s' executed successfully", request.Action),
	}, nil
}

// processControlAction handles API control actions
func (br *BraviaRemote) processControlAction(request *device.ActionRequest) (*device.ActionResponse, error) {
	// Convert action string to ControlAction
	controlAction := device.ControlAction(request.Action)

	// Look up the corresponding endpoint and method
	actionInfo, exists := controlActionMap[controlAction]
	if !exists {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("unsupported control action: %s", request.Action),
		}, nil
	}

	// Create payload with parameters
	params, err := br.buildControlParameters(controlAction, request.Parameters)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("invalid parameters: %v", err),
		}, nil
	}

	payload := CreatePayload(1, actionInfo.method, params)

	// Execute the control request
	resp, err := br.client.ControlRequest(actionInfo.endpoint, payload)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("control request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read and parse response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &device.ActionResponse{
			Success: false,
			Error:   fmt.Sprintf("failed to read response: %v", err),
		}, nil
	}

	// Parse JSON response
	var responseData interface{}
	if err := json.Unmarshal(responseBody, &responseData); err != nil {
		// If JSON parsing fails, return raw response
		return &device.ActionResponse{
			Success: true,
			Data:    string(responseBody),
		}, nil
	}

	return &device.ActionResponse{
		Success: true,
		Data:    responseData,
	}, nil
}

// buildControlParameters builds parameters for control actions
func (br *BraviaRemote) buildControlParameters(action device.ControlAction, requestParams map[string]interface{}) ([]map[string]string, error) {
	params := []map[string]string{}

	switch action {
	case device.ControlActionSetVolume:
		// Volume setting requires a volume parameter
		if requestParams != nil {
			if volume, exists := requestParams["volume"]; exists {
				// Convert volume to string
				var volumeStr string
				switch v := volume.(type) {
				case int:
					volumeStr = strconv.Itoa(v)
				case float64:
					volumeStr = strconv.Itoa(int(v))
				case string:
					volumeStr = v
				default:
					return nil, fmt.Errorf("invalid volume parameter type")
				}

				params = append(params, map[string]string{
					"target": "speaker",
					"volume": volumeStr,
				})
			} else {
				return nil, fmt.Errorf("volume parameter is required for set_volume action")
			}
		} else {
			return nil, fmt.Errorf("parameters are required for set_volume action")
		}

	case device.ControlActionSetMute:
		// Mute setting requires a status parameter
		if requestParams != nil {
			if status, exists := requestParams["status"]; exists {
				// Convert status to string
				var statusStr string
				switch s := status.(type) {
				case bool:
					if s {
						statusStr = "true"
					} else {
						statusStr = "false"
					}
				case string:
					statusStr = s
				default:
					return nil, fmt.Errorf("invalid status parameter type")
				}

				params = append(params, map[string]string{
					"status": statusStr,
				})
			} else {
				return nil, fmt.Errorf("status parameter is required for set_mute action")
			}
		} else {
			return nil, fmt.Errorf("parameters are required for set_mute action")
		}

	default:
		// Most actions don't require parameters
		// If parameters are provided, we can try to convert them
		if requestParams != nil {
			param := make(map[string]string)
			for key, value := range requestParams {
				switch v := value.(type) {
				case string:
					param[key] = v
				case int:
					param[key] = strconv.Itoa(v)
				case float64:
					param[key] = strconv.FormatFloat(v, 'f', -1, 64)
				case bool:
					param[key] = strconv.FormatBool(v)
				default:
					param[key] = fmt.Sprintf("%v", v)
				}
			}
			if len(param) > 0 {
				params = append(params, param)
			}
		}
	}

	return params, nil
}

// parseActionRequest parses JSON input into ActionRequest
func parseActionRequest(actionJSON []byte) (*device.ActionRequest, error) {
	var request device.ActionRequest
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


// remoteActionMap maps RemoteAction to BraviaRemoteCode
var remoteActionMap = map[device.RemoteAction]BraviaRemoteCode{
	device.RemoteActionPower:       PowerButton,
	device.RemoteActionPowerOn:     PowerOn,
	device.RemoteActionPowerOff:    PowerOff,
	device.RemoteActionVolumeUp:    VolumeUp,
	device.RemoteActionVolumeDown:  VolumeDown,
	device.RemoteActionMute:        Mute,
	device.RemoteActionChannelUp:   ChannelUp,
	device.RemoteActionChannelDown: ChannelDown,
	device.RemoteActionUp:          Up,
	device.RemoteActionDown:        Down,
	device.RemoteActionLeft:        Left,
	device.RemoteActionRight:       Right,
	device.RemoteActionConfirm:     Confirm,
	device.RemoteActionHome:        Home,
	device.RemoteActionMenu:        Menu,
	device.RemoteActionBack:        Back,
	device.RemoteActionInput:       Input,
	device.RemoteActionHDMI1:       HDMI1,
	device.RemoteActionHDMI2:       HDMI2,
	device.RemoteActionHDMI3:       HDMI3,
	device.RemoteActionHDMI4:       HDMI4,
}

// controlActionMap maps ControlAction to endpoint and method
type controlActionInfo struct {
	endpoint BraviaEndpoint
	method   BraviaMethod
}

var controlActionMap = map[device.ControlAction]controlActionInfo{
	device.ControlActionPowerStatus: {
		endpoint: SystemEndpoint,
		method:   GetPowerStatus,
	},
	device.ControlActionSystemInfo: {
		endpoint: SystemEndpoint,
		method:   GetSystemInformation,
	},
	device.ControlActionVolumeInfo: {
		endpoint: AudioEndpoint,
		method:   GetVolumeInformation,
	},
	device.ControlActionPlayingContent: {
		endpoint: AVContentEndpoint,
		method:   GetPlayingContentInfo,
	},
	device.ControlActionAppList: {
		endpoint: AppControlEndpoint,
		method:   GetApplicationList,
	},
	device.ControlActionContentList: {
		endpoint: AVContentEndpoint,
		method:   GetContentList,
	},
	device.ControlActionSetVolume: {
		endpoint: AudioEndpoint,
		method:   SetAudioVolume,
	},
	device.ControlActionSetMute: {
		endpoint: AudioEndpoint,
		method:   SetAudioMute,
	},
}
