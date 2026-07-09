#!/bin/sh
# This script is called after PostgreSQL is confirmed ready by start.sh
if ! su postgres -c "psql -U blubase -d blubase_control -c 'SELECT 1 FROM platform_users LIMIT 1'" >/dev/null 2>&1; then
    echo "Empty database, restoring from seed.sql..."
    su postgres -c "psql -U blubase -d blubase_control -f /app/seed.sql"
    echo "Restore complete."
else
    echo "Database already contains data. Skipping restore."
fi
