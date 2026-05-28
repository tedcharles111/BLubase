FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init github.com/blubase/auth-server \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/go-chi/cors@v1.2.1 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/gorilla/sessions@v1.2.2 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/redis/go-redis/v9@v9.5.1 \
 && go get golang.org/x/oauth2@v0.20.0 \
 && go mod tidy \
 && go build -o auth-server .
FROM alpine:3.19
COPY --from=builder /app/auth-server .
EXPOSE 3001
CMD ["./auth-server"]
