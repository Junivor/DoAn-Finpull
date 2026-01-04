-- ==============================
-- STAGING SCHEMA: finpull_staging
-- ==============================

CREATE DATABASE IF NOT EXISTS finpull_staging;

-- Raw ticks table
CREATE TABLE IF NOT EXISTS finpull_staging.rt_ticks_raw (
                                                            ts DateTime64(3, 'UTC'),
    symbol LowCardinality(String),
    price Float64 CODEC(ZSTD(3)),
    volume Float64 CODEC(ZSTD(3)),
    source LowCardinality(String),
    event_id String,
    seq UInt64,
    org_id LowCardinality(String)
    ) ENGINE = MergeTree()
    PARTITION BY toDate(ts)
    ORDER BY (symbol, ts, seq)
    TTL toDateTime(ts, 'Asia/Ho_Chi_Minh') + INTERVAL 30 DAY RECOMPRESS CODEC(ZSTD(12))
    SETTINGS index_granularity = 8192;

-- Candles table 1s
CREATE TABLE IF NOT EXISTS finpull_staging.rt_candles_1s (
                                                             bucket DateTime('Asia/Ho_Chi_Minh'),
    symbol LowCardinality(String),
    open Float64 CODEC(ZSTD(3)),
    high Float64 CODEC(ZSTD(3)),
    low Float64 CODEC(ZSTD(3)),
    close Float64 CODEC(ZSTD(3)),
    vol Float64 CODEC(ZSTD(3)),
    org_id LowCardinality(String)
    ) ENGINE = MergeTree()
    PARTITION BY toDate(bucket)
    ORDER BY (symbol, bucket)
    TTL bucket + INTERVAL 90 DAY RECOMPRESS CODEC(ZSTD(12))
    SETTINGS index_granularity = 8192;

-- Candles table 1m
CREATE TABLE IF NOT EXISTS finpull_staging.rt_candles_1m AS finpull_staging.rt_candles_1s
    ENGINE = MergeTree()
    PARTITION BY toDate(bucket)
    ORDER BY (symbol, bucket)
    TTL bucket + INTERVAL 180 DAY RECOMPRESS CODEC(ZSTD(12));

-- MV: ticks → 1s candles
CREATE MATERIALIZED VIEW IF NOT EXISTS finpull_staging.mv_ticks_to_1s
TO finpull_staging.rt_candles_1s AS
SELECT
    toStartOfSecond(convertTimezone('UTC','Asia/Ho_Chi_Minh', toDateTime(ts))) AS bucket,
    symbol,
    anyLast(price)  AS close,
    argMin(price, ts) AS open,
    max(price)      AS high,
    min(price)      AS low,
    sum(volume)     AS vol,
    anyLast(org_id) AS org_id
FROM finpull_staging.rt_ticks_raw
GROUP BY bucket, symbol;

-- MV: 1s candles → 1m candles
CREATE MATERIALIZED VIEW IF NOT EXISTS finpull_staging.mv_1s_to_1m
TO finpull_staging.rt_candles_1m AS
SELECT
    toStartOfMinute(bucket) AS bucket,
    symbol,
    argMin(open, bucket) AS open,
    max(high)            AS high,
    min(low)             AS low,
    anyLast(close)       AS close,
    sum(vol)             AS vol,
    anyLast(org_id)      AS org_id
FROM finpull_staging.rt_candles_1s
GROUP BY bucket, symbol;

-- == End Phase 1 DDL (finpull_staging) ==
