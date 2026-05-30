FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY go.work go.work.sum ./
COPY services/ services/

# Download dependencies once for all modules
RUN go work sync && go mod download

# Build all services in parallel
RUN cd services/auth-server && go build -o /app/auth-server . && \
    cd /app && cd services/project-manager && go build -o /app/project-manager . && \
    cd /app && cd services/db-proxy && go build -o /app/db-proxy . && \
    cd /app && cd services/storage && go build -o /app/storage . && \
    cd /app && cd services/sql-editor-backend && go build -o /app/sql-editor . && \
    cd /app && cd services/edge-functions && go build -o /app/edge-functions . && \
    cd /app && cd services/realtime-go && go build -o /app/realtime .

FROM alpine:3.19
RUN apk add --no-cache supervisor nginx curl postgresql-client python3 py3-pip
COPY --from=builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /app/realtime /usr/local/bin/
COPY services/ai-assistant /app/ai-assistant
RUN pip3 install --break-system-packages -r /app/ai-assistant/requirements.txt
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY supervisord.conf /etc/supervisor/supervisord.conf
EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
