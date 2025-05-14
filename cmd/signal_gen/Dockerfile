FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o signal_gen ./cmd/signal_gen

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/signal_gen .
CMD ["./signal_gen"]
