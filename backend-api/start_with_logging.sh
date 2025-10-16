#!/bin/bash

# GiddyUp API Server Startup Script with Logging
# This script sources environment variables and starts the API with proper logging

set -e  # Exit on error

# Colors for output
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}🏇 Starting GiddyUp API Server with logging...${NC}"
echo ""

# Check if we're in the right directory
if [ ! -f "bin/api" ]; then
    echo -e "${RED}❌ Error: bin/api not found. Please build first:${NC}"
    echo -e "   go build -o bin/api cmd/api/main.go"
    exit 1
fi

# Source environment variables from settings.env
if [ -f "../settings.env" ]; then
    echo -e "${GREEN}✅ Loading environment variables from settings.env${NC}"
    source ../settings.env
else
    echo -e "${YELLOW}⚠️  Warning: settings.env not found in parent directory${NC}"
    echo -e "${YELLOW}   Using default configuration${NC}"
fi

# Override/set specific logging variables
export LOG_LEVEL=info
export AUTO_UPDATE_ON_STARTUP=true
export DATA_DIR=${DATA_DIR:-/home/smonaghan/GiddyUp/data}

# Create logs directory if it doesn't exist
mkdir -p logs

# Kill any existing API process on port 8000
if lsof -ti:8000 > /dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Port 8000 is in use, killing existing process...${NC}"
    lsof -ti:8000 | xargs kill -9 2>/dev/null || true
    sleep 2
fi

echo ""
echo -e "${BLUE}📝 Configuration:${NC}"
echo -e "   Port: ${PORT:-8000}"
echo -e "   Log Level: ${LOG_LEVEL}"
echo -e "   Data Directory: ${DATA_DIR}"
echo -e "   Auto Update: ${AUTO_UPDATE_ON_STARTUP}"
echo -e "   Betfair Live Prices: ${ENABLE_LIVE_PRICES:-false}"
echo ""

# Start the API server with logging
echo -e "${BLUE}🚀 Starting API server...${NC}"
echo -e "${BLUE}📝 Logging to: logs/server.log${NC}"
echo -e "${BLUE}   (Also displaying to console)${NC}"
echo ""
echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
echo ""

# Run with both stdout/stderr captured to file AND displayed in terminal
./bin/api 2>&1 | tee logs/server.log
