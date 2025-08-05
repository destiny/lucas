package bravia

import (
	"encoding/json"
	"fmt"
	"lucas/internal/device"
)

// Example demonstrates how to use the Device interface and BraviaRemote
func Example() {
	// Create a new Bravia device
	device := NewBraviaRemote("192.168.1.100:80", "0000", false)

	// Get device information
	info := device.GetDeviceInfo()
	fmt.Printf("Device: %s %s at %s\n", info.Model, info.Type, info.Address)
	fmt.Printf("Capabilities: %v\n", info.Capabilities)

	// Example 1: Remote control action
	remoteActionJSON := `{
		"type": "remote",
		"action": "power"
	}`

	response, err := device.Process([]byte(remoteActionJSON))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if response.Success {
		fmt.Printf("Remote action successful: %v\n", response.Data)
	} else {
		fmt.Printf("Remote action failed: %s\n", response.Error)
	}

	// Example 2: Control API action
	controlActionJSON := `{
		"type": "control",
		"action": "power_status"
	}`

	response, err = device.Process([]byte(controlActionJSON))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if response.Success {
		fmt.Printf("Control action successful: %v\n", response.Data)
	} else {
		fmt.Printf("Control action failed: %s\n", response.Error)
	}

	// Example 3: Control action with parameters
	volumeActionJSON := `{
		"type": "control",
		"action": "set_volume",
		"parameters": {
			"volume": 50
		}
	}`

	response, err = device.Process([]byte(volumeActionJSON))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if response.Success {
		fmt.Printf("Volume set successfully: %v\n", response.Data)
	} else {
		fmt.Printf("Volume set failed: %s\n", response.Error)
	}
}

// CreateActionJSON is a helper function to create action JSON strings
func CreateActionJSON(actionType device.ActionType, action string, parameters map[string]interface{}) ([]byte, error) {
	request := device.ActionRequest{
		Type:       actionType,
		Action:     action,
		Parameters: parameters,
	}

	return json.Marshal(request)
}

// Available actions for reference
var AvailableRemoteActions = []string{
	"power", "power_on", "power_off",
	"volume_up", "volume_down", "mute",
	"channel_up", "channel_down",
	"up", "down", "left", "right", "confirm",
	"home", "menu", "back", "input",
	"hdmi1", "hdmi2", "hdmi3", "hdmi4",
}

var AvailableControlActions = []string{
	"power_status", "system_info", "volume_info",
	"playing_content", "app_list", "content_list",
	"set_volume", "set_mute",
}
