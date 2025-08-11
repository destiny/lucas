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