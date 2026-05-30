FROM golang:1.24-alpine AS go-builder
WORKDIR /build
COPY services/auth-server ./auth-server
COPY services/project-manager ./project-manager
COPY services/db-proxy ./db-proxy
COPY services/storage ./storage
COPY services/sql-editor-backend ./sql-editor-backend
COPY services/edge-functions ./edge-functions

RUN cd /build/auth-server && rm -f go.mod go.sum \
 && go mod init auth \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/go-chi/cors@v1.2.1 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/redis/go-redis/v9@v9.5.1 \
 && go get golang.org/x/crypto@v0.17.0 \
 && go mod tidy && go build -o /app/auth-server .

RUN cd /build/project-manager && rm -f go.mod go.sum \
 && go mod init projects \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/minio/minio-go/v7@v7.0.61 \
 && go mod tidy && go build -o /app/project-manager .

RUN cd /build/db-proxy && rm -f go.mod go.sum \
 && go mod init proxy && go mod tidy && go build -o /app/db-proxy .

RUN cd /build/storage && rm -f go.mod go.sum \
 && go mod init storage \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/minio/minio-go/v7@v7.0.61 \
 && go mod tidy && go build -o /app/storage .

RUN cd /build/sql-editor-backend && rm -f go.mod go.sum \
 && go mod init sql-editor \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy && go build -o /app/sql-editor .

RUN cd /build/edge-functions && rm -f go.mod go.sum \
 && go mod init edge \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go mod tidy && go build -o /app/edge-functions .

FROM elixir:1.15-alpine AS elixir-builder
RUN mix local.hex --force && mix local.rebar --force
WORKDIR /app
COPY services/realtime .
RUN rm -f lib/realtime/application.ex
RUN mix deps.get && mix compile --no-warnings-as-errors

FROM alpine:3.19
RUN apk add --no-cache supervisor nginx curl postgresql-client redis python3 py3-pip elixir erlang
COPY --from=go-builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /usr/local/bin/
COPY --from=elixir-builder /app/_build /app/_build
COPY --from=elixir-builder /app/deps /app/deps
COPY --from=elixir-builder /app/mix.exs /app/
COPY services/ai-assistant /app/ai-assistant
RUN pip3 install --break-system-packages -r /app/ai-assistant/requirements.txt
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY supervisord.conf /etc/supervisor/supervisord.conf
EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
