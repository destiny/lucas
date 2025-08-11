package gateway_test

import (
	"os"
	"path/filepath"
	"testing"

	"lucas/internal/gateway"
)

func setupTestDB(t *testing.T) (*gateway.Database, func()) {
	t.Helper()

	// Create temporary database file
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := gateway.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}

	cleanup := func() {
		db.Close()
		os.Remove(dbPath)
	}

	return db, cleanup
}

func TestNewDatabase(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	db, err := gateway.NewDatabase(dbPath)
	if err != nil {
		t.Fatalf("Expected successful database creation, got error: %v", err)
	}
	defer db.Close()

	// Test database functionality by attempting basic operations
	// If tables exist and schema is correct, these operations should work
	testUser, err := db.CreateUser("test_user", "test@example.com")
	if err != nil {
		t.Errorf("Failed to create user, database may not be properly initialized: %v", err)
	}
	if testUser == nil {
		t.Error("Expected user to be created")
	}
}

func TestUserOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("CreateUser", func(t *testing.T) {
		user, err := db.CreateUser("testuser", "test@example.com")
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		if user.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", user.Username)
		}
		if user.Email != "test@example.com" {
			t.Errorf("Expected email 'test@example.com', got %s", user.Email)
		}
		if user.APIKey == "" {
			t.Error("Expected API key to be generated")
		}
		if user.ID == 0 {
			t.Error("Expected user ID to be set")
		}
	})

	t.Run("CreateUserWithPassword", func(t *testing.T) {
		user, err := db.CreateUserWithPassword("testuser2", "test2@example.com", "hashedpassword123")
		if err != nil {
			t.Fatalf("Failed to create user with password: %v", err)
		}

		if user.PasswordHash != "hashedpassword123" {
			t.Errorf("Expected password hash 'hashedpassword123', got %s", user.PasswordHash)
		}
	})

	t.Run("GetUserByAPIKey", func(t *testing.T) {
		originalUser, err := db.CreateUser("apiuser", "api@example.com")
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		foundUser, err := db.GetUserByAPIKey(originalUser.APIKey)
		if err != nil {
			t.Fatalf("Failed to get user by API key: %v", err)
		}

		if foundUser.ID != originalUser.ID {
			t.Errorf("Expected user ID %d, got %d", originalUser.ID, foundUser.ID)
		}
	})

	t.Run("GetUserByUsername", func(t *testing.T) {
		originalUser, err := db.CreateUser("usernameuser", "username@example.com")
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		foundUser, err := db.GetUserByUsername("usernameuser")
		if err != nil {
			t.Fatalf("Failed to get user by username: %v", err)
		}

		if foundUser.ID != originalUser.ID {
			t.Errorf("Expected user ID %d, got %d", originalUser.ID, foundUser.ID)
		}
	})

	t.Run("GetUserByEmail", func(t *testing.T) {
		originalUser, err := db.CreateUser("emailuser", "email@example.com")
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}

		foundUser, err := db.GetUserByEmail("email@example.com")
		if err != nil {
			t.Fatalf("Failed to get user by email: %v", err)
		}

		if foundUser.ID != originalUser.ID {
			t.Errorf("Expected user ID %d, got %d", originalUser.ID, foundUser.ID)
		}
	})

	t.Run("NonexistentUser", func(t *testing.T) {
		_, err := db.GetUserByAPIKey("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent API key")
		}

		_, err = db.GetUserByUsername("nonexistent")
		if err == nil {
			t.Error("Expected error for nonexistent username")
		}

		_, err = db.GetUserByEmail("nonexistent@example.com")
		if err == nil {
			t.Error("Expected error for nonexistent email")
		}
	})
}

func TestHubOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create a user first
	user, err := db.CreateUser("hubuser", "hub@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	t.Run("CreateHub", func(t *testing.T) {
		hub, err := db.CreateHub(user.ID, "hub123", "Test Hub", "publickey123", "http://localhost:8080")
		if err != nil {
			t.Fatalf("Failed to create hub: %v", err)
		}

		if hub.HubID != "hub123" {
			t.Errorf("Expected hub ID 'hub123', got %s", hub.HubID)
		}
		if hub.Name != "Test Hub" {
			t.Errorf("Expected hub name 'Test Hub', got %s", hub.Name)
		}
		if hub.Status != "online" {
			t.Errorf("Expected hub status 'online', got %s", hub.Status)
		}
	})

	t.Run("RegisterHub", func(t *testing.T) {
		hub, err := db.RegisterHub("reghub123", "regpubkey123", "Registered Hub", "product123")
		if err != nil {
			t.Fatalf("Failed to register hub: %v", err)
		}

		if hub.HubID != "reghub123" {
			t.Errorf("Expected hub ID 'reghub123', got %s", hub.HubID)
		}
		if hub.ProductKey != "product123" {
			t.Errorf("Expected product key 'product123', got %s", hub.ProductKey)
		}
		if !hub.AutoRegistered {
			t.Error("Expected auto_registered to be true")
		}
		if hub.UserID.Valid {
			t.Error("Expected user_id to be NULL for auto-registered hub")
		}
	})

	t.Run("RegisterHubDuplicateProductKey", func(t *testing.T) {
		// First registration should succeed
		_, err := db.RegisterHub("hub1", "pubkey1", "Hub 1", "duplicate_product")
		if err != nil {
			t.Fatalf("First registration should succeed: %v", err)
		}

		// Second registration with same product key should fail
		_, err = db.RegisterHub("hub2", "pubkey2", "Hub 2", "duplicate_product")
		if err == nil {
			t.Error("Expected error for duplicate product key")
		}
	})

	t.Run("GetHubByHubID", func(t *testing.T) {
		originalHub, err := db.CreateHub(user.ID, "gethub123", "Get Hub", "getpubkey123", "http://localhost:8080")
		if err != nil {
			t.Fatalf("Failed to create hub: %v", err)
		}

		foundHub, err := db.GetHubByHubID("gethub123")
		if err != nil {
			t.Fatalf("Failed to get hub by ID: %v", err)
		}

		if foundHub.ID != originalHub.ID {
			t.Errorf("Expected hub ID %d, got %d", originalHub.ID, foundHub.ID)
		}
	})

	t.Run("GetHubByProductKey", func(t *testing.T) {
		hub, err := db.RegisterHub("prodhub123", "prodpubkey123", "Product Hub", "prodkey123")
		if err != nil {
			t.Fatalf("Failed to register hub: %v", err)
		}

		foundHub, err := db.GetHubByProductKey("prodkey123")
		if err != nil {
			t.Fatalf("Failed to get hub by product key: %v", err)
		}

		if foundHub.ID != hub.ID {
			t.Errorf("Expected hub ID %d, got %d", hub.ID, foundHub.ID)
		}
	})

	t.Run("ClaimHub", func(t *testing.T) {
		hub, err := db.RegisterHub("claimhub123", "claimpubkey123", "Claim Hub", "claimkey123")
		if err != nil {
			t.Fatalf("Failed to register hub: %v", err)
		}

		err = db.ClaimHub(hub.HubID, user.ID)
		if err != nil {
			t.Fatalf("Failed to claim hub: %v", err)
		}

		// Verify hub is claimed
		claimedHub, err := db.GetHub(hub.ID)
		if err != nil {
			t.Fatalf("Failed to get claimed hub: %v", err)
		}

		if !claimedHub.UserID.Valid || claimedHub.UserID.Int32 != int32(user.ID) {
			t.Errorf("Expected user ID %d, got %v", user.ID, claimedHub.UserID)
		}
		if claimedHub.AutoRegistered {
			t.Error("Expected auto_registered to be false after claiming")
		}
	})

	t.Run("UpdateHubStatus", func(t *testing.T) {
		hub, err := db.CreateHub(user.ID, "statushub123", "Status Hub", "statuspubkey123", "http://localhost:8080")
		if err != nil {
			t.Fatalf("Failed to create hub: %v", err)
		}

		err = db.UpdateHubStatus(hub.HubID, "offline")
		if err != nil {
			t.Fatalf("Failed to update hub status: %v", err)
		}

		updatedHub, err := db.GetHub(hub.ID)
		if err != nil {
			t.Fatalf("Failed to get updated hub: %v", err)
		}

		if updatedHub.Status != "offline" {
			t.Errorf("Expected status 'offline', got %s", updatedHub.Status)
		}
	})

	t.Run("EnsureHubExists", func(t *testing.T) {
		hubID := "ensurhub123"

		// Hub doesn't exist initially
		_, err := db.GetHubByHubID(hubID)
		if err == nil {
			t.Error("Hub should not exist initially")
		}

		// EnsureHubExists should create it
		err = db.EnsureHubExists(hubID)
		if err != nil {
			t.Fatalf("Failed to ensure hub exists: %v", err)
		}

		// Hub should now exist
		hub, err := db.GetHubByHubID(hubID)
		if err != nil {
			t.Fatalf("Hub should exist after EnsureHubExists: %v", err)
		}

		if !hub.AutoRegistered {
			t.Error("Auto-created hub should be marked as auto_registered")
		}
	})

	t.Run("GetUserHubs", func(t *testing.T) {
		// Create multiple hubs for user
		_, err := db.CreateHub(user.ID, "userhub1", "User Hub 1", "pubkey1", "http://localhost:8081")
		if err != nil {
			t.Fatalf("Failed to create hub 1: %v", err)
		}

		_, err = db.CreateHub(user.ID, "userhub2", "User Hub 2", "pubkey2", "http://localhost:8082")
		if err != nil {
			t.Fatalf("Failed to create hub 2: %v", err)
		}

		hubs, err := db.GetUserHubs(user.ID)
		if err != nil {
			t.Fatalf("Failed to get user hubs: %v", err)
		}

		if len(hubs) < 2 {
			t.Errorf("Expected at least 2 hubs, got %d", len(hubs))
		}
	})
}

