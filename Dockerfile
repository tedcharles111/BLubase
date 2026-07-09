FROM golang:1.24-alpine AS go-builder
WORKDIR /app
COPY services/ services/
RUN cd services/auth-server && rm -f go.mod go.sum \
 && go mod init auth && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/go-chi/cors@v1.2.1 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/redis/go-redis/v9@v9.5.1 \
 && go get golang.org/x/crypto@v0.17.0 \
 && go get golang.org/x/oauth2@v0.20.0 \
 && go mod tidy && go build -o /app/auth-server .
RUN cd services/project-manager && rm -f go.mod go.sum \
 && go mod init projects && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get golang.org/x/crypto@v0.17.0 \
 && go mod tidy && go build -o /app/project-manager .
RUN cd services/db-proxy && rm -f go.mod go.sum \
 && go mod init proxy && go mod tidy && go build -o /app/db-proxy .
RUN cd services/storage && rm -f go.mod go.sum \
 && go mod init storage && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/storage .
RUN cd services/sql-editor-backend && rm -f go.mod go.sum \
 && go mod init sql-editor && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/sql-editor .
RUN cd services/edge-functions && rm -f go.mod go.sum \
 && go mod init edge && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go mod tidy && go build -o /app/edge-functions .

FROM alpine:3.19
RUN apk add --no-cache supervisor nginx curl postgresql postgresql-contrib redis python3 py3-pip

# ---------- PostgreSQL minimal init ----------
RUN mkdir -p /run/postgresql && chown postgres:postgres /run/postgresql
USER postgres
RUN initdb -D /var/lib/postgresql/data --encoding=UTF8 --lc-collate=C --lc-ctype=C
RUN echo "max_connections = 5" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "log_statement = 'none'" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "fsync = off" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "synchronous_commit = off" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "full_page_writes = off" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "shared_buffers = 128kB" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "max_wal_size = 64MB" >> /var/lib/postgresql/data/postgresql.conf && \
    echo "min_wal_size = 32MB" >> /var/lib/postgresql/data/postgresql.conf

USER root
# Copy the init SQL file and run it while postgres is temporarily running
COPY services/postgres/init-minimal.sql /tmp/init-minimal.sql
RUN su postgres -c "pg_ctl -D /var/lib/postgresql/data -o '-c listen_addresses=*' start" && \
    sleep 2 && \
    su postgres -c "psql -U postgres -f /tmp/init-minimal.sql" && \
    su postgres -c "pg_ctl -D /var/lib/postgresql/data stop"

COPY --from=go-builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /usr/local/bin/
COPY services/ai-assistant /app/ai-assistant
RUN pip3 install --break-system-packages -r /app/ai-assistant/requirements.txt
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY supervisord.conf /etc/supervisor/supervisord.conf

# ---------- Immortal database restore ----------
COPY seed.sql /app/seed.sql
COPY restore-db.sh /app/restore-db.sh
RUN chmod +x /app/restore-db.sh

EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
