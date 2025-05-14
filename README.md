# tradebot

This tradebot repo contains of source of 3 services: `cacher`, `fetcher`, `signal_gen`.
Its docker compose launches redis & mongodb in addition to these services to communicate & store & load.

- `fetcher` fetches the price data online, and sends it to `cacher` via redis.
- `cacher` listens to the `fetcher`. Aggregates it to various time resolutions. Sends the aggregates to `signal_gen` via redis. Stores/loads aggregate data via mongodb.
- `signal_gen` listens to the aggregates from `cacher`. Processes the aggregates to generate buy or sell signals. Sends to a redis channel.

## Build & Run Everything

To build everything and run all services:

```bash
docker compose up --build
```

## Build Locally

To build 3 services and put the executables under `./bin`:
```bash
make build-all
```

To build the docker containers for these 3 services:
```bash
make docker-all
```

## Test

Unit tests:
```bash
go test ./...
```

Unit & Integration tests:
```bash
go test -tags=integration ./...
```