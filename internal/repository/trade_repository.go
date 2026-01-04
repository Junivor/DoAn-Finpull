package repository

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "time"

	"FinPull/internal/domain/models"
	"FinPull/internal/domain/repository"
	pkgkafka "FinPull/pkg/kafka"
)

// ClickHouseStorage implements Storage for ClickHouse.
type ClickHouseStorage struct {
	db    *sql.DB
	table string
}

// NewClickHouseStorage creates ClickHouse storage.
func NewClickHouseStorage(db *sql.DB, table string) repository.Storage {
	return &ClickHouseStorage{db: db, table: table}
}

func (s *ClickHouseStorage) Init(ctx context.Context) error {
	return nil // Schema init in pkg
}

func (s *ClickHouseStorage) Store(ctx context.Context, t *models.Trade) error {
	// Insert into rt_ticks_raw schema
	q := fmt.Sprintf("INSERT INTO %s (ts, symbol, price, volume, source, event_id, seq, org_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)", s.table)
	// Simple idempotency placeholders: event_id and seq derived from symbol+timestamp
	eventID := fmt.Sprintf("%s-%d", t.Symbol, t.Timestamp)
	seq := uint64(t.Timestamp)
	_, err := s.db.ExecContext(ctx, q,
		time.Unix(t.Timestamp, 0),
		t.Symbol,
		t.Price,
		t.Volume,
		"finnhub",
		eventID,
		seq,
		"",
	)
	return err
}

func (s *ClickHouseStorage) StoreBatch(ctx context.Context, trades []*models.Trade) error {
    if len(trades) == 0 {
        return nil
    }
    // Batch insert using VALUES multi-row to reduce round-trips.
    // Chunk size tuned to 2000 rows per batch.
    const chunkSize = 2000
    for start := 0; start < len(trades); start += chunkSize {
        end := start + chunkSize
        if end > len(trades) { end = len(trades) }

        // Build VALUES list
        values := make([]string, 0, end-start)
        args := make([]interface{}, 0, (end-start)*8)
        for _, t := range trades[start:end] {
            if t == nil || t.Symbol == "" || t.Timestamp == 0 { continue }
            eventID := fmt.Sprintf("%s-%d", t.Symbol, t.Timestamp)
            seq := uint64(t.Timestamp)
            values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?)")
            args = append(args,
                time.Unix(t.Timestamp, 0),
                t.Symbol,
                t.Price,
                t.Volume,
                "finnhub",
                eventID,
                seq,
                "",
            )
        }
        if len(values) == 0 { continue }
        q := fmt.Sprintf("INSERT INTO %s (ts, symbol, price, volume, source, event_id, seq, org_id) VALUES %s", s.table, strings.Join(values, ","))
        if _, err := s.db.ExecContext(ctx, q, args...); err != nil {
            return err
        }
    }
    return nil
}

func (s *ClickHouseStorage) Query(ctx context.Context, symbol string, from, to time.Time, limit int) ([]*models.Trade, error) {
	q := fmt.Sprintf("SELECT symbol, ts, price, volume FROM %s WHERE symbol = ? AND ts >= ? AND ts <= ? ORDER BY ts DESC LIMIT ?", s.table)
	rows, err := s.db.QueryContext(ctx, q, symbol, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*models.Trade
	for rows.Next() {
		var t models.Trade
		var ts time.Time
		if err := rows.Scan(&t.Symbol, &ts, &t.Price, &t.Volume); err != nil {
			return nil, err
		}
		t.Timestamp = ts.Unix()
		trades = append(trades, &t)
	}
	return trades, rows.Err()
}

func (s *ClickHouseStorage) Health(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *ClickHouseStorage) Close() error {
	return nil // Managed by pkg
}

// KafkaPublisher implements Publisher for Kafka.
type KafkaPublisher struct {
    producer *pkgkafka.Producer
    topic    string
}

// NewKafkaPublisher creates Kafka publisher.
func NewKafkaPublisher(producer *pkgkafka.Producer, topic string) repository.Publisher {
	return &KafkaPublisher{producer: producer, topic: topic}
}

func (p *KafkaPublisher) Publish(ctx context.Context, t *models.Trade) error {
	return p.producer.Publish(ctx, p.topic, []byte(t.Symbol), map[string]interface{}{
		"symbol": t.Symbol,
		"t":      t.Timestamp,
		"c":      t.Price,
		"v":      t.Volume,
	})
}

func (p *KafkaPublisher) PublishBatch(ctx context.Context, trades []*models.Trade) error {
	if len(trades) == 0 {
		return nil
	}
	msgs := make([]pkgkafka.Message, len(trades))
	for i, t := range trades {
		msgs[i] = pkgkafka.Message{
			Key: []byte(t.Symbol),
			Value: map[string]interface{}{
				"symbol": t.Symbol,
				"t":      t.Timestamp,
				"c":      t.Price,
				"v":      t.Volume,
			},
		}
	}
	return p.producer.PublishBatch(ctx, p.topic, msgs)
}

func (p *KafkaPublisher) Close() error {
    if p.producer != nil {
        return p.producer.Close()
    }
    return nil
}
