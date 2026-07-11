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
ENV MISTRAL_API_KEY=bnXRKcksJSDzVBl6lWYM6LVeL7XKFZ0e
RUN apk add --no-cache supervisor nginx curl redis python3 py3-pip bash
COPY --from=go-builder /app/auth-server /app/project-manager /app/db-proxy /app/storage /app/sql-editor /app/edge-functions /usr/local/bin/
COPY services/ai-assistant /app/ai-assistant
RUN pip3 install --break-system-packages -r /app/ai-assistant/requirements.txt
COPY nginx/default.conf /etc/nginx/http.d/default.conf
COPY supervisord.conf /etc/supervisor/supervisord.conf
EXPOSE 10000
CMD ["supervisord", "-c", "/etc/supervisor/supervisord.conf"]
