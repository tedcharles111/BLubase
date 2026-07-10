#!/bin/sh
echo "Starting PostgreSQL..."
su postgres -c "pg_ctl -D /var/lib/postgresql/data -o '-c listen_addresses=*' start"

# Wait for PostgreSQL to be ready
for i in $(seq 1 30); do
    if su postgres -c "pg_isready -q"; then
        echo "PostgreSQL is ready."
        break
    fi
    echo "Waiting for PostgreSQL... ($i)"
    sleep 1
done

# Create blubase user (if missing)
if ! su postgres -c "psql -t -c '\du' | cut -d \| -f 1 | grep -qw blubase"; then
    echo "Creating blubase user..."
    su postgres -c "createuser blubase -s"
fi

# Create blubase_control database (if missing)
if ! su postgres -c "psql -lqt | cut -d \| -f 1 | grep -qw blubase_control"; then
    echo "Creating blubase_control database..."
    su postgres -c "createdb blubase_control -O blubase"
fi

# Run the restore script (imports seed.sql if tables are missing)
echo "Checking database state..."
/app/restore-db.sh

# Launch all other services via supervisord
exec supervisord -c /etc/supervisor/supervisord.conf
