package middleware

import (
	"log"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogging logs HTTP requests.
func RequestLogging() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()
			start := time.Now()

			err := next(c)

			latency := time.Since(start)
			log.Printf("[%s] %s %s - %d (%s)",
				req.Method,
				req.RequestURI,
				req.RemoteAddr,
				res.Status,
				latency,
			)

			return err
		}
	}
}
