FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init storage \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/minio/minio-go/v7@v7.0.61 \
 && go mod tidy \
 && go build -o storage .
FROM alpine:3.19
COPY --from=builder /app/storage .
EXPOSE 3004
CMD ["./storage"]
