FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY services/ services/

# Auth
RUN cd services/auth-server && rm -f go.mod go.sum \
 && go mod init auth && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/go-chi/cors@v1.2.1 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/redis/go-redis/v9@v9.5.1 \
 && go get golang.org/x/crypto@v0.17.0 \
 && go get golang.org/x/oauth2@v0.20.0 \
 && go mod tidy && go build -o /app/auth-server .

# Projects
RUN cd services/project-manager && rm -f go.mod go.sum \
 && go mod init projects && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/project-manager .

# DB Proxy
RUN cd services/db-proxy && rm -f go.mod go.sum \
 && go mod init proxy && go mod tidy && go build -o /app/db-proxy .

# Storage
RUN cd services/storage && rm -f go.mod go.sum \
 && go mod init storage && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/storage .

# SQL Editor
RUN cd services/sql-editor-backend && rm -f go.mod go.sum \
 && go mod init sql-editor && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/sql-editor .

# Edge Functions
RUN cd services/edge-functions && rm -f go.mod go.sum \
 && go mod init edge && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/edge-functions .

FROM alpine:3.19
RUN apk add --no-cache supervisor nginx curl postgresql-client python3 py3-pip redis
COPY --from=builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /usr/local/bin/
COPY services/ai-assistant /app/ai-assistant
RUN pip3 install --break-system-packages -r /app/ai-assistant/requirements.txt
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY supervisord.conf /etc/supervisor/supervisord.conf
EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
# force rebuild
# force oauth reload
# force oauth redeploy
# force rebuild Sun Jun 28 00:39:20 UTC 2026
# force dns refresh Sun Jun 28 21:40:26 UTC 2026
