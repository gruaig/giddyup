#!/bin/bash
# Quick verification that everything is ready

echo "🔍 GiddyUp Project Verification"
echo "================================"
echo ""

# Check structure
echo "📁 Project Structure:"
if [ -f "README.md" ]; then
    echo "  ✅ Main README.md exists"
else
    echo "  ❌ Main README.md missing"
fi

if [ -f "docs/README.md" ]; then
    echo "  ✅ Documentation index exists"
else
    echo "  ❌ Documentation index missing"
fi

if [ -f "docs/features/AUTO_UPDATE.md" ]; then
    echo "  ✅ Auto-update docs exist"
else
    echo "  ❌ Auto-update docs missing"
fi

if [ -f "backend-api/cmd/README.md" ]; then
    echo "  ✅ CLI tools guide exists"
else
    echo "  ❌ CLI tools guide missing"
fi

echo ""
echo "🔧 Built Tools:"
for tool in api load_master backfill_dates check_missing; do
    if [ -f "backend-api/bin/$tool" ]; then
        size=$(du -h "backend-api/bin/$tool" | cut -f1)
        echo "  ✅ $tool ($size)"
    else
        echo "  ❌ $tool not built"
    fi
done

echo ""
echo "🗄️  Database:"
if docker ps | grep -q horse_racing; then
    echo "  ✅ PostgreSQL container running"
    
    # Check if database has data
    count=$(docker exec horse_racing psql -U postgres -d horse_db -t -c "SELECT COUNT(*) FROM racing.races;" 2>/dev/null | xargs)
    if [ ! -z "$count" ] && [ "$count" -gt 0 ]; then
        echo "  ✅ Database has data ($count races)"
    else
        echo "  ⚠️  Database is empty (run load_master or restore backup)"
    fi
else
    echo "  ⚠️  PostgreSQL container not running"
    echo "     Start with: cd postgres && docker-compose up -d"
fi

echo ""
echo "📊 Statistics:"
docs_count=$(find docs -name "*.md" | wc -l)
echo "  📄 Documentation files: $docs_count"

go_files=$(find backend-api/internal -name "*.go" | wc -l)
echo "  🔧 Go source files: $go_files"

echo ""
echo "✅ Verification Complete!"
echo ""
echo "To start the server with auto-update:"
echo "  cd backend-api"
echo "  AUTO_UPDATE_ON_STARTUP=true ./bin/api"
echo ""

