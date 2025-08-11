package hub

import (
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"lucas/internal/device"
)

// NonceResponse represents a cached response for a specific nonce
type NonceResponse struct {
	Nonce     string                  `json:"nonce"`
	Response  *device.ActionResponse  `json:"response"`
	Timestamp time.Time               `json:"timestamp"`
}

// NonceCache manages nonce-based deduplication per device
type NonceCache struct {
	deviceCaches map[string]*lru.Cache[string, *NonceResponse]
	mutex        sync.RWMutex
	maxSize      int
	expiration   time.Duration
}

// NewNonceCache creates a new nonce cache
func NewNonceCache(maxSize int, expiration time.Duration) *NonceCache {
	if maxSize <= 0 {
		maxSize = 50 // Default to 50 nonces per device
	}
	if expiration <= 0 {
		expiration = time.Hour // Default to 1 hour expiration
	}

	nc := &NonceCache{
		deviceCaches: make(map[string]*lru.Cache[string, *NonceResponse]),
		maxSize:      maxSize,
		expiration:   expiration,
	}

	// Start cleanup routine for expired entries
	go nc.cleanupExpired()

	return nc
}

// GenerateNonce generates a unique nonce with timestamp and random component
func GenerateNonce() string {
	// Get current timestamp in milliseconds
	timestamp := time.Now().UnixMilli()
	
	// Generate 4 random bytes
	randomBytes := make([]byte, 4)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to pseudo-random if crypto/rand fails
		timestamp += int64(time.Now().Nanosecond())
		randomBytes = []byte{
			byte(timestamp >> 24),
			byte(timestamp >> 16),
			byte(timestamp >> 8),
			byte(timestamp),
		}
	}
	
	// Format: timestamp_ms-random_hex
	return fmt.Sprintf("%d-%x", timestamp, randomBytes)
}

// getDeviceCache gets or creates a cache for a specific device
func (nc *NonceCache) getDeviceCache(deviceID string) *lru.Cache[string, *NonceResponse] {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()

	cache, exists := nc.deviceCaches[deviceID]
	if !exists {
		// Create new LRU cache for this device
		cache, _ = lru.New[string, *NonceResponse](nc.maxSize)
		nc.deviceCaches[deviceID] = cache
	}

	return cache
}

// CheckNonce checks if a nonce has been seen for a device and returns cached response if found
func (nc *NonceCache) CheckNonce(deviceID, nonce string) (*device.ActionResponse, bool) {
	if nonce == "" {
		return nil, false // No nonce provided, treat as new request
	}

	cache := nc.getDeviceCache(deviceID)
	
	if cachedResponse, found := cache.Get(nonce); found {
		// Check if the cached response has expired
		if time.Since(cachedResponse.Timestamp) > nc.expiration {
			cache.Remove(nonce)
			return nil, false
		}
		
		return cachedResponse.Response, true
	}

	return nil, false
}

// StoreResponse stores a response for a specific nonce and device
func (nc *NonceCache) StoreResponse(deviceID, nonce string, response *device.ActionResponse) {
	if nonce == "" {
		return // No nonce provided, nothing to cache
	}

	cache := nc.getDeviceCache(deviceID)
	
	nonceResponse := &NonceResponse{
		Nonce:     nonce,
		Response:  response,
		Timestamp: time.Now(),
	}
	
	cache.Add(nonce, nonceResponse)
}

// RemoveNonce removes a specific nonce from a device's cache
func (nc *NonceCache) RemoveNonce(deviceID, nonce string) {
	nc.mutex.RLock()
	cache, exists := nc.deviceCaches[deviceID]
	nc.mutex.RUnlock()

	if exists {
		cache.Remove(nonce)
	}
}

// ClearDevice clears all nonces for a specific device
func (nc *NonceCache) ClearDevice(deviceID string) {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()

	if cache, exists := nc.deviceCaches[deviceID]; exists {
		cache.Purge()
	}
}

