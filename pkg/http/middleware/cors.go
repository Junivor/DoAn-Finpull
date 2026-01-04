package middleware

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
)

// CORSConfig holds CORS configuration.
type CORSConfig struct {
	AllowOrigins []string
	AllowMethods []string
	AllowHeaders []string
}

// CORS returns CORS middleware.
func CORS(cfg CORSConfig) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			origin := c.Request().Header.Get("Origin")

			// Check if origin is allowed
			if len(cfg.AllowOrigins) > 0 {
				allowed := false
				for _, o := range cfg.AllowOrigins {
					if o == "*" || o == origin {
						allowed = true
						break
					}
				}
				if !allowed {
					return next(c)
				}
			}

			// Set CORS headers
			if origin != "" {
				c.Response().Header().Set("Access-Control-Allow-Origin", origin)
			} else if len(cfg.AllowOrigins) > 0 && cfg.AllowOrigins[0] == "*" {
				c.Response().Header().Set("Access-Control-Allow-Origin", "*")
			}

			if len(cfg.AllowMethods) > 0 {
				c.Response().Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowMethods, ", "))
			}

			if len(cfg.AllowHeaders) > 0 {
				c.Response().Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowHeaders, ", "))
			}

			// Handle preflight
			if c.Request().Method == http.MethodOptions {
				return c.NoContent(http.StatusNoContent)
			}

			return next(c)
		}
	}
}
