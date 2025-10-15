#!/bin/bash
# Start GiddyUp API server with verbose logging

echo "🚀 Starting GiddyUp API with verbose logging..."
echo ""

# Set log level to DEBUG for maximum verbosity
export LOG_LEVEL=DEBUG

# Enable auto-update (optional - comment out if you don't want it)
export AUTO_UPDATE_ON_STARTUP=true

# Set data directory
export DATA_DIR=/home/smonaghan/GiddyUp/data

# Logs will be written to logs/server.log by default
# You can change this with LOG_DIR environment variable
# export LOG_DIR=/path/to/custom/logs

# Start the server
cd /home/smonaghan/GiddyUp/backend-api
./bin/api

# Logs are being written to:
# - stdout/stderr (your terminal)
# - logs/server.log (file)
#
# To tail the log file in another terminal:
#   tail -f logs/server.log

