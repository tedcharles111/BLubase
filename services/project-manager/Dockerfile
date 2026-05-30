FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init github.com/blubase/project-manager \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/golang-jwt/jwt/v5@v5.2.1 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go get github.com/minio/minio-go/v7@v7.0.61 \
 && go mod tidy \
 && go build -o project-manager .
FROM alpine:3.19
COPY --from=builder /app/project-manager .
EXPOSE 3002
CMD ["./project-manager"]
