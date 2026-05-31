FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o consumer ./cmd/server

FROM alpine:3.20
WORKDIR /app
COPY --from=builder /app/consumer /app/consumer
CMD ["/app/consumer"]
