#!/bin/bash

# GiddyUp API Server Startup Script (Background)
# Starts the API in the background with proper logging

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🏇 Starting GiddyUp API Server (background mode)...${NC}"

# Check if we're in the right directory
if [ ! -f "bin/api" ]; then
    echo -e "${RED}❌ Error: bin/api not found. Please build first:${NC}"
    echo -e "   go build -o bin/api cmd/api/main.go"
    exit 1
fi

# Source environment variables
if [ -f "../settings.env" ]; then
    echo -e "${GREEN}✅ Loading environment variables from settings.env${NC}"
    source ../settings.env
else
    echo -e "${YELLOW}⚠️  Warning: settings.env not found, using defaults${NC}"
fi

# Create logs directory if it doesn't exist
mkdir -p logs

# Kill any existing API process on port 8000
if lsof -ti:8000 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Port 8000 is in use, killing existing process...${NC}"
    lsof -ti:8000 | xargs kill -9 2>/dev/null || true
    sleep 2
fi

# Start the API server in background
echo -e "${BLUE}🚀 Starting API on port ${PORT:-8000} (background)...${NC}"
echo -e "${BLUE}📝 Logging to: logs/server.log${NC}"
echo ""

nohup ./bin/api > logs/server.log 2>&1 &
API_PID=$!

sleep 2

# Check if process is still running
if ps -p $API_PID > /dev/null; then
    echo -e "${GREEN}✅ API server started successfully!${NC}"
    echo -e "${GREEN}   PID: $API_PID${NC}"
    echo -e "${GREEN}   URL: http://localhost:${PORT:-8000}${NC}"
    echo -e "${GREEN}   Health: http://localhost:${PORT:-8000}/health${NC}"
    echo ""
    echo -e "${BLUE}📊 View logs:${NC}"
    echo -e "   tail -f logs/server.log"
    echo ""
    echo -e "${BLUE}🛑 Stop server:${NC}"
    echo -e "   kill $API_PID"
    echo -e "   # or"
    echo -e "   pkill -f 'bin/api'"
else
    echo -e "${RED}❌ Failed to start API server. Check logs/server.log${NC}"
    cat logs/server.log | tail -20
    exit 1
fi
