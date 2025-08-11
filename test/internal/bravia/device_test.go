package bravia_test

import (
	"encoding/json"
	"lucas/internal"
	"lucas/internal/bravia"
	"lucas/internal/device"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBraviaRemote(t *testing.T) {
	t.Run("creates BraviaRemote with proper device info", func(t *testing.T) {
		options := &internal.FnModeOptions{Debug: false, Test: false}
		remote := bravia.NewBraviaRemote("192.168.1.100:80", "test-psk", options)

		assert.NotNil(t, remote)

		info := remote.GetDeviceInfo()
		assert.Equal(t, "bravia_tv", info.Type)
		assert.Equal(t, "Sony Bravia", info.Model)
		assert.Equal(t, "192.168.1.100:80", info.Address)
		assert.Contains(t, info.Capabilities, "remote_control")
		assert.Contains(t, info.Capabilities, "system_control")
		assert.Contains(t, info.Capabilities, "audio_control")
	})
}

func TestBraviaRemote_Process_RemoteActions(t *testing.T) {
	t.Run("processes remote power action successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify this is an IRCC request
			assert.Equal(t, "/sony/ircc", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		address := strings.TrimPrefix(server.URL, "http://")
		remote := bravia.NewBraviaRemote(address, "test-credential", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "remote",
			"action": "power"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.Contains(t, response.Data.(string), "power")
		assert.Empty(t, response.Error)
	})

	t.Run("processes volume up action successfully", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/sony/ircc", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		address := strings.TrimPrefix(server.URL, "http://")
		remote := bravia.NewBraviaRemote(address, "test-credential", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "remote",
			"action": "volume_up"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("handles unsupported remote action", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "remote",
			"action": "invalid_action"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "unsupported remote action")
	})

	t.Run("handles remote request failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Authentication failed"))
		}))
		defer server.Close()

		address := strings.TrimPrefix(server.URL, "http://")
		remote := bravia.NewBraviaRemote(address, "test-credential", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "remote",
			"action": "power"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "remote request failed")
	})
}

func TestBraviaRemote_Process_ControlActions(t *testing.T) {
	t.Run("processes power status control action successfully", func(t *testing.T) {
		expectedResponse := map[string]interface{}{
			"id":     float64(1),
			"result": []interface{}{map[string]interface{}{"status": "active"}},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Verify this is a system endpoint request
			assert.Equal(t, "/sony/system", r.URL.Path)
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(expectedResponse)
		}))
		defer server.Close()

		address := strings.TrimPrefix(server.URL, "http://")
		remote := bravia.NewBraviaRemote(address, "test-credential", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "control",
			"action": "power_status"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.True(t, response.Success)
		assert.NotNil(t, response.Data)
	})

	t.Run("processes set volume action with parameters", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "/sony/audio", r.URL.Path)

			// Verify the request body contains volume parameter
			var payload bravia.BraviaPayload
			json.NewDecoder(r.Body).Decode(&payload)

			assert.Equal(t, "setAudioVolume", payload.Method)
			assert.Len(t, payload.Params, 1)
			assert.Equal(t, "50", payload.Params[0]["volume"])
			assert.Equal(t, "speaker", payload.Params[0]["target"])

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"result": [{"volume": 50}]}`))
		}))
		defer server.Close()

		address := strings.TrimPrefix(server.URL, "http://")
		remote := bravia.NewBraviaRemote(address, "test-credential", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "control",
			"action": "set_volume",
			"parameters": {
				"volume": 50
			}
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.True(t, response.Success)
	})

	t.Run("handles missing required parameters for set_volume", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "control",
			"action": "set_volume"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "parameters are required")
	})

	t.Run("handles unsupported control action", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "control",
			"action": "invalid_control_action"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "unsupported control action")
	})
}

func TestBraviaRemote_Process_ErrorHandling(t *testing.T) {
	t.Run("handles invalid JSON", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		invalidJSON := `{"type": "remote", "action": }`

		response, err := remote.Process([]byte(invalidJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "failed to parse action request")
	})

	t.Run("handles missing action type", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"action": "power"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "action type is required")
	})

	t.Run("handles missing action", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "remote"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "action is required")
	})

	t.Run("handles unsupported action type", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		actionJSON := `{
			"type": "unknown_type",
			"action": "some_action"
		}`

		response, err := remote.Process([]byte(actionJSON))
		require.NoError(t, err)
		assert.False(t, response.Success)
		assert.Contains(t, response.Error, "unsupported action type")
	})
}

func TestBraviaRemote_DeviceInterface(t *testing.T) {
	t.Run("implements Device interface", func(t *testing.T) {
		remote := bravia.NewBraviaRemote("localhost:80", "test", &internal.FnModeOptions{Debug: false, Test: false})

		// Test that BraviaRemote implements Device interface
		var device device.Device = remote
		assert.NotNil(t, device)

		// Test DeviceInfo
		info := device.GetDeviceInfo()
		assert.Equal(t, "bravia_tv", info.Type)
		assert.NotEmpty(t, info.Capabilities)
	})
}
