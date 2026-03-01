package middleware

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// CORS returns a Gin middleware configured for Cross-Origin Resource Sharing.
//
// In "debug" mode every origin is allowed so the front-end dev server can talk
// to the API without friction.  In "release" mode only the explicitly listed
// origins are permitted.
func CORS(mode string) gin.HandlerFunc {
	var allowOrigins []string

	if mode == "release" {
		// TODO: replace with actual production domains.
		allowOrigins = []string{
			"https://nongqucun.example.com",
		}
	} else {
		// Allow all origins in development / test.
		allowOrigins = []string{"*"}
	}

	cfg := cors.Config{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Request-ID"},
		ExposeHeaders:    []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: mode == "release", // credentials + wildcard origin is invalid
		MaxAge:           12 * time.Hour,
	}

	// cors.New panics if AllowCredentials is true AND AllowOrigins contains "*".
	// The condition above already prevents that combination, but add an explicit
	// guard to be safe during future refactors.
	if cfg.AllowCredentials {
		for _, o := range cfg.AllowOrigins {
			if o == "*" {
				cfg.AllowCredentials = false
				break
			}
		}
	}

	return cors.New(cfg)
}
