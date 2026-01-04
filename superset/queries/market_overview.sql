-- ============================================================================
-- Market Overview Dashboard - SQL Queries for Superset
-- ============================================================================

-- Query 1: Top Movers 1h (Heatmap)
-- Shows percentage change by symbol and hour
WITH now() AS t
SELECT 
    symbol,
    toHour(bucket) AS hour,
    100 * (anyLast(close) - anyLastIf(close, bucket <= t - INTERVAL 1 HOUR))
         / anyLastIf(close, bucket <= t - INTERVAL 1 HOUR) AS pct_change,
    sum(vol) AS volume_1h
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol, hour
HAVING volume_1h > 1000000  -- Minimum liquidity filter
ORDER BY pct_change DESC
LIMIT 100;

-- Query 2: Volume Rank (Top 20 by 1h volume)
WITH now() AS t
SELECT 
    symbol,
    sum(vol) AS volume_1h,
    anyLast(close) AS last_price,
    100 * (anyLast(close) - min(close)) / min(close) AS pct_change_1h
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
ORDER BY volume_1h DESC
LIMIT 20;

-- Query 3: Volatility Rank (Top 20 by realized volatility)
WITH now() AS t
SELECT 
    symbol,
    stddevPop(close) / avg(close) * 100 AS volatility_pct,
    sum(vol) AS volume_1h,
    anyLast(close) AS last_price
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
HAVING volume_1h > 1000000
ORDER BY volatility_pct DESC
LIMIT 20;

-- Query 4: % Change Distribution (Histogram)
WITH now() AS t
SELECT 
    symbol,
    100 * (anyLast(close) - anyLastIf(close, bucket <= t - INTERVAL 1 HOUR))
         / anyLastIf(close, bucket <= t - INTERVAL 1 HOUR) AS pct_1h
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
HAVING sum(vol) > 1000000;

-- Query 5: Market Heatmap by Timeframe
-- Shows price movement heatmap (5m/1h/1d)
SELECT 
    symbol,
    CASE 
        WHEN bucket >= now() - INTERVAL 5 MINUTE THEN '5m'
        WHEN bucket >= now() - INTERVAL 1 HOUR THEN '1h'
        WHEN bucket >= now() - INTERVAL 1 DAY THEN '1d'
    END AS timeframe,
    100 * (max(close) - min(close)) / min(close) AS pct_range
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 1 DAY
GROUP BY symbol, timeframe
HAVING sum(vol) > 1000000
ORDER BY pct_range DESC
LIMIT 50;

-- Query 6: Top Gainers/Losers (1h)
WITH now() AS t,
ranked AS (
    SELECT 
        symbol,
        100 * (anyLast(close) - anyLastIf(close, bucket <= t - INTERVAL 1 HOUR))
             / anyLastIf(close, bucket <= t - INTERVAL 1 HOUR) AS pct_1h,
        sum(vol) AS volume_1h,
        anyLast(close) AS last_price
    FROM finpull.rt_candles_1m
    WHERE bucket >= t - INTERVAL 1 HOUR
    GROUP BY symbol
    HAVING volume_1h > 1000000
)
SELECT * FROM ranked
ORDER BY pct_1h DESC
LIMIT 10
UNION ALL
SELECT * FROM ranked
ORDER BY pct_1h ASC
LIMIT 10;

