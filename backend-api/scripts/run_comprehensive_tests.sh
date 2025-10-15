#!/bin/bash

# GiddyUp API Comprehensive End-to-End Test Runner

echo "🧪 GiddyUp API Comprehensive Test Suite"
echo "=========================================="
echo ""
echo "Test Fixtures:"
echo "  HORSE_ID:   9643 (Captain Scooby - 195 runs)"
echo "  TRAINER_ID: 666"
echo "  JOCKEY_ID:  1548"
echo "  COURSE_ID:  82 (Aintree)"
echo "  RACE_ID:    339 (12 runners)"
echo "  DATE1:      2024-01-01"
echo "  DATE2:      2024-01-02"
echo ""

# Check if server is running
echo "Checking if API server is running..."
if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
    echo "❌ API server is not running!"
    echo ""
    echo "Starting server..."
    cd /home/smonaghan/GiddyUp/backend-api
    ./bin/api > /tmp/api-test.log 2>&1 &
    API_PID=$!
    sleep 3
    
    if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
        echo "❌ Failed to start server!"
        echo "Check logs: cat /tmp/api-test.log"
        exit 1
    fi
    
    echo "✅ Server started (PID: $API_PID)"
    STARTED_SERVER=1
else
    echo "✅ API server is already running"
    STARTED_SERVER=0
fi

echo ""

# Run the comprehensive tests
echo "Running comprehensive test suite..."
echo "===================================="
echo ""

cd /home/smonaghan/GiddyUp/backend-api

# Run tests with verbose output
go test -v ./tests/comprehensive_test.go -timeout 10m 2>&1 | tee /tmp/test-results.log

# Capture exit code
TEST_EXIT_CODE=${PIPESTATUS[0]}

echo ""
echo "===================================="
echo ""

# Count test results
PASSED=$(grep "PASS:" /tmp/test-results.log | wc -l)
FAILED=$(grep "FAIL:" /tmp/test-results.log | wc -l)
SKIPPED=$(grep "SKIP:" /tmp/test-results.log | wc -l)

echo "Test Results Summary:"
echo "  ✅ Passed:  $PASSED"
echo "  ❌ Failed:  $FAILED"
echo "  ⊘  Skipped: $SKIPPED"
echo ""

# Clean up if we started the server
if [ "$STARTED_SERVER" = "1" ]; then
    echo "Stopping server (PID: $API_PID)..."
    kill $API_PID 2>/dev/null
fi

# Exit with test result code
if [ $TEST_EXIT_CODE -eq 0 ]; then
    echo "✅ ALL TESTS PASSED!"
    exit 0
else
    echo "❌ SOME TESTS FAILED!"
    echo "Full results: cat /tmp/test-results.log"
    exit 1
fi

