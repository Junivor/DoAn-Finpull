package middleware

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"

	"github.com/labstack/echo/v4"
)

// Recover returns recovery middleware.
func Recover() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			defer func() {
				if r := recover(); r != nil {
					err, ok := r.(error)
					if !ok {
						err = fmt.Errorf("%v", r)
					}
					log.Printf("PANIC: %v\n%s", err, debug.Stack())
					_ = c.JSON(http.StatusInternalServerError, map[string]interface{}{
						"status":  http.StatusInternalServerError,
						"message": "Internal Server Error",
					})
				}
			}()
			return next(c)
		}
	}
}
