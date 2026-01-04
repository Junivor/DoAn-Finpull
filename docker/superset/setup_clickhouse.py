#!/usr/bin/env python3
"""
Auto-configure ClickHouse database connection in Superset
This script runs after Superset initialization to create a pre-configured database connection
"""
import os
import sys

def create_clickhouse_database():
    """Create ClickHouse database connection if it doesn't exist"""
    try:
        from superset import app, db
        from superset.models.core import Database

        # Create app context
        with app.app.app_context():
            # Check if ClickHouse database already exists
            existing_db = db.session.query(Database).filter_by(
                database_name="ClickHouse - FinPull"
            ).first()

            if existing_db:
                print("✓ ClickHouse database connection already exists")
                return True

            # Get connection details from environment
            clickhouse_host = os.getenv('CLICKHOUSE_HOST', 'clickhouse')
            clickhouse_port = os.getenv('CLICKHOUSE_PORT', '8123')
            clickhouse_database = os.getenv('CLICKHOUSE_DATABASE', 'finpull_dev')
            clickhouse_user = os.getenv('CLICKHOUSE_USER', 'default')
            clickhouse_password = os.getenv('CLICKHOUSE_PASSWORD', '')

            # Build SQLAlchemy URI for ClickHouse
            # Format: clickhousedb://user:password@host:port/database
            if clickhouse_password:
                sqlalchemy_uri = f"clickhousedb://{clickhouse_user}:{clickhouse_password}@{clickhouse_host}:{clickhouse_port}/{clickhouse_database}"
            else:
                sqlalchemy_uri = f"clickhousedb://{clickhouse_user}@{clickhouse_host}:{clickhouse_port}/{clickhouse_database}"

            # Create new database connection
            new_database = Database(
                database_name="ClickHouse - FinPull",
                sqlalchemy_uri=sqlalchemy_uri,
                expose_in_sqllab=True,
                allow_run_async=True,
                allow_ctas=True,
                allow_cvas=True,
                allow_dml=True,
                extra='{"allows_virtual_table_explore": true, "allows_subquery": true}'
            )

            db.session.add(new_database)
            db.session.commit()

            print("========================================")
            print("✓ ClickHouse database connection created successfully!")
            print(f"  Database Name: ClickHouse - FinPull")
            print(f"  Host: {clickhouse_host}:{clickhouse_port}")
            print(f"  Database: {clickhouse_database}")
            print("========================================")
            return True

    except Exception as e:
        print(f"⚠ Warning: Failed to create ClickHouse database connection: {e}", file=sys.stderr)
        print("  You can manually add the connection in Superset UI", file=sys.stderr)
        return False

if __name__ == "__main__":
    create_clickhouse_database()

