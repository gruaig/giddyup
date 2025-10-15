#!/bin/bash

# GiddyUp API Server Startup Script

PORT=${SERVER_PORT:-8000}
LOG_FILE=${LOG_FILE:-/tmp/giddyup-api.log}

echo "🚀 Starting GiddyUp API Server"
echo "Port: $PORT"
echo "Log file: $LOG_FILE"

# Kill existing processes on the port
echo "Cleaning up existing processes..."
lsof -ti:$PORT 2>/dev/null | xargs -r kill -9 2>/dev/null
fuser -k $PORT/tcp 2>/dev/null
sleep 1

# Start server
cd /home/smonaghan/GiddyUp/backend-api

echo "Starting server..."
SERVER_PORT=$PORT LOG_LEVEL=DEBUG ./bin/api > $LOG_FILE 2>&1 &
SERVER_PID=$!

echo "Server PID: $SERVER_PID"
echo $SERVER_PID > /tmp/giddyup-api.pid

# Wait and verify
sleep 3

if curl -s http://localhost:$PORT/health > /dev/null 2>&1; then
    echo "✅ Server is running on http://localhost:$PORT"
    echo "   Health: http://localhost:$PORT/health"
    echo "   API: http://localhost:$PORT/api/v1/"
    echo "   Logs: tail -f $LOG_FILE"
    exit 0
else
    echo "❌ Server failed to start"
    echo "Check logs: cat $LOG_FILE"
    exit 1
fi

