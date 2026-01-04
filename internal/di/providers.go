package di

import (
    "context"
    "fmt"
    "time"

    "FinPull/internal/domain/repository"
    mid "FinPull/internal/middleware"
    internalrepo "FinPull/internal/repository"
    "FinPull/internal/service/finnhub"
    "FinPull/internal/usecase"
    pkgch "FinPull/pkg/clickhouse"
    "FinPull/pkg/config"
    pkgkafka "FinPull/pkg/kafka"
    "FinPull/pkg/metrics"
    "FinPull/pkg/server"
)

// ProvideClickHouseClient creates a ClickHouse client.
func ProvideClickHouseClient(cfg *config.Config) (*pkgch.Client, error) {
	client, err := pkgch.NewClient(
		pkgch.WithHost(cfg.ClickHouse.Host),
		pkgch.WithPort(cfg.ClickHouse.Port),
		pkgch.WithDatabase(cfg.ClickHouse.Database),
		pkgch.WithCredentials(cfg.ClickHouse.User, cfg.ClickHouse.Password),
		pkgch.WithMaxConnections(10, 5),
		pkgch.WithHTTP(cfg.ClickHouse.UseHTTP),
		pkgch.WithAsyncInsert(cfg.ClickHouse.AsyncInsert, cfg.ClickHouse.WaitForAsync),
		pkgch.WithTimeouts(cfg.ClickHouse.DialTimeout, cfg.ClickHouse.ReadTimeout, cfg.ClickHouse.WriteTimeout),
		pkgch.WithMaxExecutionTime(cfg.ClickHouse.MaxExecutionTime),
	)
	if err != nil {
		return nil, fmt.Errorf("clickhouse client: %w", err)
	}

	// Initialize schema
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := client.InitSchema(ctx, []string{
		"CREATE DATABASE IF NOT EXISTS finpull",
		"CREATE TABLE IF NOT EXISTS finpull.binance_btc (symbol String, t DateTime, c Float64, v Float64) ENGINE=MergeTree ORDER BY (symbol, t)",
	}); err != nil {
		_ = client.Close() // cannot log here (DI layer no logger); propagate error
		return nil, fmt.Errorf("clickhouse schema: %w", err)
	}

	return client, nil
}

// ProvideKafkaProducer creates a Kafka producer.
func ProvideKafkaProducer(cfg *config.Config) (*pkgkafka.Producer, error) {
	producer, err := pkgkafka.NewProducer(
		pkgkafka.WithBrokers(cfg.Kafka.Brokers),
		pkgkafka.WithCompression(cfg.Kafka.Compression),
		pkgkafka.WithRequiredAcks(cfg.Kafka.RequiredAcks),
		pkgkafka.WithBatchSize(cfg.Kafka.Producer.BatchSize),
		pkgkafka.WithBatchBytes(cfg.Kafka.Producer.BatchBytes),
		pkgkafka.WithBatchTimeout(cfg.Kafka.Producer.Linger),
		pkgkafka.WithTimeouts(cfg.Kafka.Producer.WriteTimeout, cfg.Kafka.Producer.ReadTimeout),
		pkgkafka.WithMaxAttempts(cfg.Kafka.Producer.MaxAttempts),
		pkgkafka.WithAsync(cfg.Kafka.Producer.Async),
		pkgkafka.WithHashByKey(true),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka producer: %w", err)
	}

	return producer, nil
}

// ProvideMetrics creates a Prometheus metrics recorder.
func ProvideMetrics() repository.Metrics {
	return metrics.New()
}

// ProvideTradeStorage creates ClickHouse storage repository.
func ProvideTradeStorage(chClient *pkgch.Client, cfg *config.Config) repository.Storage {
	return internalrepo.NewClickHouseStorage(chClient.DB(), cfg.ClickHouse.Database+".rt_ticks_raw")
}

// ProvideTradePublisher creates Kafka publisher repository.
func ProvideTradePublisher(producer *pkgkafka.Producer, cfg *config.Config) repository.Publisher {
	return internalrepo.NewKafkaPublisher(producer, cfg.Kafka.Topic)
}

// ProvideKafkaConsumer creates a Kafka consumer configured from YAML.
func ProvideKafkaConsumer(cfg *config.Config) (*pkgkafka.Consumer, error) {
	consumer, err := pkgkafka.NewConsumer(
		pkgkafka.WithConsumerBrokers(cfg.Kafka.Brokers),
		pkgkafka.WithConsumerGroupID(cfg.Kafka.Consumer.GroupID),
		pkgkafka.WithConsumerWorkers(cfg.Kafka.Consumer.Workers),
		pkgkafka.WithConsumerBufferSize(cfg.Kafka.Consumer.BufferSize),
		pkgkafka.WithConsumerRetry(cfg.Kafka.Consumer.RetryMax, cfg.Kafka.Consumer.BackoffMin, cfg.Kafka.Consumer.BackoffMax),
		pkgkafka.WithConsumerDLQ(cfg.Kafka.Consumer.DLQTopic),
		pkgkafka.WithConsumerFetch(cfg.Kafka.Consumer.MinBytes, cfg.Kafka.Consumer.MaxBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("kafka consumer: %w", err)
	}
	return consumer, nil
}

// ProvideKafkaTicksHandler registers handler for ticks topic.
func ProvideKafkaTicksHandler(store repository.Storage, metrics repository.Metrics, cfg *config.Config) *usecase.KafkaTicksHandler {
	return usecase.NewKafkaTicksHandler(cfg.Kafka.Topic, store, metrics)
}

// ProvideFinnhubStream creates Finnhub WebSocket stream.
func ProvideFinnhubStream(cfg *config.Config) repository.MarketStream {
	return finnhub.New(
		cfg.Finnhub.APIKey,
		cfg.Finnhub.WebSocketURL,
		cfg.Finnhub.Symbols,
		cfg.Finnhub.ReconnectDelay,
		cfg.Finnhub.PingInterval,
	)
}

// ProvideTradeProcessor creates trade processor use case.
func ProvideTradeProcessor(
	pub repository.Publisher,
	store repository.Storage,
	metrics repository.Metrics,
	cfg *config.Config,
) *usecase.TradeProcessor {
	return usecase.NewTradeProcessor(
		pub,
		store,
		metrics,
		cfg.Backend.Type,
		cfg.Backend.BatchSize,
		cfg.Backend.BatchTimeout,
	)
}

// ProvideTradeCollector creates trade collector use case.
func ProvideTradeCollector(
    stream repository.MarketStream,
    processor *usecase.TradeProcessor,
    metrics repository.Metrics,
) *usecase.TradeCollector {
    // Build middleware pipeline between WebSocket and Kafka
    pipe := mid.NewRealtimePipeline(processor, metrics,
        mid.WithMaxRPS(50),
        mid.WithBufferSize(2000),
    )
    return usecase.NewTradeCollector(stream, processor, metrics, pipe)
}

// ProvideApp creates the application server.
func ProvideApp(
    cfg *config.Config,
    collector *usecase.TradeCollector,
    consumer *pkgkafka.Consumer,
    kh *usecase.KafkaTicksHandler,
    chClient *pkgch.Client,
) *server.App {
    // Attach hook to consumer: example NoopHook for now; can be replaced via config
    if consumer != nil {
        consumer.WithConsumerHook(pkgkafka.NoopHook{})
    }
    app := server.New(cfg, collector, consumer, kh, chClient)
    // attach trade processor to app for closing resources via collector
    if collector != nil {
        app.TradeProc = collector.Processor()
    }
    return app
}
