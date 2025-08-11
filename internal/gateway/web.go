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
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// Generate web assets before embedding with clean build flow
//go:generate rm -rf ./web-dist
//go:generate npm --prefix=../../web install
//go:generate npm --prefix=../../web run build
//go:generate cp -r ../../web/dist ./web-dist

// Embed the built web application
//go:embed all:web-dist
var webAssets embed.FS

// SetupWebApp configures web app serving
func (api *APIServer) SetupWebApp(router *mux.Router) {
	// Extract the dist subdirectory
	webFS, err := fs.Sub(webAssets, "web-dist")
	if err != nil {
		api.logger.Fatal().Err(err).Msg("Failed to setup web filesystem")
	}

	fileServer := http.FileServer(http.FS(webFS))

	// Serve web app (everything except /api routes)
	router.PathPrefix("/").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// API routes handled elsewhere
		if strings.HasPrefix(r.URL.Path, "/api/") {
			http.NotFound(w, r)
			return
		}

		// SPA fallback - serve index.html for app routes
		if r.URL.Path != "/" && !strings.Contains(r.URL.Path, ".") {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}