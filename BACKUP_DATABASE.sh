#!/bin/bash
#
# Database Backup Script for GiddyUp Racing Database
# Creates timestamped backup of entire horse_db database
#

BACKUP_DIR="/home/smonaghan/rpscrape"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/db_backup_$TIMESTAMP.sql"

echo "╔══════════════════════════════════════════════════════════════════════╗"
echo "║                                                                      ║"
echo "║              🗄️  Database Backup Utility 🗄️                         ║"
echo "║                                                                      ║"
echo "╚══════════════════════════════════════════════════════════════════════╝"
echo ""

# Create backup directory if it doesn't exist
mkdir -p "$BACKUP_DIR"

echo "📅 Timestamp: $(date)"
echo "📂 Backup directory: $BACKUP_DIR"
echo "📄 Backup file: db_backup_$TIMESTAMP.sql"
echo ""

# Get database stats before backup
echo "📊 Database Statistics:"
docker exec horse_racing psql -U postgres -d horse_db -t -c "
SELECT 
    (SELECT COUNT(*) FROM racing.races) as total_races,
    (SELECT COUNT(*) FROM racing.runners) as total_runners,
    (SELECT MIN(race_date) FROM racing.races) as earliest_date,
    (SELECT MAX(race_date) FROM racing.races) as latest_date;
" 2>&1 | sed 's/^/   /'

echo ""
echo "💾 Starting backup..."
echo ""

# Perform the backup
docker exec horse_racing pg_dump -U postgres -d horse_db > "$BACKUP_FILE" 2>&1

if [ $? -eq 0 ]; then
    # Get file size
    SIZE=$(du -h "$BACKUP_FILE" | cut -f1)
    
    echo "✅ Backup completed successfully!"
    echo ""
    echo "📄 File: $BACKUP_FILE"
    echo "💾 Size: $SIZE"
    echo ""
    echo "To restore this backup:"
    echo "  docker exec -i horse_racing psql -U postgres -d horse_db < $BACKUP_FILE"
    echo ""
    
    # Keep only last 5 backups
    echo "🧹 Cleanup: Keeping only last 5 backups..."
    cd "$BACKUP_DIR"
    ls -t db_backup_*.sql | tail -n +6 | xargs rm -f 2>/dev/null
    
    echo ""
    echo "📁 Available backups:"
    ls -lh "$BACKUP_DIR"/db_backup_*.sql 2>/dev/null | tail -5 | awk '{print "   " $9 " (" $5 ")"}'
    
else
    echo "❌ Backup failed!"
    exit 1
fi

echo ""
echo "✅ Done!"

