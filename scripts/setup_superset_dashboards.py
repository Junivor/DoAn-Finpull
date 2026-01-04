#!/usr/bin/env python3
"""
Superset Dashboard Setup Script

Automatically creates:
1. ClickHouse database connection
2. Datasets (virtual tables)
3. Charts for Market Overview, Symbol Detail, Alert Monitor
4. Dashboards with charts

Usage:
    python scripts/setup_superset_dashboards.py \
        --superset-url http://localhost:8088 \
        --username admin \
        --password admin \
        --clickhouse-host clickhouse \
        --clickhouse-port 8123 \
        --clickhouse-user default \
        --clickhouse-password "" \
        --clickhouse-db finpull
"""

import argparse
import json
import os
import sys
import time
from typing import Any, Dict, List, Optional
import requests
from requests.auth import HTTPBasicAuth


class SupersetClient:
    """Client for Superset REST API."""
    
    def __init__(self, base_url: str, username: str, password: str):
        self.base_url = base_url.rstrip('/')
        self.username = username
        self.password = password
        self.session = requests.Session()
        self.csrf_token: Optional[str] = None
        self._login()
    
    def _login(self):
        """Login and get CSRF token."""
        login_url = f"{self.base_url}/api/v1/security/login"
        resp = self.session.post(login_url, json={
            "username": self.username,
            "password": self.password,
            "provider": "db",
            "refresh": True
        })
        resp.raise_for_status()
        data = resp.json()
        self.access_token = data["access_token"]
        self.session.headers.update({
            "Authorization": f"Bearer {self.access_token}",
            "Content-Type": "application/json"
        })
        
        # Get CSRF token
        csrf_resp = self.session.get(f"{self.base_url}/api/v1/security/csrf_token/")
        csrf_resp.raise_for_status()
        self.csrf_token = csrf_resp.json()["result"]
    
    def create_database(self, name: str, sqlalchemy_uri: str, display_name: Optional[str] = None) -> Dict[str, Any]:
        """Create database connection."""
        url = f"{self.base_url}/api/v1/database/"
        payload = {
            "database_name": display_name or name,
            "sqlalchemy_uri": sqlalchemy_uri,
            "cache_timeout": 3600,
            "expose_in_sqllab": True,
        }
        resp = self.session.post(url, json=payload)
        if resp.status_code == 422:
            # Database might exist, try to find it
            list_resp = self.session.get(f"{self.base_url}/api/v1/database/?q=(filters:!((col:database_name,opr:eq,value:{name})))")
            if list_resp.status_code == 200:
                results = list_resp.json().get("result", [])
                if results:
                    return results[0]
        resp.raise_for_status()
        return resp.json()["result"]
    
    def create_dataset(
        self,
        db_id: int,
        table_name: str,
        schema: Optional[str] = None,
        display_name: Optional[str] = None
    ) -> Dict[str, Any]:
        """Create dataset (virtual table)."""
        url = f"{self.base_url}/api/v1/dataset/"
        payload = {
            "database": db_id,
            "table_name": table_name,
            "schema": schema,
            "owners": [],
        }
        if display_name:
            payload["table_name"] = display_name
        
        resp = self.session.post(url, json=payload)
        if resp.status_code == 422:
            # Dataset might exist
            list_resp = self.session.get(
                f"{self.base_url}/api/v1/dataset/?"
                f"q=(filters:!((col:table_name,opr:eq,value:{table_name})))"
            )
            if list_resp.status_code == 200:
                results = list_resp.json().get("result", [])
                if results:
                    return results[0]
        resp.raise_for_status()
        return resp.json()["result"]
    
    def create_chart(
        self,
        dataset_id: int,
        chart_type: str,
        viz_type: str,
        params: Dict[str, Any],
        chart_name: str
    ) -> Dict[str, Any]:
        """Create chart."""
        url = f"{self.base_url}/api/v1/chart/"
        payload = {
            "slice_name": chart_name,
            "viz_type": viz_type,
            "datasource_id": dataset_id,
            "datasource_type": "table",
            "params": json.dumps(params),
            "owners": [],
        }
        resp = self.session.post(url, json=payload)
        if resp.status_code == 422:
            # Chart might exist
            list_resp = self.session.get(
                f"{self.base_url}/api/v1/chart/?"
                f"q=(filters:!((col:slice_name,opr:eq,value:{chart_name})))"
            )
            if list_resp.status_code == 200:
                results = list_resp.json().get("result", [])
                if results:
                    return results[0]
        resp.raise_for_status()
        return resp.json()["result"]
    
    def create_dashboard(
        self,
        dashboard_title: str,
        chart_ids: List[int],
        positions: Optional[Dict[str, Any]] = None
    ) -> Dict[str, Any]:
        """Create dashboard with charts."""
        url = f"{self.base_url}/api/v1/dashboard/"
        
        # Default positions (grid layout)
        if positions is None:
            positions = {}
            cols = 12
            x, y = 0, 0
            for i, chart_id in enumerate(chart_ids):
                positions[f"{chart_id}"] = {
                    "x": (i % 2) * cols,
                    "y": (i // 2) * 4,
                    "w": cols,
                    "h": 4
                }
        
        payload = {
            "dashboard_title": dashboard_title,
            "slug": dashboard_title.lower().replace(" ", "-"),
            "published": True,
            "charts": chart_ids,
            "position_json": json.dumps(positions),
            "owners": [],
        }
        
        resp = self.session.post(url, json=payload)
        if resp.status_code == 422:
            # Dashboard might exist
            list_resp = self.session.get(
                f"{self.base_url}/api/v1/dashboard/?"
                f"q=(filters:!((col:dashboard_title,opr:eq,value:{dashboard_title})))"
            )
            if list_resp.status_code == 200:
                results = list_resp.json().get("result", [])
                if results:
                    return results[0]
        resp.raise_for_status()
        return resp.json()["result"]


def create_clickhouse_uri(host: str, port: int, user: str, password: str, db: str) -> str:
    """Create ClickHouse SQLAlchemy URI."""
    if password:
        return f"clickhouse+http://{user}:{password}@{host}:{port}/{db}"
    else:
        return f"clickhouse+http://{user}@{host}:{port}/{db}"


def create_market_overview_charts(client: SupersetClient, candles_dataset_id: int) -> List[int]:
    """Create charts for Market Overview dashboard."""
    charts = []
    
    # 1. Top Movers 1h (heatmap)
    chart1 = client.create_chart(
        candles_dataset_id,
        chart_type="heatmap",
        viz_type="heatmap",
        params={
            "queryType": "sql",
            "sql": """
WITH now() AS t
SELECT 
    symbol,
    toHour(bucket) AS hour,
    100 * (anyLast(close) - anyLastIf(close, bucket <= t - INTERVAL 1 HOUR))
         / anyLastIf(close, bucket <= t - INTERVAL 1 HOUR) AS pct_change
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol, hour
HAVING sum(vol) > 1e6
ORDER BY pct_change DESC
LIMIT 100
""",
            "columns": ["symbol", "hour", "pct_change"],
        },
        chart_name="Top Movers 1h (Heatmap)"
    )
    charts.append(chart1["id"])
    
    # 2. Volume Rank
    chart2 = client.create_chart(
        candles_dataset_id,
        chart_type="bar",
        viz_type="dist_bar",
        params={
            "queryType": "sql",
            "sql": """
WITH now() AS t
SELECT 
    symbol,
    sum(vol) AS volume_1h
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
ORDER BY volume_1h DESC
LIMIT 20
""",
            "columns": ["symbol", "volume_1h"],
        },
        chart_name="Volume Rank (1h)"
    )
    charts.append(chart2["id"])
    
    # 3. Volatility Rank
    chart3 = client.create_chart(
        candles_dataset_id,
        chart_type="bar",
        viz_type="dist_bar",
        params={
            "queryType": "sql",
            "sql": """
WITH now() AS t
SELECT 
    symbol,
    stddevPop(close) / avg(close) * 100 AS volatility_pct
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
HAVING sum(vol) > 1e6
ORDER BY volatility_pct DESC
LIMIT 20
""",
            "columns": ["symbol", "volatility_pct"],
        },
        chart_name="Volatility Rank (1h)"
    )
    charts.append(chart3["id"])
    
    # 4. % Change Distribution
    chart4 = client.create_chart(
        candles_dataset_id,
        chart_type="histogram",
        viz_type="histogram",
        params={
            "queryType": "sql",
            "sql": """
WITH now() AS t
SELECT 
    100 * (anyLast(close) - anyLastIf(close, bucket <= t - INTERVAL 1 HOUR))
         / anyLastIf(close, bucket <= t - INTERVAL 1 HOUR) AS pct_1h
FROM finpull.rt_candles_1m
WHERE bucket >= t - INTERVAL 1 HOUR
GROUP BY symbol
HAVING sum(vol) > 1e6
""",
            "columns": ["pct_1h"],
        },
        chart_name="% Change Distribution (1h)"
    )
    charts.append(chart4["id"])
    
    return charts


def create_symbol_detail_charts(client: SupersetClient, candles_dataset_id: int, symbol: str = "BINANCE:BTCUSDT") -> List[int]:
    """Create charts for Symbol Detail dashboard."""
    charts = []
    
    # 1. OHLC Candlestick
    chart1 = client.create_chart(
        candles_dataset_id,
        chart_type="candlestick",
        viz_type="candlestick",
        params={
            "queryType": "sql",
            "sql": f"""
SELECT 
    bucket AS timestamp,
    open,
    high,
    low,
    close,
    vol AS volume
FROM finpull.rt_candles_1m
WHERE symbol = '{symbol}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440
""",
            "columns": ["timestamp", "open", "high", "low", "close", "volume"],
        },
        chart_name=f"{symbol} - OHLC (24h)"
    )
    charts.append(chart1["id"])
    
    # 2. Volume Profile
    chart2 = client.create_chart(
        candles_dataset_id,
        chart_type="bar",
        viz_type="dist_bar",
        params={
            "queryType": "sql",
            "sql": f"""
SELECT 
    toStartOfHour(bucket) AS hour,
    sum(vol) AS volume
FROM finpull.rt_candles_1m
WHERE symbol = '{symbol}'
    AND bucket >= now() - INTERVAL 24 HOUR
GROUP BY hour
ORDER BY hour DESC
""",
            "columns": ["hour", "volume"],
        },
        chart_name=f"{symbol} - Volume Profile (24h)"
    )
    charts.append(chart2["id"])
    
    # 3. Price Range
    chart3 = client.create_chart(
        candles_dataset_id,
        chart_type="line",
        viz_type="line",
        params={
            "queryType": "sql",
            "sql": f"""
SELECT 
    bucket AS timestamp,
    high AS price_high,
    low AS price_low,
    close AS price_close
FROM finpull.rt_candles_1m
WHERE symbol = '{symbol}'
    AND bucket >= now() - INTERVAL 24 HOUR
ORDER BY bucket DESC
LIMIT 1440
""",
            "columns": ["timestamp", "price_high", "price_low", "price_close"],
        },
        chart_name=f"{symbol} - Price Range (24h)"
    )
    charts.append(chart3["id"])
    
    return charts


def create_alert_monitor_charts(client: SupersetClient, candles_dataset_id: int) -> List[int]:
    """Create charts for Alert Monitor dashboard."""
    charts = []
    
    # 1. Alert Rule Triggers (placeholder - would need alert table)
    chart1 = client.create_chart(
        candles_dataset_id,
        chart_type="table",
        viz_type="table",
        params={
            "queryType": "sql",
            "sql": """
SELECT 
    symbol,
    count(*) AS trigger_count,
    max(bucket) AS last_trigger
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 24 HOUR
    AND (high - low) / low > 0.05  -- Example: 5% range trigger
GROUP BY symbol
ORDER BY trigger_count DESC
LIMIT 50
""",
            "columns": ["symbol", "trigger_count", "last_trigger"],
        },
        chart_name="Potential Alerts (5% Range)"
    )
    charts.append(chart1["id"])
    
    # 2. Precision@N (placeholder)
    chart2 = client.create_chart(
        candles_dataset_id,
        chart_type="line",
        viz_type="line",
        params={
            "queryType": "sql",
            "sql": """
SELECT 
    toStartOfHour(bucket) AS hour,
    count(DISTINCT symbol) AS symbols_monitored
FROM finpull.rt_candles_1m
WHERE bucket >= now() - INTERVAL 24 HOUR
GROUP BY hour
ORDER BY hour DESC
""",
            "columns": ["hour", "symbols_monitored"],
        },
        chart_name="Symbols Monitored Over Time"
    )
    charts.append(chart2["id"])
    
    # 3. Latency Strip Chart (would need latency metrics table)
    chart3 = client.create_chart(
        candles_dataset_id,
        chart_type="scatter",
        viz_type="scatter",
        params={
            "queryType": "sql",
            "sql": """
SELECT 
    bucket AS timestamp,
    symbol,
    (high - low) / low AS volatility
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
ORDER BY bucket DESC
LIMIT 600
""",
            "columns": ["timestamp", "symbol", "volatility"],
        },
        chart_name="Volatility Strip Chart (Top 10 by Volume)"
    )
    charts.append(chart3["id"])
    
    return charts


def main():
    parser = argparse.ArgumentParser(description="Setup Superset dashboards for FinPull")
    parser.add_argument("--superset-url", default="http://localhost:8088", help="Superset base URL")
    parser.add_argument("--username", default="admin", help="Superset admin username")
    parser.add_argument("--password", required=True, help="Superset admin password")
    parser.add_argument("--clickhouse-host", default="clickhouse", help="ClickHouse host")
    parser.add_argument("--clickhouse-port", type=int, default=8123, help="ClickHouse HTTP port")
    parser.add_argument("--clickhouse-user", default="default", help="ClickHouse user")
    parser.add_argument("--clickhouse-password", default="", help="ClickHouse password")
    parser.add_argument("--clickhouse-db", default="finpull", help="ClickHouse database")
    parser.add_argument("--dry-run", action="store_true", help="Print what would be created without creating")
    
    args = parser.parse_args()
    
    if args.dry_run:
        print("DRY RUN - Would create:")
        print(f"  Database: ClickHouse connection to {args.clickhouse_host}:{args.clickhouse_port}")
        print(f"  Datasets: rt_candles_1m")
        print(f"  Dashboards: Market Overview, Symbol Detail, Alert Monitor")
        return
    
    print("Connecting to Superset...")
    client = SupersetClient(args.superset_url, args.username, args.password)
    
    # Create database connection
    print("Creating ClickHouse database connection...")
    ch_uri = create_clickhouse_uri(
        args.clickhouse_host,
        args.clickhouse_port,
        args.clickhouse_user,
        args.clickhouse_password,
        args.clickhouse_db
    )
    db = client.create_database("finpull_clickhouse", ch_uri, "FinPull ClickHouse")
    db_id = db["id"]
    print(f"  Database created/updated: ID={db_id}")
    
    # Create dataset
    print("Creating dataset rt_candles_1m...")
    dataset = client.create_dataset(db_id, "rt_candles_1m", schema="finpull", display_name="Real-time Candles 1m")
    dataset_id = dataset["id"]
    print(f"  Dataset created/updated: ID={dataset_id}")
    
    # Create Market Overview dashboard
    print("Creating Market Overview dashboard...")
    market_charts = create_market_overview_charts(client, dataset_id)
    market_dashboard = client.create_dashboard("FinPull - Market Overview", market_charts)
    print(f"  Dashboard created: ID={market_dashboard['id']}, URL={args.superset_url}/superset/dashboard/{market_dashboard['id']}")
    
    # Create Symbol Detail dashboard
    print("Creating Symbol Detail dashboard...")
    symbol_charts = create_symbol_detail_charts(client, dataset_id)
    symbol_dashboard = client.create_dashboard("FinPull - Symbol Detail", symbol_charts)
    print(f"  Dashboard created: ID={symbol_dashboard['id']}, URL={args.superset_url}/superset/dashboard/{symbol_dashboard['id']}")
    
    # Create Alert Monitor dashboard
    print("Creating Alert Monitor dashboard...")
    alert_charts = create_alert_monitor_charts(client, dataset_id)
    alert_dashboard = client.create_dashboard("FinPull - Alert Monitor", alert_charts)
    print(f"  Dashboard created: ID={alert_dashboard['id']}, URL={args.superset_url}/superset/dashboard/{alert_dashboard['id']}")
    
    print("\n✅ All dashboards created successfully!")
    print(f"\nAccess Superset at: {args.superset_url}")


if __name__ == "__main__":
    main()

