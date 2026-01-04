#!/bin/bash
# Script to restart FinPull with updated configuration

set -e

echo "=== FinPull Restart Script ==="
echo ""

# Check if we're using Docker or binary
if [ -f "docker/docker-compose.yml" ]; then
    echo "Found Docker Compose configuration"
    echo "Restarting FinPull via Docker..."
    cd docker
    docker-compose down finpull 2>/dev/null || true
    docker-compose up -d finpull
    echo ""
    echo "✅ FinPull restarted via Docker"
    echo ""
    echo "To view logs:"
    echo "  docker logs -f finpull-app"
    echo ""
    echo "To check subscriptions:"
    echo "  docker logs finpull-app | grep 'finnhub: subscribed'"
else
    echo "Docker Compose not found, using binary..."

    # Load environment variables
    if [ -f "config.dev.env" ]; then
        echo "Loading config.dev.env..."
        export $(cat config.dev.env | grep -v '^#' | xargs)
    fi

    # Kill existing process if running
    pkill -f bin/finpull || true
    sleep 2

    # Start the application
    echo "Starting FinPull..."
    nohup ./bin/finpull -config config/config.yaml > finpull.log 2>&1 &

    echo ""
    echo "✅ FinPull started"
    echo ""
    echo "To view logs:"
    echo "  tail -f finpull.log"
    echo ""
    echo "To check subscriptions:"
    echo "  grep 'finnhub: subscribed' finpull.log"
fi

echo ""
echo "Expected subscriptions:"
echo "  - BINANCE:BTCUSDT"
echo "  - OANDA:XAU_USD"
echo "  - OANDA:WTICO_USD"
echo "  - OANDA:XAG_USD"

