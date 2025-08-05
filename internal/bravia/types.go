package bravia

// BraviaRemoteCode represents a remote control code for Sony Bravia TVs
type BraviaRemoteCode string

// BraviaEndpoint represents an API endpoint for Sony Bravia control
type BraviaEndpoint string

// BraviaMethod represents an API method for Sony Bravia control
type BraviaMethod string

// BraviaPayload represents the JSON payload structure for control API requests
type BraviaPayload struct {
	ID      int                 `json:"id"`
	Version string              `json:"version"`
	Method  string              `json:"method"`
	Params  []map[string]string `json:"params"`
}