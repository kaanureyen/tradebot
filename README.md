# tradebot

This tradebot repo contains of source of 2 services: `aggregator`, `fetcher`.
Its docker compose launches the following containers also:
- redis & mongodb in addition to these services to communicate & store & load.
- cadvisor & prometheus & grafana to collect, store and plot service metrics and container resource consumption data. See the section `Monitoring` down the page.

- `fetcher` fetches the price data online, and sends it to `aggregator` via redis.
- `aggregator` listens to the `fetcher`. Buckets the price data to configured time resolution and calculates stats of the price. Calculates SMAs & generates buy-sell signals. Stores them in a mongo database.

## Build & Run Everything

To build everything and run all services:

```bash
docker compose up --build
```

This is recommended over running locally since redis, mongo is readily avaliable and the monitoring works here.

## Run Locally

Warning: Monitoring won't work by default when running locally.

To run the services locally, you can use the following commands:

```bash
go run ./cmd/fetcher
```
```bash
go run ./cmd/aggregator
```

Note: You will need to have Redis & MongoDB instances running on your localhost. You can use the ones on docker though, application detects on runtime whether it is on docker or not and connects to the proper address. You can use the following commands:

```bash
docker compose up --build 'redis'
```
```bash
docker compose up --build 'mongodb'
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
