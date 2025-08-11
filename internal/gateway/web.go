package gateway

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// Generate web assets before embedding with error handling and cross-platform support
//go:generate bash -c "cd ../../web && npm install"
//go:generate bash -c "cd ../../web && npm run build"
//go:generate bash -c "rm -rf ./web-dist && cp -r ../../web/dist ./web-dist"

// Embed the built web application
//
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
