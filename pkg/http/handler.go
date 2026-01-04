package http

import "github.com/labstack/echo/v4"

// Handler defines HTTP route registration interface.
type Handler interface {
	RegisterRoutes(e *echo.Echo)
}
