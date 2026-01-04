#!/usr/bin/env bash
set -euo pipefail

# Defaults for admin bootstrap if not set
: "${ADMIN_USERNAME:=admin}"
: "${ADMIN_PASSWORD:=admin}"
: "${ADMIN_FIRST_NAME:=Admin}"
: "${ADMIN_LAST_NAME:=User}"
: "${ADMIN_EMAIL:=admin@example.com}"

superset db upgrade

# Create admin if not exists
superset fab create-admin \
  --username "$ADMIN_USERNAME" \
  --firstname "$ADMIN_FIRST_NAME" \
  --lastname "$ADMIN_LAST_NAME" \
  --email "$ADMIN_EMAIL" \
  --password "$ADMIN_PASSWORD" || true

superset init

# Auto-configure ClickHouse database connection
echo ""
echo "Configuring ClickHouse database connection..."
python /app/setup_clickhouse.py || echo "⚠ Warning: ClickHouse auto-configuration failed, you can add it manually"

echo ""
echo "Starting Superset server..."
exec gunicorn -w 2 -k gthread --threads 16 --timeout 120 -b 0.0.0.0:8088 'superset.app:create_app()'


