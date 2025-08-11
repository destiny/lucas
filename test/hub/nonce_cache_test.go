package hub_test

import (
	"testing"
	"time"

	"lucas/internal/device"
	"lucas/internal/hub"
)

func TestNewNonceCache(t *testing.T) {
	t.Run("creates cache with invalid parameters", func(t *testing.T) {
		cache := hub.NewNonceCache(0, 0)
		defer cache.Shutdown()

		// Test that cache was created successfully by verifying it can store and retrieve responses
		deviceID := "test_device"
		nonce := hub.GenerateNonce()
		response := &device.ActionResponse{Success: true, Data: "test"}

		cache.StoreResponse(deviceID, nonce, response)
		retrieved, found := cache.CheckNonce(deviceID, nonce)
		if !found {
			t.Error("Expected to find stored nonce")
		}
		if retrieved.Success != response.Success {
			t.Error("Retrieved response doesn't match stored response")
		}
	})

	t.Run("creates cache with custom values", func(t *testing.T) {
		maxSize := 100
		expiration := 30 * time.Minute
		cache := hub.NewNonceCache(maxSize, expiration)
		defer cache.Shutdown()

		// Test that cache was created successfully
		deviceID := "test_device"
		nonce := hub.GenerateNonce()
		response := &device.ActionResponse{Success: true, Data: "test"}

		cache.StoreResponse(deviceID, nonce, response)
		retrieved, found := cache.CheckNonce(deviceID, nonce)
		if !found {
			t.Error("Expected to find stored nonce")
		}
		if retrieved.Success != response.Success {
			t.Error("Retrieved response doesn't match stored response")
		}
	})
}

func TestGenerateNonce(t *testing.T) {
	t.Run("generates unique nonces", func(t *testing.T) {
		nonce1 := hub.GenerateNonce()
		nonce2 := hub.GenerateNonce()

		if nonce1 == nonce2 {
			t.Error("Expected unique nonces, got identical ones")
		}
	})

	t.Run("validates generated nonce format", func(t *testing.T) {
		nonce := hub.GenerateNonce()
		if !hub.ValidateNonce(nonce) {
			t.Errorf("Generated nonce %s failed validation", nonce)
		}
	})
}

func TestValidateNonce(t *testing.T) {
	tests := []struct {
		name     string
		nonce    string
		expected bool
	}{
		{"empty nonce", "", false},
		{"too short", "123", false},
		{"valid nonce", "1691234567890-a1b2c3d4", true},
		{"no dash", "1691234567890a1b2c3d4", false},
		{"multiple dashes", "1691234567890-a1b2-c3d4", false},
		{"dash at start", "-1691234567890a1b2c3d4", false},
		{"dash at end", "1691234567890a1b2c3d4-", false},
		{"non-numeric timestamp", "abc1234567890-a1b2c3d4", false},
		{"short timestamp", "123456789-a1b2c3d4", false},
		{"invalid hex", "1691234567890-xyz2c3d4", false},
		{"short hex", "1691234567890-a1b2c3", false},
		{"long hex", "1691234567890-a1b2c3d4e", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := hub.ValidateNonce(tt.nonce)
			if result != tt.expected {
				t.Errorf("hub.ValidateNonce(%s) = %v, expected %v", tt.nonce, result, tt.expected)
			}
		})
	}
}

func TestNonceCacheBasicOperations(t *testing.T) {
	cache := hub.NewNonceCache(10, time.Hour)
	defer cache.Shutdown()

	deviceID := "device123"
	nonce := "1691234567890-a1b2c3d4"
	response := &device.ActionResponse{
		Success: true,
		Data:    "Test response",
	}

	t.Run("check nonexistent nonce", func(t *testing.T) {
		resp, found := cache.CheckNonce(deviceID, nonce)
		if found {
			t.Error("Expected nonce not found, but it was")
		}
		if resp != nil {
			t.Error("Expected nil response for nonexistent nonce")
		}
	})

	t.Run("store and retrieve nonce", func(t *testing.T) {
		cache.StoreResponse(deviceID, nonce, response)

		resp, found := cache.CheckNonce(deviceID, nonce)
		if !found {
			t.Error("Expected to find stored nonce")
		}
		if resp == nil {
			t.Fatal("Expected response, got nil")
		}
		if resp.Success != response.Success {
			t.Errorf("Expected Success %v, got %v", response.Success, resp.Success)
		}
		if resp.Data != response.Data {
			t.Errorf("Expected Data %v, got %v", response.Data, resp.Data)
		}
	})

	t.Run("empty nonce handling", func(t *testing.T) {
		cache.StoreResponse(deviceID, "", response)
		resp, found := cache.CheckNonce(deviceID, "")
		if found {
			t.Error("Expected empty nonce to not be found")
		}
		if resp != nil {
			t.Error("Expected nil response for empty nonce")
		}
	})
}

func TestNonceCacheExpiration(t *testing.T) {
	shortExpiration := 50 * time.Millisecond
	cache := hub.NewNonceCache(10, shortExpiration)
	defer cache.Shutdown()

	deviceID := "device123"
	nonce := "1691234567890-a1b2c3d4"
	response := &device.ActionResponse{
		Success: true,
		Data:    "Test response",
	}

	cache.StoreResponse(deviceID, nonce, response)

	// Should find it immediately
	resp, found := cache.CheckNonce(deviceID, nonce)
	if !found || resp == nil {
		t.Error("Expected to find fresh nonce")
	}

	// Wait for expiration
	time.Sleep(shortExpiration + 10*time.Millisecond)

	// Should not find it after expiration
	resp, found = cache.CheckNonce(deviceID, nonce)
	if found {
		t.Error("Expected expired nonce to not be found")
	}
	if resp != nil {
		t.Error("Expected nil response for expired nonce")
	}
}

