package bravia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// BraviaClient represents a client for Sony Bravia TV control
type BraviaClient struct {
	httpClient *http.Client
	host       string
	credential string
	debug      bool
	logger     zerolog.Logger
}

// NewBraviaClient creates a new Bravia client instance
func NewBraviaClient(host string, credential string, debug bool) *BraviaClient {
	client := &BraviaClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		host:       host,
		credential: credential,
		debug:      debug,
		logger:     logger.New(),
	}

	if debug {
		logger.SetLevel("debug")
	}

	return client
}

// remoteRequest sends an IRCC SOAP request for remote control commands
func (c *BraviaClient) RemoteRequest(code BraviaRemoteCode) error {
	// SOAP envelope for IRCC command
	soapBody := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:X_SendIRCC xmlns:u="urn:schemas-sony-com:service:IRCC:1">
      <IRCCCode>%s</IRCCCode>
    </u:X_SendIRCC>
  </s:Body>
</s:Envelope>`, string(code))

	// Build URL
	url := fmt.Sprintf("http://%s%s", c.host, IRCCEndpoint)

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return fmt.Errorf("failed to create IRCC request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "urn:schemas-sony-com:service:IRCC:1#X_SendIRCC")
	req.Header.Set("X-Auth-PSK", c.credential)

	if c.debug {
		c.logger.Debug().
			Str("url", url).
			Str("code", string(code)).
			Msg("Sending IRCC remote request")
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send IRCC request: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		if c.debug {
			c.logger.Error().
				Int("status", resp.StatusCode).
				Str("body", string(body)).
				Msg("IRCC request failed")
		}
		return fmt.Errorf("IRCC request failed with status %d: %s", resp.StatusCode, string(body))
	}

	if c.debug {
		c.logger.Debug().
			Int("status", resp.StatusCode).
			Msg("IRCC request successful")
	}

	return nil
}

// controlRequest sends a JSON API control request
func (c *BraviaClient) ControlRequest(endpoint BraviaEndpoint, payload BraviaPayload) (*http.Response, error) {
	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("http://%s%s", c.host, string(endpoint))

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create control request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-PSK", c.credential)

	if c.debug {
		c.logger.Debug().
			Str("url", url).
			Str("endpoint", string(endpoint)).
			Str("method", payload.Method).
			Str("payload", string(jsonData)).
			Msg("Sending control API request")
	}

	// Send request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send control request: %w", err)
	}

	if c.debug {
		c.logger.Debug().
			Int("status", resp.StatusCode).
			Str("method", payload.Method).
			Msg("Control API request completed")
	}

	return resp, nil
}

// Helper method to create a basic payload with default values
func CreatePayload(id int, method BraviaMethod, params []map[string]string) BraviaPayload {
	if params == nil {
		params = []map[string]string{}
	}
	
	return BraviaPayload{
		ID:      id,
		Version: "1.0",
		Method:  string(method),
		Params:  params,
	}
}