// GetDeviceNonceCount returns the number of cached nonces for a device
func (nc *NonceCache) GetDeviceNonceCount(deviceID string) int {
	nc.mutex.RLock()
	cache, exists := nc.deviceCaches[deviceID]
	nc.mutex.RUnlock()

	if !exists {
		return 0
	}

	return cache.Len()
}

// GetStats returns cache statistics
func (nc *NonceCache) GetStats() map[string]interface{} {
	nc.mutex.RLock()
	defer nc.mutex.RUnlock()

	totalDevices := len(nc.deviceCaches)
	totalNonces := 0
	deviceStats := make(map[string]int)

	for deviceID, cache := range nc.deviceCaches {
		count := cache.Len()
		totalNonces += count
		deviceStats[deviceID] = count
	}

	return map[string]interface{}{
		"total_devices":   totalDevices,
		"total_nonces":    totalNonces,
		"max_size":        nc.maxSize,
		"expiration":      nc.expiration.String(),
		"device_stats":    deviceStats,
	}
}

// cleanupExpired runs a periodic cleanup of expired nonce responses
func (nc *NonceCache) cleanupExpired() {
	ticker := time.NewTicker(10 * time.Minute) // Cleanup every 10 minutes
	defer ticker.Stop()

	for range ticker.C {
		nc.performCleanup()
	}
}

// performCleanup removes expired entries from all device caches
func (nc *NonceCache) performCleanup() {
	nc.mutex.RLock()
	deviceCaches := make(map[string]*lru.Cache[string, *NonceResponse])
	for deviceID, cache := range nc.deviceCaches {
		deviceCaches[deviceID] = cache
	}
	nc.mutex.RUnlock()

	now := time.Now()
	expiredCount := 0

	for deviceID, cache := range deviceCaches {
		// Get all keys and check expiration
		keys := cache.Keys()
		for _, nonce := range keys {
			if value, found := cache.Peek(nonce); found {
				if now.Sub(value.Timestamp) > nc.expiration {
					cache.Remove(nonce)
					expiredCount++
				}
			}
		}

		// Remove empty device caches
		if cache.Len() == 0 {
			nc.mutex.Lock()
			delete(nc.deviceCaches, deviceID)
			nc.mutex.Unlock()
		}
	}

	if expiredCount > 0 {
		// Note: Could add logging here if needed
		// log.Debug().Int("expired_count", expiredCount).Msg("Cleaned up expired nonces")
	}
}

// Shutdown gracefully shuts down the nonce cache
func (nc *NonceCache) Shutdown() {
	nc.mutex.Lock()
	defer nc.mutex.Unlock()

	// Clear all caches
	for _, cache := range nc.deviceCaches {
		cache.Purge()
	}
	nc.deviceCaches = make(map[string]*lru.Cache[string, *NonceResponse])
}

// ValidateNonce validates the format of a simple nonce (timestamp-hex)
func ValidateNonce(nonce string) bool {
	if nonce == "" {
		return false
	}
	
	// Basic format validation: should be at least 13 characters (timestamp-hex)
	// Example: 1691234567890-a1b2c3d4 (13 + 1 + 8 = 22 chars minimum)
	if len(nonce) < 13 {
		return false
	}
	
	// Must contain exactly one dash separator
	dashCount := strings.Count(nonce, "-")
	if dashCount != 1 {
		return false
	}
	
	// Find dash position
	dashIndex := strings.Index(nonce, "-")
	if dashIndex <= 0 || dashIndex >= len(nonce)-1 {
		return false
	}
	
	// Timestamp part should be numeric
	timestampPart := nonce[:dashIndex]
	if len(timestampPart) < 13 { // Unix timestamp in milliseconds is 13+ digits
		return false
	}
	for _, c := range timestampPart {
		if c < '0' || c > '9' {
			return false
		}
	}
	
	// Random part should be 8-character hex
	randomPart := nonce[dashIndex+1:]
	if len(randomPart) != 8 {
		return false
	}
	for _, c := range randomPart {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	
	return true
}