func TestNonceCacheDeviceOperations(t *testing.T) {
	cache := hub.NewNonceCache(10, time.Hour)
	defer cache.Shutdown()

	device1 := "device1"
	device2 := "device2"
	nonce1 := "1691234567890-a1b2c3d4"
	nonce2 := "1691234567890-e5f6g7h8"
	response := &device.ActionResponse{Success: true}

	cache.StoreResponse(device1, nonce1, response)
	cache.StoreResponse(device1, nonce2, response)
	cache.StoreResponse(device2, nonce1, response)

	t.Run("device nonce count", func(t *testing.T) {
		count1 := cache.GetDeviceNonceCount(device1)
		count2 := cache.GetDeviceNonceCount(device2)
		countNone := cache.GetDeviceNonceCount("nonexistent")

		if count1 != 2 {
			t.Errorf("Expected device1 count 2, got %d", count1)
		}
		if count2 != 1 {
			t.Errorf("Expected device2 count 1, got %d", count2)
		}
		if countNone != 0 {
			t.Errorf("Expected nonexistent device count 0, got %d", countNone)
		}
	})

	t.Run("clear device", func(t *testing.T) {
		cache.ClearDevice(device1)

		count1 := cache.GetDeviceNonceCount(device1)
		count2 := cache.GetDeviceNonceCount(device2)

		if count1 != 0 {
			t.Errorf("Expected cleared device1 count 0, got %d", count1)
		}
		if count2 != 1 {
			t.Errorf("Expected device2 count unchanged at 1, got %d", count2)
		}
	})

	t.Run("remove specific nonce", func(t *testing.T) {
		cache.RemoveNonce(device2, nonce1)

		count2 := cache.GetDeviceNonceCount(device2)
		if count2 != 0 {
			t.Errorf("Expected device2 count 0 after removal, got %d", count2)
		}
	})
}

func TestNonceCacheStats(t *testing.T) {
	cache := hub.NewNonceCache(10, time.Hour)
	defer cache.Shutdown()

	device1 := "device1"
	device2 := "device2"
	response := &device.ActionResponse{Success: true}

	cache.StoreResponse(device1, "nonce1", response)
	cache.StoreResponse(device1, "nonce2", response)
	cache.StoreResponse(device2, "nonce3", response)

	stats := cache.GetStats()

	if stats["total_devices"] != 2 {
		t.Errorf("Expected 2 total devices, got %v", stats["total_devices"])
	}
	if stats["total_nonces"] != 3 {
		t.Errorf("Expected 3 total nonces, got %v", stats["total_nonces"])
	}
	if stats["max_size"] != 10 {
		t.Errorf("Expected max_size 10, got %v", stats["max_size"])
	}

	deviceStats := stats["device_stats"].(map[string]int)
	if deviceStats[device1] != 2 {
		t.Errorf("Expected device1 stats 2, got %v", deviceStats[device1])
	}
	if deviceStats[device2] != 1 {
		t.Errorf("Expected device2 stats 1, got %v", deviceStats[device2])
	}
}

func TestNonceCacheShutdown(t *testing.T) {
	cache := hub.NewNonceCache(10, time.Hour)

	deviceID := "device123"
	nonce := "1691234567890-a1b2c3d4"
	response := &device.ActionResponse{Success: true}

	cache.StoreResponse(deviceID, nonce, response)

	// Verify it's stored
	if cache.GetDeviceNonceCount(deviceID) != 1 {
		t.Error("Expected nonce to be stored before shutdown")
	}

	cache.Shutdown()

	// Verify it's cleared
	if cache.GetDeviceNonceCount(deviceID) != 0 {
		t.Error("Expected cache to be cleared after shutdown")
	}

	stats := cache.GetStats()
	if stats["total_devices"] != 0 {
		t.Error("Expected no devices after shutdown")
	}
}

func TestNonceCachePerformCleanup(t *testing.T) {
	shortExpiration := 50 * time.Millisecond
	cache := hub.NewNonceCache(10, shortExpiration)
	defer cache.Shutdown()

	deviceID := "device123"
	response := &device.ActionResponse{Success: true}

	// Add some nonces
	cache.StoreResponse(deviceID, "nonce1", response)
	cache.StoreResponse(deviceID, "nonce2", response)

	if cache.GetDeviceNonceCount(deviceID) != 2 {
		t.Error("Expected 2 nonces before cleanup")
	}

	// Wait for expiration and automatic cleanup
	time.Sleep(shortExpiration + 100*time.Millisecond) // Give more time for automatic cleanup

	// Test that cleanup happened by checking if expired nonces are gone
	// Since nonces are expired, CheckNonce should return false
	_, found1 := cache.CheckNonce(deviceID, "nonce1")
	_, found2 := cache.CheckNonce(deviceID, "nonce2")
	if found1 || found2 {
		t.Error("Expected expired nonces to be cleaned up")
	}
}
