#!/bin/bash
# GiddyUp API Service Control Script
# Usage: ./api_service.sh {start|stop|restart|status}

set -e

# Configuration
API_DIR="/home/smonaghan/GiddyUp/backend-api"
API_BIN="$API_DIR/bin/api"
PID_FILE="$API_DIR/logs/api.pid"
LOG_FILE="$API_DIR/logs/api_service.log"
ENV_FILE="/home/smonaghan/GiddyUp/settings.env"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Functions
get_pid() {
    if [ -f "$PID_FILE" ]; then
        cat "$PID_FILE"
    fi
}

is_running() {
    local pid=$(get_pid)
    if [ -n "$pid" ] && kill -0 "$pid" 2>/dev/null; then
        return 0
    else
        return 1
    fi
}

start_api() {
    if is_running; then
        local pid=$(get_pid)
        echo -e "${YELLOW}API is already running (PID: $pid)${NC}"
        return 0
    fi
    
    echo -e "${GREEN}Starting GiddyUp API...${NC}"
    
    # Create logs directory if it doesn't exist
    mkdir -p "$API_DIR/logs"
    
    # Source environment variables
    if [ -f "$ENV_FILE" ]; then
        set -a
        source "$ENV_FILE"
        set +a
        echo "✓ Loaded environment from $ENV_FILE"
    else
        echo -e "${YELLOW}Warning: $ENV_FILE not found${NC}"
    fi
    
    # Start API in background
    cd "$API_DIR"
    nohup "$API_BIN" > "$LOG_FILE" 2>&1 &
    local pid=$!
    
    # Save PID
    echo $pid > "$PID_FILE"
    
    # Wait a moment and verify it started
    sleep 3
    
    if is_running; then
        echo -e "${GREEN}✅ API started successfully (PID: $pid)${NC}"
        echo "   Log file: $LOG_FILE"
        echo "   Health check: http://localhost:8000/health"
        echo ""
        echo "Recent log output:"
        tail -20 "$LOG_FILE"
        return 0
    else
        echo -e "${RED}❌ API failed to start${NC}"
        echo "Check logs: $LOG_FILE"
        tail -30 "$LOG_FILE"
        return 1
    fi
}

stop_api() {
    if ! is_running; then
        echo -e "${YELLOW}API is not running${NC}"
        [ -f "$PID_FILE" ] && rm "$PID_FILE"
        return 0
    fi
    
    local pid=$(get_pid)
    echo -e "${YELLOW}Stopping API (PID: $pid)...${NC}"
    
    # Try graceful shutdown first
    kill -TERM "$pid" 2>/dev/null || true
    
    # Wait up to 10 seconds for graceful shutdown
    for i in {1..10}; do
        if ! kill -0 "$pid" 2>/dev/null; then
            echo -e "${GREEN}✅ API stopped gracefully${NC}"
            rm "$PID_FILE"
            return 0
        fi
        sleep 1
    done
    
    # Force kill if still running
    echo -e "${YELLOW}Forcing shutdown...${NC}"
    kill -9 "$pid" 2>/dev/null || true
    sleep 1
    
    if ! kill -0 "$pid" 2>/dev/null; then
        echo -e "${GREEN}✅ API stopped (forced)${NC}"
        rm "$PID_FILE"
        return 0
    else
        echo -e "${RED}❌ Failed to stop API${NC}"
        return 1
    fi
}

restart_api() {
    echo -e "${YELLOW}Restarting API...${NC}"
    stop_api
    sleep 2
    start_api
}

status_api() {
    if is_running; then
        local pid=$(get_pid)
        echo -e "${GREEN}✅ API is running${NC}"
        echo "   PID: $pid"
        echo "   Port: 8000"
        
        # Check if port is actually listening
        if command -v lsof &> /dev/null; then
            if lsof -i :8000 -sTCP:LISTEN -t >/dev/null 2>&1; then
                echo "   Status: ✅ Listening on port 8000"
            else
                echo -e "   Status: ${YELLOW}⚠️  Process running but not listening${NC}"
            fi
        fi
        
        # Show uptime
        if [ -f "$PID_FILE" ]; then
            local start_time=$(stat -c %Y "$PID_FILE" 2>/dev/null || stat -f %m "$PID_FILE" 2>/dev/null)
            if [ -n "$start_time" ]; then
                local now=$(date +%s)
                local uptime=$((now - start_time))
                local hours=$((uptime / 3600))
                local minutes=$(((uptime % 3600) / 60))
                echo "   Uptime: ${hours}h ${minutes}m"
            fi
        fi
        
        # Show recent logs
        echo ""
        echo "Recent log entries:"
        tail -10 "$LOG_FILE" 2>/dev/null | sed 's/^/   /'
        
        return 0
    else
        echo -e "${RED}❌ API is not running${NC}"
        
        # Show last log entries if available
        if [ -f "$LOG_FILE" ]; then
            echo ""
            echo "Last log entries:"
            tail -10 "$LOG_FILE" | sed 's/^/   /'
        fi
        
        return 1
    fi
}

# Main script
case "${1:-}" in
    start)
        start_api
        ;;
    stop)
        stop_api
        ;;
    restart)
        restart_api
        ;;
    status)
        status_api
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        echo ""
        echo "Commands:"
        echo "  start   - Start the API server"
        echo "  stop    - Stop the API server"
        echo "  restart - Restart the API server"
        echo "  status  - Check API server status"
        echo ""
        echo "Example: ./api_service.sh start"
        exit 1
        ;;
esac

