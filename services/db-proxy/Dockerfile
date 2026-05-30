FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init db-proxy \
 && go mod tidy \
 && go build -o db-proxy .
FROM alpine:3.19
COPY --from=builder /app/db-proxy .
EXPOSE 6543
CMD ["./db-proxy"]