func TestDeviceOperations(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	// Create user and hub first
	user, err := db.CreateUser("deviceuser", "device@example.com")
	if err != nil {
		t.Fatalf("Failed to create user: %v", err)
	}

	hub, err := db.CreateHub(user.ID, "devicehub123", "Device Hub", "devicepubkey123", "http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to create hub: %v", err)
	}

	t.Run("CreateDevice", func(t *testing.T) {
		capabilities := []string{"power", "volume", "channel"}
		device, err := db.CreateDevice(hub.ID, "device123", "tv", "Test TV", "Sony Bravia", "192.168.1.100", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}

		if device.DeviceID != "device123" {
			t.Errorf("Expected device ID 'device123', got %s", device.DeviceID)
		}
		if device.DeviceType != "tv" {
			t.Errorf("Expected device type 'tv', got %s", device.DeviceType)
		}
		if len(device.Capabilities) != 3 {
			t.Errorf("Expected 3 capabilities, got %d", len(device.Capabilities))
		}
	})

	t.Run("GetHubDevices", func(t *testing.T) {
		capabilities := []string{"power"}

		// Create multiple devices for hub
		_, err := db.CreateDevice(hub.ID, "hubdev1", "tv", "Hub Device 1", "Model 1", "192.168.1.101", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device 1: %v", err)
		}

		_, err = db.CreateDevice(hub.ID, "hubdev2", "speaker", "Hub Device 2", "Model 2", "192.168.1.102", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device 2: %v", err)
		}

		devices, err := db.GetHubDevices(hub.ID)
		if err != nil {
			t.Fatalf("Failed to get hub devices: %v", err)
		}

		if len(devices) < 2 {
			t.Errorf("Expected at least 2 devices, got %d", len(devices))
		}
	})

	t.Run("GetUserDevices", func(t *testing.T) {
		devices, err := db.GetUserDevices(user.ID)
		if err != nil {
			t.Fatalf("Failed to get user devices: %v", err)
		}

		// Should have devices from previous tests
		if len(devices) == 0 {
			t.Error("Expected at least some devices for user")
		}
	})

	t.Run("FindDeviceByID", func(t *testing.T) {
		capabilities := []string{"power"}
		originalDevice, err := db.CreateDevice(hub.ID, "finddev123", "tv", "Find Device", "Model X", "192.168.1.200", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}

		foundDevice, foundHub, err := db.FindDeviceByID("finddev123")
		if err != nil {
			t.Fatalf("Failed to find device: %v", err)
		}

		if foundDevice.ID != originalDevice.ID {
			t.Errorf("Expected device ID %d, got %d", originalDevice.ID, foundDevice.ID)
		}
		if foundHub.ID != hub.ID {
			t.Errorf("Expected hub ID %d, got %d", hub.ID, foundHub.ID)
		}
	})

	t.Run("UpdateDeviceStatus", func(t *testing.T) {
		capabilities := []string{"power"}
		device, err := db.CreateDevice(hub.ID, "statusdev123", "tv", "Status Device", "Model Y", "192.168.1.201", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}

		err = db.UpdateDeviceStatus("statusdev123", "offline")
		if err != nil {
			t.Fatalf("Failed to update device status: %v", err)
		}

		updatedDevice, err := db.GetDevice(device.ID)
		if err != nil {
			t.Fatalf("Failed to get updated device: %v", err)
		}

		if updatedDevice.Status != "offline" {
			t.Errorf("Expected status 'offline', got %s", updatedDevice.Status)
		}
	})

	t.Run("DeleteDevice", func(t *testing.T) {
		capabilities := []string{"power"}
		device, err := db.CreateDevice(hub.ID, "deletedev123", "tv", "Delete Device", "Model Z", "192.168.1.202", capabilities)
		if err != nil {
			t.Fatalf("Failed to create device: %v", err)
		}

		err = db.DeleteDevice(device.ID)
		if err != nil {
			t.Fatalf("Failed to delete device: %v", err)
		}

		// Verify device is deleted
		_, err = db.GetDevice(device.ID)
		if err == nil {
			t.Error("Expected error when getting deleted device")
		}
	})
}

