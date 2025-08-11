package bravia

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"lucas/internal/logger"
)

// BraviaClient represents a client for Sony Bravia TV control
type BraviaClient struct {
	httpClient *http.Client
	address    string
	credential string
	debug      bool
	testMode   bool
	logger     zerolog.Logger
}

// NewBraviaClient creates a new Bravia client instance
func NewBraviaClient(address string, credential string, debug bool) *BraviaClient {
	return NewBraviaClientWithFlags(address, credential, debug, false)
}

// NewBraviaClientWithFlags creates a new Bravia client instance with test mode support
func NewBraviaClientWithFlags(address string, credential string, debug bool, testMode bool) *BraviaClient {
	client := &BraviaClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		address:    address,
		credential: credential,
		debug:      debug,
		testMode:   testMode,
		logger:     logger.New(),
	}

	if debug {
		logger.SetLevel("debug")
	}

	return client
}

// remoteRequest sends an IRCC SOAP request for remote control commands
func (c *BraviaClient) RemoteRequest(code BraviaRemoteCode) error {
	// Test mode: simulate successful request without HTTP call
	if c.testMode {
		c.logger.Info().
			Str("code", string(code)).
			Str("address", c.address).
			Msg("Test mode: Remote request simulated")
		return nil
	}

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
	url := fmt.Sprintf("http://%s%s", c.address, IRCCEndpoint)

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBufferString(soapBody))
	if err != nil {
		return fmt.Errorf("failed to create IRCC request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", "\"urn:schemas-sony-com:service:IRCC:1#X_SendIRCC\"")
	req.Header.Set("X-Auth-PSK", c.credential)

	// Log complete HTTP request if debug is enabled
	c.logHTTPRequest(req)

	// Send request and measure duration
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return fmt.Errorf("failed to send IRCC request: %w", err)
	}
	defer resp.Body.Close()

	// Log complete HTTP response if debug is enabled
	c.logHTTPResponse(resp, duration)

	// Check response status
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("IRCC request failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// controlRequest sends a JSON API control request
func (c *BraviaClient) ControlRequest(endpoint BraviaEndpoint, payload BraviaPayload) (*http.Response, error) {
	// Test mode: simulate successful request without HTTP call
	if c.testMode {
		c.logger.Info().
			Str("endpoint", string(endpoint)).
			Str("method", payload.Method).
			Str("address", c.address).
			Msg("Test mode: Control request simulated")

		// Create a mock response for test mode
		mockResp := &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"result": [{"status": "success"}]}`)),
		}
		mockResp.Header.Set("Content-Type", "application/json")
		return mockResp, nil
	}

	// Marshal payload to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Build URL
	url := fmt.Sprintf("http://%s%s", c.address, string(endpoint))

	// Create request
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create control request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Auth-PSK", c.credential)

	// Log complete HTTP request if debug is enabled
	c.logHTTPRequest(req)

	// Send request and measure duration
	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("failed to send control request: %w", err)
	}

	// Log complete HTTP response if debug is enabled
	c.logHTTPResponse(resp, duration)

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

// logHTTPRequest logs the complete HTTP request details when debug is enabled
func (c *BraviaClient) logHTTPRequest(req *http.Request) {
	if !c.debug {
		return
	}

	// Use httputil.DumpRequest to get complete request details
	reqDump, err := httputil.DumpRequest(req, true)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to dump HTTP request")
		return
	}

	// Mask sensitive credential in the dump
	reqDumpStr := string(reqDump)
	if c.credential != "" {
		reqDumpStr = strings.ReplaceAll(reqDumpStr, c.credential, "****")
	}

	c.logger.Debug().
		Str("http_request", reqDumpStr).
		Msg("Bravia HTTP Request")
}

// logHTTPResponse logs the complete HTTP response details when debug is enabled
func (c *BraviaClient) logHTTPResponse(resp *http.Response, duration time.Duration) {
	if !c.debug {
		return
	}

	// Use httputil.DumpResponse to get complete response details
	respDump, err := httputil.DumpResponse(resp, true)
	if err != nil {
		c.logger.Error().Err(err).Msg("Failed to dump HTTP response")
		return
	}

	c.logger.Debug().
		Str("http_response", string(respDump)).
		Dur("duration", duration).
		Msg("Bravia HTTP Response")
}
