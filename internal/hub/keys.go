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

package hub

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/destiny/zmq4/v25/security/curve"
	"gopkg.in/yaml.v3"
)

// HubKeyPair represents a CurveZMQ key pair for the hub
type HubKeyPair struct {
	PublicKey  string `json:"public_key" yaml:"public_key"`
	PrivateKey string `json:"private_key" yaml:"private_key"`
}

// HubKeys holds hub cryptographic keys and gateway public key
type HubKeys struct {
	Hub     HubKeyPair `json:"hub" yaml:"hub"`
	Gateway string     `json:"gateway_public_key" yaml:"gateway_public_key"`
}

// GenerateHubKeyPair generates a new CurveZMQ key pair for the hub
func GenerateHubKeyPair() (*HubKeyPair, error) {
	// Use the proper CurveZMQ key generation from the library
	keyPair, err := curve.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CurveZMQ keypair: %w", err)
	}
	
	// Convert to Z85 format
	publicKey, err := keyPair.PublicKeyZ85()
	if err != nil {
		return nil, fmt.Errorf("failed to encode public key: %w", err)
	}
	
	privateKey, err := keyPair.SecretKeyZ85()
	if err != nil {
		return nil, fmt.Errorf("failed to encode private key: %w", err)
	}

	return &HubKeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// GenerateHubID generates a unique hub ID using UUID
func GenerateHubID() (string, error) {
	// Generate a new UUID v4
	hubUUID, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("failed to generate UUID for hub ID: %w", err)
	}

	// Return UUID string with "hub_" prefix for clarity
	return "hub_" + hubUUID.String(), nil
}

// SaveHubKeys saves hub keys to a file (YAML or JSON based on extension)
func SaveHubKeys(keys *HubKeys, keyFile string) error {
	var data []byte
	var err error

	// Determine format based on file extension
	if isYAMLExtension(keyFile) {
		data, err = yaml.Marshal(keys)
		if err != nil {
			return fmt.Errorf("failed to marshal keys as YAML: %w", err)
		}
	} else {
		data, err = json.MarshalIndent(keys, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal keys as JSON: %w", err)
		}
	}

	// Write with restricted permissions (600 - owner read/write only)
	if err := os.WriteFile(keyFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// LoadHubKeys loads hub keys from a file (auto-detects YAML or JSON)
func LoadHubKeys(keyFile string) (*HubKeys, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keys HubKeys

	// Determine format based on file extension or content
	if isYAMLFormat(keyFile, data) {
		if err := yaml.Unmarshal(data, &keys); err != nil {
			return nil, fmt.Errorf("failed to parse YAML key file: %w", err)
		}
	} else {
		if err := json.Unmarshal(data, &keys); err != nil {
			return nil, fmt.Errorf("failed to parse JSON key file: %w", err)
		}
	}

	// Validate key format
	if err := keys.Validate(); err != nil {
		return nil, fmt.Errorf("invalid keys in file: %w", err)
	}

	return &keys, nil
}

// LoadOrGenerateHubKeys loads existing keys or generates new ones
func LoadOrGenerateHubKeys(keyFile string) (*HubKeys, error) {
	// Try to load existing keys
	if _, err := os.Stat(keyFile); err == nil {
		keys, err := LoadHubKeys(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing keys: %w", err)
		}
		return keys, nil
	}

	// Generate new keys
	hubKeyPair, err := GenerateHubKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate hub keypair: %w", err)
	}

	keys := &HubKeys{
		Hub:     *hubKeyPair,
		Gateway: "", // Will be filled in during gateway discovery/registration
	}

	// Save keys to file
	if err := SaveHubKeys(keys, keyFile); err != nil {
		return nil, fmt.Errorf("failed to save keys: %w", err)
	}

	return keys, nil
}

// Validate checks if the hub keys are valid
func (hk *HubKeys) Validate() error {
	if err := hk.Hub.Validate(); err != nil {
		return fmt.Errorf("invalid hub keys: %w", err)
	}

	// Gateway key is optional (may not be set yet)
	if hk.Gateway != "" {
		if err := ValidateCurveKey(hk.Gateway); err != nil {
			return fmt.Errorf("invalid gateway public key: %w", err)
		}
	}

	return nil
}

// Validate checks if a hub key pair is valid
func (hkp *HubKeyPair) Validate() error {
	if hkp.PublicKey == "" {
		return fmt.Errorf("hub public key is empty")
	}
	if hkp.PrivateKey == "" {
		return fmt.Errorf("hub private key is empty")
	}

	// Check key length (CurveZMQ keys are 40 characters when Z85 encoded)
	if len(hkp.PublicKey) != 40 {
		return fmt.Errorf("invalid hub public key length: expected 40, got %d", len(hkp.PublicKey))
	}
	if len(hkp.PrivateKey) != 40 {
		return fmt.Errorf("invalid hub private key length: expected 40, got %d", len(hkp.PrivateKey))
	}

	return nil
}

// ValidateCurveKey validates a CurveZMQ key format
func ValidateCurveKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is empty")
	}
	if len(key) != 40 {
		return fmt.Errorf("invalid key length: expected 40, got %d", len(key))
	}

	// Try to decode as Z85 to validate format using the curve package validation
	if err := curve.ValidateZ85Key(key); err != nil {
		return fmt.Errorf("invalid CurveZMQ key: %w", err)
	}

	return nil
}

