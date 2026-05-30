FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN rm -f go.mod go.sum \
 && go mod init edge-functions \
 && go get github.com/go-chi/chi/v5@v5.0.11 \
 && go mod tidy \
 && go build -o edge-functions .
FROM alpine:3.19
RUN apk add --no-cache deno
COPY --from=builder /app/edge-functions .
EXPOSE 3005
CMD ["./edge-functions"]
