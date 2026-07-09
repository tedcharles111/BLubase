#!/bin/sh
echo "Waiting for PostgreSQL to start..."
until pg_isready -U postgres; do sleep 1; done

# Check if the platform_users table exists
if ! psql -U blubase -d blubase_control -c "SELECT 1 FROM platform_users LIMIT 1" >/dev/null 2>&1; then
    echo "Empty database detected. Restoring from seed.sql..."
    psql -U blubase -d blubase_control -f /app/seed.sql
    echo "Restore complete."
else
    echo "Database already contains data. Skipping restore."
fi
