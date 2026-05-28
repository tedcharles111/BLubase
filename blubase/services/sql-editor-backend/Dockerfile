FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init github.com/blubase/sql-editor-backend \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go get github.com/jackc/pgx/v5@v5.5.5 \
 && go mod tidy \
 && go build -o sql-editor .
FROM alpine:3.19
COPY --from=builder /app/sql-editor .
EXPOSE 3007
CMD ["./sql-editor"]
