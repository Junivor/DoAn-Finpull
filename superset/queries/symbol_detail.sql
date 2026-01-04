-- ============================================================================
-- Symbol Detail Dashboard - SQL Queries for Superset
-- ============================================================================
-- Replace {SYMBOL} with actual symbol, e.g., 'BINANCE:BTCUSDT'

-- Query 1: OHLC Candlestick Chart (24h, 1m candles)
SELECT 
    bucket AS timestamp,
    open,
    high,
    low,
    close,
    vol AS volume
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440;  -- 24h * 60min

-- Query 2: Volume Profile by Hour (24h)
SELECT 
    toStartOfHour(bucket) AS hour,
    sum(vol) AS volume,
    avg(close) AS avg_price,
    max(high) AS max_price,
    min(low) AS min_price
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
GROUP BY hour
ORDER BY hour DESC;

-- Query 3: Price Range (High/Low/Close) - Line Chart
SELECT 
    bucket AS timestamp,
    high AS price_high,
    low AS price_low,
    close AS price_close,
    open AS price_open
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440;

-- Query 4: Realized Volatility Over Time
SELECT 
    bucket AS timestamp,
    stddevPop(close) OVER (
        PARTITION BY symbol 
        ORDER BY bucket 
        ROWS BETWEEN 59 PRECEDING AND CURRENT ROW
    ) / avg(close) OVER (
        PARTITION BY symbol 
        ORDER BY bucket 
        ROWS BETWEEN 59 PRECEDING AND CURRENT ROW
    ) * 100 AS realized_vol_pct
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440;

-- Query 5: Volume Distribution (Histogram)
SELECT 
    CASE 
        WHEN vol < 1000 THEN '<1k'
        WHEN vol < 10000 THEN '1k-10k'
        WHEN vol < 100000 THEN '10k-100k'
        WHEN vol < 1000000 THEN '100k-1M'
        ELSE '>1M'
    END AS volume_bucket,
    count(*) AS candle_count,
    avg(close) AS avg_price
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
GROUP BY volume_bucket
ORDER BY 
    CASE volume_bucket
        WHEN '<1k' THEN 1
        WHEN '1k-10k' THEN 2
        WHEN '10k-100k' THEN 3
        WHEN '100k-1M' THEN 4
        ELSE 5
    END;

-- Query 6: Price Statistics Summary
SELECT 
    symbol,
    min(low) AS price_min_24h,
    max(high) AS price_max_24h,
    avg(close) AS price_avg_24h,
    anyLast(close) AS price_last,
    100 * (anyLast(close) - min(low)) / min(low) AS pct_change_24h,
    sum(vol) AS total_volume_24h,
    count(*) AS candle_count
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
GROUP BY symbol;

-- Query 7: Regime Indicators (if available from analytics)
-- This would join with analytics results if stored in ClickHouse
-- Placeholder for future integration
SELECT 
    bucket AS timestamp,
    close AS price,
    vol AS volume,
    'normal' AS regime  -- Would come from analytics service
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440;

-- Query 8: Anomaly Markers (if anomalies stored)
-- Placeholder - would require anomaly table or MV
SELECT 
    bucket AS timestamp,
    CASE 
        WHEN (high - low) / low > 0.05 THEN 'price_shock'
        WHEN vol > avg(vol) OVER (ORDER BY bucket ROWS BETWEEN 59 PRECEDING AND CURRENT ROW) * 3 THEN 'volume_spike'
        ELSE 'normal'
    END AS anomaly_type,
    close AS price
FROM finpull.rt_candles_1m
WHERE symbol = '{SYMBOL}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440;

