package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"FinPull/internal/domain/models"
	domrepo "FinPull/internal/domain/repository"
	pkgch "FinPull/pkg/clickhouse"
	applogger "FinPull/pkg/logger"
)

// CHFeatureStore implements FeatureStore backed by ClickHouse.
type CHFeatureStore struct {
	db *sql.DB
	l  *applogger.Logger
}

func NewCHFeatureStore(ch *pkgch.Client) *CHFeatureStore {
	return &CHFeatureStore{db: ch.DB()}
}

// SetLogger injects a structured logger.
func (s *CHFeatureStore) SetLogger(l *applogger.Logger) { s.l = l }

func (s *CHFeatureStore) GetCandles(ctx context.Context, symbol string, from, to time.Time, tf domrepo.Timeframe) ([]models.Candle, error) {
	start := time.Now()
	table, err := tableForTF(tf)
	if err != nil {
		return nil, err
	}
	const qtpl = `
        SELECT bucket, symbol, open, high, low, close, vol, '' AS org_id
        FROM %s
        WHERE symbol = ? AND bucket >= ? AND bucket <= ?
        ORDER BY bucket ASC
    `
	q := fmt.Sprintf(qtpl, table)
	rows, err := s.db.QueryContext(ctx, q, symbol, from, to)
	if err != nil {
		if s.l != nil {
			s.l.Error("clickhouse get_candles query error",
				applogger.String("table", table),
				applogger.String("symbol", symbol),
				applogger.String("tf", string(tf)),
				applogger.Error(err),
			)
		}
		return nil, fmt.Errorf("get candles: %w", err)
	}
	defer rows.Close()

	out := make([]models.Candle, 0, 1024)
	for rows.Next() {
		var c models.Candle
		if err := rows.Scan(&c.Bucket, &c.Symbol, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.OrgID); err != nil {
			if s.l != nil {
				s.l.Error("clickhouse get_candles scan error",
					applogger.String("table", table),
					applogger.String("symbol", symbol),
					applogger.String("tf", string(tf)),
					applogger.Error(err),
				)
			}
			return nil, fmt.Errorf("scan candle: %w", err)
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		if s.l != nil {
			s.l.Error("clickhouse get_candles rows error",
				applogger.String("table", table),
				applogger.String("symbol", symbol),
				applogger.String("tf", string(tf)),
				applogger.Error(err),
			)
		}
		return nil, fmt.Errorf("rows: %w", err)
	}
	if s.l != nil {
		s.l.Info("clickhouse get_candles ok",
			applogger.String("table", table),
			applogger.String("symbol", symbol),
			applogger.String("tf", string(tf)),
			applogger.Int("rows", len(out)),
			applogger.Duration("duration_ms", time.Since(start)),
		)
	}
	return out, nil
}

func (s *CHFeatureStore) GetLatestNCandles(ctx context.Context, symbol string, n int, tf domrepo.Timeframe) ([]models.Candle, error) {
	start := time.Now()
	table, err := tableForTF(tf)
	if err != nil {
		return nil, err
	}
	const qtpl = `
        SELECT bucket, symbol, open, high, low, close, vol, '' AS org_id
        FROM %s
        WHERE symbol = ?
        ORDER BY bucket DESC
        LIMIT ?
    `
	q := fmt.Sprintf(qtpl, table)
	rows, err := s.db.QueryContext(ctx, q, symbol, n)
	if err != nil {
		if s.l != nil {
			s.l.Error("clickhouse latest_candles query error",
				applogger.String("table", table),
				applogger.String("symbol", symbol),
				applogger.String("tf", string(tf)),
				applogger.Int("limit", n),
				applogger.Error(err),
			)
		}
		return nil, fmt.Errorf("get latest candles: %w", err)
	}
	defer rows.Close()

	tmp := make([]models.Candle, 0, n)
	for rows.Next() {
		var c models.Candle
		if err := rows.Scan(&c.Bucket, &c.Symbol, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume, &c.OrgID); err != nil {
			if s.l != nil {
				s.l.Error("clickhouse latest_candles scan error",
					applogger.String("table", table),
					applogger.String("symbol", symbol),
					applogger.String("tf", string(tf)),
					applogger.Int("limit", n),
					applogger.Error(err),
				)
			}
			return nil, fmt.Errorf("scan candle: %w", err)
		}
		tmp = append(tmp, c)
	}
	if err := rows.Err(); err != nil {
		if s.l != nil {
			s.l.Error("clickhouse latest_candles rows error",
				applogger.String("table", table),
				applogger.String("symbol", symbol),
				applogger.String("tf", string(tf)),
				applogger.Int("limit", n),
				applogger.Error(err),
			)
		}
		return nil, fmt.Errorf("rows: %w", err)
	}
	// reverse to ASC
	for i, j := 0, len(tmp)-1; i < j; i, j = i+1, j-1 {
		tmp[i], tmp[j] = tmp[j], tmp[i]
	}
	if s.l != nil {
		s.l.Info("clickhouse latest_candles ok",
			applogger.String("table", table),
			applogger.String("symbol", symbol),
			applogger.String("tf", string(tf)),
			applogger.Int("limit", n),
			applogger.Int("rows", len(tmp)),
			applogger.Duration("duration_ms", time.Since(start)),
		)
	}
	return tmp, nil
}

func tableForTF(tf domrepo.Timeframe) (string, error) {
	switch tf {
	case domrepo.TF1s:
		return "finpull.rt_candles_1s", nil
	case domrepo.TF1m:
		return "finpull.rt_candles_1m", nil
	case domrepo.TF5m:
		// fold to 1m for now; 5m can be aggregated in-memory if needed
		return "finpull.rt_candles_1m", nil
	default:
		return "", fmt.Errorf("unsupported timeframe: %s", tf)
	}
}