// GetHubPublicKey returns the hub's public key
func (hk *HubKeys) GetHubPublicKey() string {
	return hk.Hub.PublicKey
}

// GetHubPrivateKey returns the hub's private key
func (hk *HubKeys) GetHubPrivateKey() string {
	return hk.Hub.PrivateKey
}

// GetGatewayPublicKey returns the gateway's public key
func (hk *HubKeys) GetGatewayPublicKey() string {
	return hk.Gateway
}

// SetGatewayPublicKey sets the gateway's public key
func (hk *HubKeys) SetGatewayPublicKey(gatewayKey string) error {
	if err := ValidateCurveKey(gatewayKey); err != nil {
		return fmt.Errorf("invalid gateway public key: %w", err)
	}
	hk.Gateway = gatewayKey
	return nil
}

// HasGatewayKey returns true if gateway public key is set
func (hk *HubKeys) HasGatewayKey() bool {
	return hk.Gateway != ""
}

// RegenerateHubKeys generates new hub keys (use with caution)
func (hk *HubKeys) RegenerateHubKeys() error {
	newKeyPair, err := GenerateHubKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate new hub keypair: %w", err)
	}

	hk.Hub = *newKeyPair
	return nil
}

// CreateDefaultHubKeys creates a default hub keys structure
func CreateDefaultHubKeys() (*HubKeys, error) {
	hubKeyPair, err := GenerateHubKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate hub keypair: %w", err)
	}

	return &HubKeys{
		Hub:     *hubKeyPair,
		Gateway: "", // Empty until gateway registration
	}, nil
}

// HubKeyInfo provides information about hub keys without revealing private keys
type HubKeyInfo struct {
	HubPublicKey     string `json:"hub_public_key"`
	GatewayPublicKey string `json:"gateway_public_key,omitempty"`
	KeyType          string `json:"key_type"`
	HasGatewayKey    bool   `json:"has_gateway_key"`
}

// GetKeyInfo returns public information about the hub keys
func (hk *HubKeys) GetKeyInfo() HubKeyInfo {
	return HubKeyInfo{
		HubPublicKey:     hk.Hub.PublicKey,
		GatewayPublicKey: hk.Gateway,
		KeyType:          "curve25519",
		HasGatewayKey:    hk.Gateway != "",
	}
}

// HubSecurityInfo provides security-related information about the keys
type HubSecurityInfo struct {
	KeyStrength  string `json:"key_strength"`
	Algorithm    string `json:"algorithm"`
	Curve        string `json:"curve"`
	KeyLength    int    `json:"key_length"`
	CreationTime string `json:"creation_time,omitempty"`
	LastUsed     string `json:"last_used,omitempty"`
}

// GetSecurityInfo returns security information about the hub keys
func (hk *HubKeys) GetSecurityInfo() HubSecurityInfo {
	return HubSecurityInfo{
		KeyStrength: "256-bit",
		Algorithm:   "CurveZMQ",
		Curve:       "Curve25519",
		KeyLength:   40, // Z85 encoded length
	}
}

// isYAMLExtension checks if the file extension indicates YAML format
func isYAMLExtension(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return ext == ".yml" || ext == ".yaml"
}

// isYAMLFormat determines if the file should be parsed as YAML
func isYAMLFormat(filename string, content []byte) bool {
	// First check the file extension
	if isYAMLExtension(filename) {
		return true
	}

	// For files without clear extensions, try to detect based on content
	// YAML typically starts with keys without quotes, JSON starts with {
	contentStr := strings.TrimSpace(string(content))
	if strings.HasPrefix(contentStr, "{") {
		return false // Likely JSON
	}

	// If it contains lines with key: value pattern, likely YAML
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			if strings.Contains(line, ":") && !strings.HasPrefix(line, "\"") {
				return true // Likely YAML
			}
			break
		}
	}

	return false // Default to JSON
}
