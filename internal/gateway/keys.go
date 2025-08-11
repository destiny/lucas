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

package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pebbe/zmq4"
	"gopkg.in/yaml.v3"
)

// KeyPair represents a CurveZMQ key pair
type KeyPair struct {
	PublicKey  string `json:"public_key" yaml:"public_key"`
	PrivateKey string `json:"private_key" yaml:"private_key"`
}

// GatewayKeys holds all gateway cryptographic keys
type GatewayKeys struct {
	Server KeyPair `json:"server" yaml:"server"`
}

// GenerateKeyPair generates a new CurveZMQ key pair
func GenerateKeyPair() (*KeyPair, error) {
	publicKey, privateKey, err := zmq4.NewCurveKeypair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CurveZMQ keypair: %w", err)
	}

	return &KeyPair{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	}, nil
}

// GenerateRandomKey generates a random key for API tokens, etc.
func GenerateRandomKey(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate random key: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// LoadOrGenerateGatewayKeys loads gateway keys from file or generates new ones
func LoadOrGenerateGatewayKeys(keyFile string) (*GatewayKeys, error) {
	// Try to load existing keys
	if _, err := os.Stat(keyFile); err == nil {
		keys, err := LoadGatewayKeys(keyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing keys: %w", err)
		}
		return keys, nil
	}

	// Generate new keys
	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server keypair: %w", err)
	}

	keys := &GatewayKeys{
		Server: *serverKeyPair,
	}

	// Save keys to file using format based on file extension
	if err := SaveGatewayKeys(keys, keyFile); err != nil {
		return nil, fmt.Errorf("failed to save keys: %w", err)
	}

	return keys, nil
}

// LoadGatewayKeys loads gateway keys from a JSON or YAML file (auto-detects format)
func LoadGatewayKeys(keyFile string) (*GatewayKeys, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read key file: %w", err)
	}

	var keys GatewayKeys

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

// SaveGatewayKeys saves gateway keys to a JSON or YAML file (format determined by extension)
func SaveGatewayKeys(keys *GatewayKeys, keyFile string) error {
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
	err = os.WriteFile(keyFile, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}

	return nil
}

// Validate checks if the gateway keys are valid
func (gk *GatewayKeys) Validate() error {
	if err := gk.Server.Validate(); err != nil {
		return fmt.Errorf("invalid server keys: %w", err)
	}
	return nil
}

// Validate checks if a key pair is valid
func (kp *KeyPair) Validate() error {
	if kp.PublicKey == "" {
		return fmt.Errorf("public key is empty")
	}
	if kp.PrivateKey == "" {
		return fmt.Errorf("private key is empty")
	}

	// Check key length (CurveZMQ keys are 40 characters when Z85 encoded)
	if len(kp.PublicKey) != 40 {
		return fmt.Errorf("invalid public key length: expected 40, got %d", len(kp.PublicKey))
	}
	if len(kp.PrivateKey) != 40 {
		return fmt.Errorf("invalid private key length: expected 40, got %d", len(kp.PrivateKey))
	}

	return nil
}

// GetServerPublicKey returns the server's public key
func (gk *GatewayKeys) GetServerPublicKey() string {
	return gk.Server.PublicKey
}

// GetServerPrivateKey returns the server's private key
func (gk *GatewayKeys) GetServerPrivateKey() string {
	return gk.Server.PrivateKey
}

// GenerateHubKeypair generates a keypair for a new hub
func GenerateHubKeypair() (*KeyPair, error) {
	return GenerateKeyPair()
}

// ValidateCurveKey validates a CurveZMQ key format
func ValidateCurveKey(key string) error {
	if key == "" {
		return fmt.Errorf("key is empty")
	}
	if len(key) != 40 {
		return fmt.Errorf("invalid key length: expected 40, got %d", len(key))
	}

	// Try to decode as Z85 to validate format
	decoded := zmq4.Z85decode(key)
	if len(decoded) != 32 { // Z85 decoded should be 32 bytes for CurveZMQ key
		return fmt.Errorf("invalid Z85 encoding or decoded key length")
	}

	return nil
}

// CreateDefaultGatewayKeys creates a default gateway keys structure
func CreateDefaultGatewayKeys() (*GatewayKeys, error) {
	serverKeyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server keypair: %w", err)
	}

	return &GatewayKeys{
		Server: *serverKeyPair,
	}, nil
}

// KeyInfo provides information about a key without revealing the private key
type KeyInfo struct {
	PublicKey string `json:"public_key"`
	KeyType   string `json:"key_type"`
	Generated bool   `json:"generated"`
}

// GetKeyInfo returns public information about the gateway keys
func (gk *GatewayKeys) GetKeyInfo() KeyInfo {
	return KeyInfo{
		PublicKey: gk.Server.PublicKey,
		KeyType:   "curve25519",
		Generated: true,
	}
}

// RegenerateServerKeys generates new server keys (use with caution)
func (gk *GatewayKeys) RegenerateServerKeys() error {
	newKeyPair, err := GenerateKeyPair()
	if err != nil {
		return fmt.Errorf("failed to generate new keypair: %w", err)
	}

	gk.Server = *newKeyPair
	return nil
}

// ExportPublicKey exports just the public key for sharing with hubs
func (gk *GatewayKeys) ExportPublicKey() string {
	return gk.Server.PublicKey
}

// SecurityInfo provides security-related information about the keys
type SecurityInfo struct {
	KeyStrength  string `json:"key_strength"`
	Algorithm    string `json:"algorithm"`
	Curve        string `json:"curve"`
	KeyLength    int    `json:"key_length"`
	CreationTime string `json:"creation_time,omitempty"`
	LastUsed     string `json:"last_used,omitempty"`
}

// GetSecurityInfo returns security information about the keys
func (gk *GatewayKeys) GetSecurityInfo() SecurityInfo {
	return SecurityInfo{
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
