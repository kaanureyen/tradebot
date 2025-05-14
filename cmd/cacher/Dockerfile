FROM golang:1.24.3-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o cacher ./cmd/cacher

FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/cacher .
CMD ["./cacher"]
