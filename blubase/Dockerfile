# Stage 1: Build all Go services
FROM golang:1.24-alpine AS go-builder
WORKDIR /build
COPY services/auth-server ./auth-server
COPY services/project-manager ./project-manager
COPY services/db-proxy ./db-proxy
COPY services/storage ./storage
COPY services/sql-editor-backend ./sql-editor-backend
COPY services/edge-functions ./edge-functions

RUN cd auth-server && rm -f go.mod go.sum && go mod init auth && go get ./... && go mod tidy && go build -o /app/auth-server . && \
    cd /build/project-manager && rm -f go.mod go.sum && go mod init projects && go get ./... && go mod tidy && go build -o /app/project-manager . && \
    cd /build/db-proxy && rm -f go.mod go.sum && go mod init proxy && go get ./... && go mod tidy && go build -o /app/db-proxy . && \
    cd /build/storage && rm -f go.mod go.sum && go mod init storage && go get ./... && go mod tidy && go build -o /app/storage . && \
    cd /build/sql-editor-backend && rm -f go.mod go.sum && go mod init sql-editor && go get ./... && go mod tidy && go build -o /app/sql-editor . && \
    cd /build/edge-functions && rm -f go.mod go.sum && go mod init edge && go get ./... && go mod tidy && go build -o /app/edge-functions .

# Stage 2: Build Elixir realtime service
FROM elixir:1.15-alpine AS elixir-builder
RUN mix local.hex --force && mix local.rebar --force
WORKDIR /app
COPY services/realtime .
RUN mix deps.get && mix compile

# Stage 3: Final image with supervisord
FROM alpine:3.19
RUN apk add --no-cache supervisor curl postgresql-client redis
COPY --from=go-builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /usr/local/bin/
COPY --from=elixir-builder /app/_build /app/_build
COPY --from=elixir-builder /app/deps /app/deps
COPY --from=elixir-builder /app/mix.exs /app/
COPY services/ai-assistant /app/ai-assistant
RUN apk add --no-cache python3 py3-pip && pip3 install -r /app/ai-assistant/requirements.txt

# Copy supervisor config
COPY supervisord.conf /etc/supervisor/supervisord.conf

EXPOSE 3001 3002 3004 3005 3006 3007 4000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
