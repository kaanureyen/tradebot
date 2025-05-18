# tradebot

This tradebot repo contains of source of 3 services: `cacher`, `fetcher`, `signal_gen`.
Its docker compose launches the following containers also:
- redis & mongodb in addition to these services to communicate & store & load.
- cadvisor & prometheus & grafana to collect, store and plot resource consumption data. See the configuration down the page.

- `fetcher` fetches the price data online, and sends it to `cacher` via redis.
- `cacher` listens to the `fetcher`. Aggregates it to various time resolutions. Stores aggregate data via mongodb. Also sends the most recent aggregates to `signal_gen`  via redis.
- `signal_gen` loads the needed aggregates from mongodb, and listens to redis to see the most recent aggregates from `cacher`

## Build & Run Everything

To build everything and run all services:

```bash
docker compose up --build
```

## Run Locally

To run the services locally, you can use the following commands:

```bash
go run ./cmd/fetcher
```
```bash
go run ./cmd/cacher
```
```bash
go run ./cmd/signal_gen
```

Note: You will need to have Redis & MongoDB instances running on your localhost. You can use the ones on docker though, with the following commands:

```bash
docker compose up --build 'redis'
```
```bash
docker compose up --build 'mongodb'
```

## See it in action

You can see the messages in redis by connecting to the redis with:

```bash
redis-cli
```

and subscribing to a channel:
```bash
subscribe binance:trade:btcusdt
```

The channel descriptions are below:
`binance:trade:btcusdt` Trade data from Binance via `fetcher`.


## Test

Unit tests:
```bash
go test ./...
```

Unit & Integration tests:
```bash
go test -tags=integration ./...
```

## Monitoring

Dashboard links are:
- cAdvisor: http://localhost:8080
- Prometheus: http://localhost:9090
- Grafana: http://localhost:3000 (default login: admin/admin)

cAdvisor collects performance data from all containers.

Prometheus scrapes cAdvisor and provided `/metrics` endpoints from the services, and stores the data.

Grafana gets data from Prometheus.

Default dashboards are configured on Grafana.