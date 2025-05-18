FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o aggregator ./cmd/aggregator

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/aggregator .
CMD ["./aggregator"]
