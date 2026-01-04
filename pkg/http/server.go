package http

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"FinPull/pkg/http/middleware"

	"github.com/labstack/echo/v4"
    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServerOption configures Server.
type ServerOption func(*ServerConfig)

// ServerConfig holds server configuration.
type ServerConfig struct {
	Host            string
	Port            int
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
	CORS            bool
}

// Server wraps Echo HTTP server.
type Server struct {
	echo   *echo.Echo
	config *ServerConfig
}

// NewServer creates a new HTTP server with Echo.
func NewServer(handler Handler, opts ...ServerOption) *Server {
	cfg := &ServerConfig{
		Host:            "0.0.0.0",
		Port:            8080,
		ReadTimeout:     10 * time.Second,
		WriteTimeout:    10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
		CORS:            true,
	}

	for _, opt := range opts {
		opt(cfg)
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// Middleware
	e.Use(middleware.Recover())
	e.Use(middleware.RequestLogging())

	if cfg.CORS {
		e.Use(middleware.CORS(middleware.CORSConfig{
			AllowOrigins: []string{"*"},
			AllowMethods: []string{
				http.MethodGet,
				http.MethodPost,
				http.MethodPut,
				http.MethodPatch,
				http.MethodDelete,
				http.MethodOptions,
			},
			AllowHeaders: []string{
				echo.HeaderOrigin,
				echo.HeaderContentType,
				echo.HeaderAccept,
				echo.HeaderAuthorization,
			},
		}))
	}

	// Register routes
	if handler != nil {
		handler.RegisterRoutes(e)
	}

    // Expose Prometheus metrics endpoint for scraping
    e.GET("/metrics", echo.WrapHandler(promhttp.Handler()))

	return &Server{
		echo:   e,
		config: cfg,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.config.Host, s.config.Port)

	go func() {
		log.Printf("http server: listening on %s", addr)
		if err := s.echo.Start(addr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("http server error: %v", err)
		}
	}()

	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if err := s.echo.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown error: %w", err)
	}
	log.Println("http server: stopped gracefully")
	return nil
}

// Echo returns the underlying Echo instance.
func (s *Server) Echo() *echo.Echo {
	return s.echo
}

// WithHost sets server host.
func WithHost(host string) ServerOption {
	return func(c *ServerConfig) {
		c.Host = host
	}
}

// WithPort sets server port.
func WithPort(port int) ServerOption {
	return func(c *ServerConfig) {
		c.Port = port
	}
}

// WithTimeouts sets read/write timeouts.
func WithTimeouts(read, write, shutdown time.Duration) ServerOption {
	return func(c *ServerConfig) {
		c.ReadTimeout = read
		c.WriteTimeout = write
		c.ShutdownTimeout = shutdown
	}
}

// WithCORS enables/disables CORS.
func WithCORS(enabled bool) ServerOption {
	return func(c *ServerConfig) {
		c.CORS = enabled
	}
}
