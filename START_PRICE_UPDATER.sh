#!/bin/bash

# Start Continuous Betfair Price Updater
# Runs every 30 minutes, updates win_ppwap for tomorrow's races

DATE="${1:-$(date -d tomorrow +%Y-%m-%d)}"
INTERVAL="${2:-30}"  # minutes

echo "Starting Betfair Price Updater..."
echo "Target Date: $DATE"
echo "Update Interval: $INTERVAL minutes"
echo ""
echo "Press Ctrl+C to stop"
echo ""

cd /home/smonaghan/GiddyUp/backend-api

# Build if needed
if [ ! -f bin/continuous_price_updater ]; then
    echo "Building price updater..."
    go build -o bin/continuous_price_updater cmd/continuous_price_updater/main.go
fi

# Load environment
source ../settings.env

# Run in foreground (or use nohup for background)
./bin/continuous_price_updater --date="$DATE" --interval=$INTERVAL
