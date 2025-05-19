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

![Dashboard screenshot showing the cAdvisor plots on Grafana](https://github.com/kaanureyen/tradebot/blob/main/doc/cadvisor.png?raw=true)

Default dashboards are configured on Grafana, to show the following stats:
- trade info rate
- trade info error rate
- trade event processing delay (0.50, 0.95, 0.99 percentiles)
- trade info age (compared to local clock) (0.50, 0.95, 0.99 percentiles)
- aggregation information delay (compared to local clock) (0.50, 0.95, 0.99 percentiles)
- BTCUSDT Price, SMA50, SMA200
- Buy - Sell order rate

![Dashboard screenshot showing the mentioned plots](https://github.com/kaanureyen/tradebot/blob/main/doc/dashboard.png?raw=true)

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

Also you can use MongoDB Compass to connect to the database to see in the `tradebot` database, the following timeseries collections:
- `price_stats` stats about the price-buckets (min-max, first-last)
- `price_stats_sma` SMA50 and SMA200 data
- `price_stats_sma_trade` Trade signals (BUY - SELL) based on SMA50 and SMA200. Also has the price at the decision.

![MongoDB tradebot database price_stats_sma_trade collection screenshot showing a BUY operation](https://github.com/kaanureyen/tradebot/blob/main/doc/price_stats_sma_trade.png?raw=true)

## Test

Unit tests:
```bash
go test ./...
```

Unit & Integration tests:
```bash
go test -tags=integration ./...
```
