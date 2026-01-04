-- ============================================================================
-- Alert Monitor Dashboard - SQL Queries for Superset
-- ============================================================================
-- Note: Full alert monitoring requires alert history table
-- These queries show potential triggers based on price/volume patterns

-- Query 1: Potential Alert Triggers (Rule-based)
-- Example: Symbols with 5%+ price range in last hour
WITH now() AS t
SELECT 
    symbol,
    count(*) AS trigger_count,
    max(bucket) AS last_trigger,
    max(high) - min(low) AS price_range,
    100 * (max(high) - min(low)) / min(low) AS pct_range
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
HAVING pct_range > 5.0  -- 5% threshold
ORDER BY trigger_count DESC, pct_range DESC
LIMIT 50;

-- Query 2: Volume Spike Detections
WITH volume_avg AS (
    SELECT 
        symbol,
        bucket,
        vol,
        avg(vol) OVER (
            PARTITION BY symbol 
            ORDER BY bucket 
            ROWS BETWEEN 59 PRECEDING AND CURRENT ROW
        ) AS avg_vol_1h
    FROM finpull.rt_candles_1m
    WHERE bucket >= now() - INTERVAL 24 HOUR
)
SELECT 
    symbol,
    bucket AS timestamp,
    vol AS volume,
    avg_vol_1h,
    vol / avg_vol_1h AS volume_multiplier,
    'volume_spike' AS alert_type
FROM volume_avg
WHERE vol > avg_vol_1h * 3  -- 3x average volume
ORDER BY bucket DESC, volume_multiplier DESC
LIMIT 100;

-- Query 3: Price Shock Detections (Rapid Price Movements)
WITH price_changes AS (
    SELECT 
        symbol,
        bucket AS timestamp,
        close,
        lag(close) OVER (PARTITION BY symbol ORDER BY bucket) AS prev_close,
        abs(close - lag(close) OVER (PARTITION BY symbol ORDER BY bucket)) / 
            lag(close) OVER (PARTITION BY symbol ORDER BY bucket) * 100 AS pct_change
    FROM finpull.rt_candles_1m
    WHERE bucket >= now() - INTERVAL 24 HOUR
)
SELECT 
    symbol,
    timestamp,
    close AS price,
    prev_close,
    pct_change,
    'price_shock' AS alert_type
FROM price_changes
WHERE abs(pct_change) > 2.0  -- 2% change in 1 minute
ORDER BY abs(pct_change) DESC, timestamp DESC
LIMIT 100;

-- Query 4: Symbols Monitored Over Time
SELECT 
    toStartOfHour(bucket) AS hour,
    count(DISTINCT symbol) AS symbols_monitored,
    sum(vol) AS total_volume,
    count(*) AS total_candles
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 24 HOUR
GROUP BY hour
ORDER BY hour DESC;

-- Query 5: Alert Precision Metrics (if alert history exists)
-- Placeholder - requires alert_history table
SELECT 
    toStartOfHour(bucket) AS hour,
    count(*) AS alerts_triggered,
    count(DISTINCT symbol) AS symbols_alerted
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 24 HOUR
    AND (high - low) / low > 0.05  -- Example threshold
GROUP BY hour
ORDER BY hour DESC;

-- Query 6: Latency Strip Chart Data
-- Shows volatility vs time for top symbols
SELECT 
    bucket AS timestamp,
    symbol,
    (high - low) / low AS volatility,
    vol AS volume
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 1 HOUR
    AND symbol IN (
        SELECT symbol 
        FROM finpull.rt_candles_1m
        WHERE bucket >= now() - INTERVAL 1 HOUR
        GROUP BY symbol
        ORDER BY sum(vol) DESC
        LIMIT 10
    )
ORDER BY bucket DESC, volatility DESC
LIMIT 600;  -- 10 symbols * 60 minutes

-- Query 7: Alert Rule Performance Summary
WITH alerts AS (
    SELECT 
        symbol,
        bucket,
        CASE 
            WHEN (high - low) / low > 0.05 THEN 'range_threshold'
            WHEN vol > (SELECT avg(vol) FROM finpull.rt_candles_1m WHERE symbol = rt.symbol AND bucket >= now() - INTERVAL 1 HOUR) * 3 THEN 'volume_spike'
            ELSE NULL
        END AS alert_type
    FROM finpull.rt_candles_1m rt
    WHERE bucket >= now() - INTERVAL 24 HOUR
)
SELECT 
    alert_type,
    count(*) AS trigger_count,
    count(DISTINCT symbol) AS symbols_affected,
    min(bucket) AS first_trigger,
    max(bucket) AS last_trigger
FROM alerts
WHERE alert_type IS NOT NULL
GROUP BY alert_type
ORDER BY trigger_count DESC;

-- Query 8: Top Alerted Symbols (24h)
WITH alerts AS (
    SELECT 
        symbol,
        bucket,
        CASE 
            WHEN (high - low) / low > 0.05 THEN 1
            WHEN vol > (SELECT avg(vol) FROM finpull.rt_candles_1m WHERE symbol = rt.symbol AND bucket >= now() - INTERVAL 1 HOUR) * 3 THEN 1
            ELSE 0
        END AS is_alert
    FROM finpull.rt_candles_1m rt
    WHERE bucket >= now() - INTERVAL 24 HOUR
)
SELECT 
    symbol,
    sum(is_alert) AS alert_count,
    anyLast(close) AS last_price,
    sum(vol) AS total_volume_24h
FROM alerts
GROUP BY symbol
HAVING alert_count > 0
ORDER BY alert_count DESC
LIMIT 20;