func TestDatabaseMigration(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("CreateUserWithPassword", func(t *testing.T) {
		// Test that password functionality works (indicating password_hash column exists)
		testUser, err := db.CreateUserWithPassword("test_pw_user", "testpw@example.com", "hashed_password")
		if err != nil {
			t.Fatalf("Failed to create user with password: %v", err)
		}
		if testUser == nil {
			t.Error("Expected user to be created")
		}
	})
}

func TestDatabaseConcurrency(t *testing.T) {
	db, cleanup := setupTestDB(t)
	defer cleanup()

	t.Run("ConcurrentEnsureHubExists", func(t *testing.T) {
		hubID := "concurrent_hub"

		// Run multiple goroutines trying to ensure the same hub exists
		done := make(chan error, 5)
		successCount := 0

		for i := 0; i < 5; i++ {
			go func() {
				err := db.EnsureHubExists(hubID)
				done <- err
			}()
		}

		// Wait for all goroutines to complete
		// Some may fail due to SQLite locking, but at least one should succeed
		var lastErr error
		for i := 0; i < 5; i++ {
			err := <-done
			if err == nil {
				successCount++
			} else {
				lastErr = err
				// Log the error but don't fail the test immediately
				t.Logf("EnsureHubExists attempt %d failed (expected with SQLite): %v", i+1, err)
			}
		}

		// At least one operation should have succeeded
		if successCount == 0 {
			t.Fatalf("All EnsureHubExists operations failed, last error: %v", lastErr)
		}

		// Verify hub exists exactly once
		hub, err := db.GetHubByHubID(hubID)
		if err != nil {
			t.Fatalf("Hub should exist after concurrent operations: %v", err)
		}

		if hub.HubID != hubID {
			t.Errorf("Expected hub ID %s, got %s", hubID, hub.HubID)
		}

		if !hub.AutoRegistered {
			t.Error("Concurrently created hub should be marked as auto_registered")
		}

		t.Logf("Concurrent test completed: %d/%d operations succeeded", successCount, 5)
	})
}
