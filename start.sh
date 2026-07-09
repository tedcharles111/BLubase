#!/bin/sh
echo "Starting PostgreSQL..."
su postgres -c "pg_ctl -D /var/lib/postgresql/data -o '-c listen_addresses=*' start"

# Wait until PostgreSQL is ready (timeout after 30 seconds)
for i in $(seq 1 30); do
    if su postgres -c "pg_isready -q"; then
        echo "PostgreSQL is ready."
        break
    fi
    echo "Waiting for PostgreSQL... ($i)"
    sleep 1
done

# Run the restore script
echo "Checking database state..."
/app/restore-db.sh

# Now launch supervisord to start all other services
exec supervisord -c /etc/supervisor/supervisord.conf
