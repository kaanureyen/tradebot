FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o fetcher ./cmd/fetcher

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/fetcher .
CMD ["./fetcher"]
