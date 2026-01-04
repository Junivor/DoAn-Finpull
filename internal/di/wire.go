//go:build wireinject
// +build wireinject

package di

import (
	"FinPull/pkg/config"
	"FinPull/pkg/server"

	"github.com/google/wire"
)

// InitializeApp wires up all dependencies and returns the application.
// Wire will generate the implementation of this function.
func InitializeApp(cfg *config.Config) (*server.App, error) {
    wire.Build(
        // Metrics
        ProvideMetrics,

		// Infrastructure clients
		ProvideClickHouseClient,
		ProvideKafkaProducer,

		// Repositories (with business logic)
		ProvideTradeStorage,
		ProvideTradePublisher,
		ProvideFinnhubStream,

        // Use cases
        ProvideTradeProcessor,
        ProvideTradeCollector,

        // Application server
        ProvideApp,
    )
    return &server.App{}, nil
}
