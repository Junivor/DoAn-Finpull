package server

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"FinPull/internal/handler/api"
	"FinPull/internal/repository"
	icache "FinPull/internal/service/cache"
	analytics "FinPull/internal/services/analytics"
	"FinPull/internal/usecase"
	pkgch "FinPull/pkg/clickhouse"
	"FinPull/pkg/config"
	xhttp "FinPull/pkg/http"
	pkgkafka "FinPull/pkg/kafka"
	applogger "FinPull/pkg/logger"
)

// App encapsulates the entire application lifecycle.
type App struct {
	cfg         *config.Config
	collector   *usecase.TradeCollector
	consumer    *pkgkafka.Consumer
	kh          pkgkafka.MessageHandler
	chClient    *pkgch.Client
	httpServer  *xhttp.Server
	httpHandler xhttp.Handler
	TradeProc   *usecase.TradeProcessor
}

// New creates a new App instance with all dependencies.
func New(
	cfg *config.Config,
	collector *usecase.TradeCollector,
	consumer *pkgkafka.Consumer,
	kh pkgkafka.MessageHandler,
	chClient *pkgch.Client,
) *App {
	return &App{
		cfg:       cfg,
		collector: collector,
		consumer:  consumer,
		kh:        kh,
		chClient:  chClient,
	}
}

// SetHTTPHandler allows DI to inject an HTTP handler.
func (a *App) SetHTTPHandler(h xhttp.Handler) { a.httpHandler = h }

// Run starts the application and blocks until interrupted.
func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// init app logger (console info by default)
	l, _ := applogger.New(&applogger.Config{Level: "info", Format: "console", Output: "stdout"})

	// Setup Echo HTTP server using pkg/http and register routes via handler
	var httpHandler xhttp.Handler
	if a.httpHandler != nil {
		httpHandler = a.httpHandler
	} else if a.chClient != nil && a.cfg.Analytics.PythonServiceURL != "" {
		store := repository.NewCHFeatureStore(a.chClient)
		store.SetLogger(l)
		regime := analytics.NewHTTPRegimeDetector(a.cfg)
		vol := analytics.NewHTTPVolatilityForecaster(a.cfg)
		anom := analytics.NewDomainAnomalyAdapter(a.cfg)
		edge := analytics.NewHTTPEdgeScorer(a.cfg)
		agg := usecase.NewSignalAggregator(store, regime, vol, anom, edge)

		// Optional cache wiring (Echo handler can internally cache if desired)
		_ = icache.NewTTLCache

		se := api.NewSignalsEchoHandler(l, agg)
		httpHandler = se
	}

	a.httpServer = xhttp.NewServer(httpHandler,
		xhttp.WithPort(a.cfg.Server.Port),
		xhttp.WithTimeouts(a.cfg.Server.ReadTimeout, a.cfg.Server.WriteTimeout, a.cfg.Server.ShutdownTimeout),
	)

	// Start collector
	go func() {
		if err := a.collector.Start(ctx); err != nil {
			l.Error("collector error", applogger.Error(err))
		}
	}()
	l.Info("collector started", applogger.Strings("symbols", a.cfg.Finnhub.Symbols))

	// Start consumer if configured
	if a.consumer != nil && a.kh != nil {
		a.consumer.RegisterHandler(a.kh)
		go func() {
			if err := a.consumer.Start(); err != nil {
				l.Error("kafka consumer error", applogger.Error(err))
			}
		}()
		l.Info("kafka consumer started", applogger.String("topic", a.kh.Topic()))
	}

	// Start HTTP server
	if err := a.httpServer.Start(); err != nil {
		l.Error("http server start error", applogger.Error(err))
		return err
	}

	// Wait for interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	l.Info("shutdown signal received")
	return a.shutdown(ctx)
}

// shutdown gracefully stops all services.
func (a *App) shutdown(ctx context.Context) error {
	// best-effort logging via stdout
	l, err := applogger.New(&applogger.Config{Level: "info", Format: "console", Output: "stdout"})
	if err != nil {
		log.Printf("failed to create logger: %v", err)
		return err
	}
	l.Info("shutting down...", applogger.Error(err))

	// Stop collector (pipeline + stream)
	if err := a.collector.Shutdown(ctx); err != nil {
		l.Warn("collector stop error", applogger.Error(err))
	}

	// Shutdown HTTP server
	shutdownCtx, cancel := context.WithTimeout(ctx, a.cfg.Server.ShutdownTimeout)
	defer cancel()
	if err := a.httpServer.Stop(shutdownCtx); err != nil {
		l.Error("http shutdown error", applogger.Error(err))
	}

	// Close infrastructure clients
	if a.chClient != nil {
		if err := a.chClient.Close(); err != nil {
			l.Warn("clickhouse close error", applogger.Error(err))
		}
	}

	// Close Kafka producer via publisher if available
	// Note: publisher Close() is managed where it's stored; here we only close consumer.

	// Stop consumer
	if a.consumer != nil {
		if err := a.consumer.Stop(ctx); err != nil {
			l.Warn("kafka consumer stop error", applogger.Error(err))
		}
	}

	// Close trade processor resources (publisher/storage)
	if a.TradeProc != nil {
		a.TradeProc.Close()
	}

	l.Info("shutdown complete")
	return nil
}

// healthHandler checks all infrastructure dependencies.
// Health and readiness endpoints should be registered via Echo when needed.
