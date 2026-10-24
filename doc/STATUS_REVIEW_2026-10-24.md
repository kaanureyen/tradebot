# tradebot — Detaylı İnceleme ve Durum Raporu

> **Not:** Bu rapor 2026-10-24 tarihinde bir Claude Code bulut oturumunda repo salt-okunur incelenerek üretildi (repo kodu değiştirilmedi). Raporda geçen `/tmp/claude-0/.../scratchpad/...` yolları o oturuma ait geçici dizinlerdir ve artık mevcut değildir; probe/demo script'leri ve loglar repoya dahil edilmedi. Ağ politikası nedeniyle borsa/Polymarket host'larına erişilemediğinden gerçek veriyle yeni sonuç üretilmedi; sayısal sonuçlar git geçmişinden, kod okumasından veya açıkça belirtilen sentetik testlerden gelir.

- **Repo:** `/home/user/tradebot` (module `github.com/kaanureyen/tradebot`, branch `claude/eloquent-mayer-rkjsj5` = `main` ile aynı HEAD `76a020f`)
- **Son commit:** 2025-05-22 "added simulator" (ilk commit 2025-05-14 → toplam ~8 günlük yoğun bir çalışma, sonra durmuş)
- **Boyut:** ~950 satır Go (5 paket), + docker-compose, Prometheus/Grafana config
- **İnceleme tarihi:** 2026-10-24
- **Yöntem:** Repo read-only incelendi, hiçbir dosya değiştirilmedi. Davranışı kanıtlamak için repo `git archive` ile scratch dizinine kopyalandı ve kopyada ek "probe" testleri çalıştırıldı (gerçek `redis-server` ile uçtan uca dahil). Probe dosyaları:
  - `/tmp/claude-0/-home-user/532e68b0-25e1-5e9f-99b8-3a4ca18e1131/scratchpad/tradebot-work/copy/cmd/aggregator/probe_test.go`
  - `/tmp/claude-0/-home-user/532e68b0-25e1-5e9f-99b8-3a4ca18e1131/scratchpad/tradebot-work/copy/cmd/fetcher/probe_test.go`
  - Çıktı logu: `/tmp/claude-0/-home-user/532e68b0-25e1-5e9f-99b8-3a4ca18e1131/scratchpad/tradebot-work/probe.log`
- **Ortam kısıtları:** Docker daemon çalışmıyor, MongoDB yok, Binance ve Polymarket host'ları network policy ile engelli (proxy 403 / connection reset). Bu yüzden canlı Binance akışı ve Mongo yazma yolu çalıştırılamadı. Redis (`redis-server` 7.0.15) mevcuttu ve kullanıldı.

---

## 0. TL;DR

- **Ne yapıyor:** Binance spot **BTCUSDT** trade WebSocket akışını alıp (`fetcher`), Redis pub/sub üzerinden `aggregator`'a iletiyor. `aggregator` fiyatları **15 saniyelik** bucket'lara (first/last/min/max) topluyor, **SMA50/SMA200** hesaplıyor, **golden/death cross** ile BUY/SELL **sinyali** üretiyor ve hepsini MongoDB time-series koleksiyonlarına yazıyor. Prometheus + Grafana + cAdvisor ile izleniyor. `simulator`, kayıtlı sinyalleri 1000 USDT'lik sanal bir cüzdana uygulayan basit bir replay aracı.
- **Gerçek emir yok.** Hiçbir API key, REST client ya da order endpoint'i yok. "Buy-Sell order rate" paneli aslında sinyal sayacı.
- **Durum:** `go build`, `go vet` ve `go test` (unit ve `-tags=integration`) **hepsi geçiyor**, ama **tüm testler `1 == 1` dummy testleri**, yani gerçek test kapsamı sıfır. Default staticcheck temiz.
- **En önemli sorunlar (kanıtlı):** (1) Her restart'ta crossover olmadan **sahte BUY/SELL sinyali** çıkıyor. (2) Uzun kesintiden sonra SMA penceresi **tamamen lineer interpolasyonla uydurulmuş** veriden oluşuyor (1 günlük boşlukta 200 noktanın 199'u sentetik) ve sinyaller bu sahte veriyle üretiliyor. (3) WS bağlantısı sessizce donarsa (half-open TCP) **yeniden bağlanma yok**; `/healthz` yine "OK" diyor. (4) Binance'e erişim yokken fetcher **graceful shutdown yapamıyor**. (5) 4,3 günden uzun boşluklarda interpolasyonda **int64 overflow** oluyor.
- **Polymarket kararı:** Kod seviyesinde düşük/orta yeniden kullanım (~%20–30). Altyapı ve pattern seviyesinde yüksek (~%70: docker-compose monitoring stack, Prometheus metrik pattern'leri, Dockerfile'lar, WS→bus→aggregator→TSDB pipeline fikri). Öneri: tradebot'u dondurup (birkaç S-efor düzeltmesi ve README ile) Polymarket için **yeni bir modül** başlatmak; monitoring stack'i ve pattern'leri taşımak.

---

## 1. Amaç ve Mimari

### 1.1 Servisler

| Servis | Dosyalar | Rol | Portlar (compose) |
|---|---|---|---|
| `fetcher` | `cmd/fetcher/main.go` (144 satır) | Binance WS `btcusdt@trade` akışına bağlanır, her trade'i `{TradeDate, Price}` JSON olarak Redis'e publish eder | health `9000`, metrics `2112` |
| `aggregator` | `cmd/aggregator/main.go`, `PeriodicPriceStats.go`, `tAggregatedTradeInfo.go`, `tSmaBuffer.go` | Redis'e subscribe olur, 15 s bucket'lar oluşturur, SMA50/200 hesaplar, crossover sinyali üretir, Mongo'ya yazar | health `9001`, metrics `2113` |
| `simulator` | `cmd/simulator/main.go` (81 satır) | `price_stats_sma_trade` koleksiyonundaki sinyalleri kronolojik sırayla sanal cüzdana uygular (all-in BUY/SELL), loglar | compose'da **yok**, Dockerfile **yok**, README'de **yok** |
| `shared` | `cmd/shared/*.go` | `ShutdownOrchestrator`, logger + health endpoint (`InitCommon`), Mongo bağlantısı ve koleksiyon oluşturma, sabitler (`RedisChannel`, `SmaShortTerm=50`, `SmaLongTerm=200`), `TradeDatePrice`, `TradeSignal` tipleri | — |
| Altyapı | `docker-compose.yml` | `redis:alpine`, `mongo:latest`, `prom/prometheus`, `grafana/grafana`, `gcr.io/cadvisor/cadvisor:latest` | 6379, 27017, 9090, 3000, 8080 |

### 1.2 İletişim ve veri akışı

```
Binance  wss://stream.binance.com:9443/ws/btcusdt@trade
   │  (binance-connector-go v0.8.0, WsTradeServe)
   ▼
fetcher ──PUBLISH──► Redis pub/sub kanal "binance:trade:btcusdt"
                       payload: {"TradeDate":<ms int64>,"Price":"<string>"}
                          │ SUBSCRIBE (go-redis v9 PubSub.Channel, buffer 100)
                          ▼
aggregator: subscribeRedis → unmarshalTradeDatePrice → calculatePriceStats (15 s bucket)
            → SmaBuffer (ring buffer 200, lineer interpolasyonlu boşluk doldurma)
            → SMA50/SMA200 → crossover → TradeSignal
                          │ InsertOne (senkron)
                          ▼
MongoDB "tradebot": price_stats | price_stats_sma | price_stats_sma_trade (time-series)
                          ▲
simulator ──Find(sort timestamp desc, limit 999999)── price_stats_sma_trade

Prometheus (15 s scrape) ◄── fetcher:2112, aggregator:2113, cadvisor:8080 ──► Grafana (provisioned dashboards)
```

- **Message queue:** Yok. Redis yalnızca **pub/sub** (fire-and-forget) olarak kullanılıyor; Streams yok, kalıcılık yok.
- **DB:** MongoDB (time-series collection'lar). Adresler `/.dockerenv` dosyasına bakılarak runtime'da seçiliyor (`redis:6379` / `localhost:6379`, `mongodb://mongodb:27017` / `mongodb://localhost:27017`), bkz. `cmd/shared/constants.go:20-33`. Auth yok.
- **Lifecycle:** Kendi yazılmış `ShutdownOrchestrator` (`cmd/shared/tShutdownOrchestrator.go`): SIGINT/SIGTERM gelince kayıtlı her `stop` kanalına sinyal gönderir, `done` kanallarını bekler, sonra `Done` kanalına sonsuza kadar değer basar.

### 1.3 Sinyal mantığı (SMA crossover)

`cmd/aggregator/main.go:105-159`:
- Her bucket'ın `LastPrice` değeri `SmaBuffer`'a eklenir (`AddWithLinInterpFill`).
- Buffer ≥200 nokta olunca `SMA50` ve `SMA200` hesaplanır. 15 s bucket'larla bu **SMA50 ≈ 12,5 dakika, SMA200 ≈ 50 dakika** demek; yani çok kısa vadeli, whipsaw'a açık bir crossover.
- `diff = SMA50 − SMA200`; `diff > 0 && lastDiff <= 0` → **BUY**, `diff < 0 && lastDiff >= 0` → **SELL**.
- Sinyal varsa `price_stats_sma_trade` koleksiyonuna yazılır ve `aggregate_info_buy_count` / `aggregate_info_sell_count` sayaçları artırılır. SMA değerleri her bucket'ta `price_stats_sma` koleksiyonuna yazılır.
- Restart'ta `LoadLastNIntoSmaBuffer` son 200 `price_stats` kaydını yükleyerek warm-up süresini kısaltıyor (`main.go:164-185`).

### 1.4 Simulator ne yapıyor?

`cmd/simulator/main.go`: Mongo'dan tüm sinyalleri okur (`limit 999999`), en eskiden başlayarak `BUY` → tüm USDT ile BTC al, `SELL` → tüm BTC'yi sat. Her adımda eski ve yeni cüzdanı loglar.
- **Yok:** fee/slippage, son durumda mark-to-market (son sinyal BUY ise sonuç BTC cinsinden kalır), PnL/drawdown/win-rate metrikleri, zaman aralığı veya parametre seçimi, farklı SMA parametreleriyle yeniden hesaplama (yani gerçek bir backtester değil, sadece **kayıtlı canlı sinyallerin replay'i**).
- **Bug:** İş bitince `main` dönüyor ama deferred `<-shutdownOrchestrator.Done` SIGINT gelene kadar **sonsuza kadar bekliyor** (`simulator/main.go:15-18`). Program kendiliğinden kapanmıyor.
- **Yan etki:** Okuma amaçlı araç olmasına rağmen `MongoTradeCollection` çağırıyor; bu `CreateCollection` + `CreateIndex` yapıyor.

### 1.5 Gerçek emir var mı?

**Hayır.** `binance-connector-go` yalnızca `NewWebsocketStreamClient(false).WsTradeServe` için kullanılıyor. `NewClient`, API key/secret, `NewCreateOrderService`, testnet kullanımı yok. "Order" kelimesi yalnızca Grafana panelinde ve README'de geçiyor ve sinyalleri kastediyor.

### 1.6 Metrikler

| Metrik | Servis | Tip | Anlam |
|---|---|---|---|
| `trades_received_total` | fetcher | Counter | Binance'ten alınan trade sayısı |
| `trades_published_total` | fetcher | Counter | Redis'e publish edilen trade sayısı (**dinleyen olmasa da artar**, publish'in döndürdüğü alıcı sayısı yok sayılıyor) |
| `trade_event_delay_microseconds` | fetcher | Summary (p50/p95/p99) | marshal + publish süresi |
| `trade_info_age_milliseconds` | fetcher | Summary | local saat − Binance trade zamanı |
| `aggregate_info_age_milliseconds` | aggregator | Summary | bucket yayınlandığında local saat − bucket'ın son trade zamanı |
| `aggregate_info_price`, `aggregate_info_sma50`, `aggregate_info_sma200` | aggregator | Gauge | son fiyat ve SMA'lar |
| `aggregate_info_buy_count`, `aggregate_info_sell_count` | aggregator | Counter | sinyal sayıları |

Grafana "services" dashboard'unda 7 panel var (trade rate, delay, "error rate" = received − published, age, Price & SMAs, aggregate delay, Buy-Sell rate). Ayrıca bir cAdvisor dashboard'u (CPU, memory, network, container info) var. **Alerting rule yok**, reconnect veya WS-error sayacı yok, `/healthz` statik "OK" döndürüyor.

---

## 2. Veri İndeksi

### 2.1 Kaynak

- **Exchange:** Binance Spot (global), `wss://stream.binance.com:9443/ws/btcusdt@trade` (connector default'u; sembol `strings.ToLower` ile küçültülüyor).
- **Sembol:** Yalnızca **BTCUSDT**, hardcoded (`fetcher/main.go:74`, `shared/constants.go:10`).
- **REST:** Kullanılmıyor (backfill yok).
- **Alınıp atılan alanlar:** Binance trade event'i `TradeID`, `Quantity`, `IsBuyerMaker`, `BuyerOrderId`, `SellerOrderId`, `Time` (event time) içeriyor, ama fetcher **yalnızca `TradeTime` ve `Price`** gönderiyor (`fetcher/main.go:83`). Sonuç: hacim, VWAP, trade sayısı, taker yönü yok; trade ID ile boşluk tespiti de yapılamıyor.
- Binance bazı bölgelerden (ör. ABD IP'leri, HTTP 451) erişilemez. Bu sandbox'tan da engelli.

### 2.2 Redis

- Kanal: `binance:trade:btcusdt`
- Payload: `{"TradeDate": 1716200000123, "Price": "67123.45000000"}` (Go struct `shared.TradeDatePrice`, alan adı yazım hatalı: Trade**Date**Price)
- Kalıcılık: **Yok** (pub/sub). Aggregator kapalıyken ya da subscribe tamamlanmadan önce publish edilen her şey kaybolur. `redis_data` volume'u pub/sub için anlamsız.

### 2.3 MongoDB şeması (`tradebot` DB)

| Koleksiyon | timeField | metaField | Alanlar | Yazma sıklığı | Index |
|---|---|---|---|---|---|
| `price_stats` | `lasttimestamp` | **yok** | `firsttimestamp`, `lasttimestamp`, `min_price`, `max_price`, `first_price`, `last_price` | her 15 s bucket (trade varsa) | `lasttimestamp: -1` |
| `price_stats_sma` | `timestamp` (= bucket'ın son trade zamanı) | yok | `timestamp`, `sma50`, `sma200` | warm-up sonrası her bucket | `timestamp: -1` |
| `price_stats_sma_trade` | `timestamp` (= **wall-clock `time.Now()`**) | yok | `timestamp`, `signal` ("BUY"/"SELL"), `price`, `sma50`, `sma200` | yalnızca crossover anında | `timestamp: -1` |

- **Retention:** Yok (`expireAfterSeconds` set edilmemiş), sınırsız büyüme. Hacim küçük (~5.760 doc/gün/koleksiyon, yılda ~2,1M).
- **granularity:** set edilmemiş (default `seconds`, 15 s için uygun).
- **metaField yok:** Çoklu sembole geçmek şema değişikliği gerektirir. Seçenekleri değiştirmek mevcut DB'de `CreateCollection` hatası ve `log.Fatal` demek (bkz. §4 M3).
- **Zaman tabanı tutarsız:** SMA dokümanları trade zamanıyla, sinyal dokümanları işlem anındaki wall-clock ile damgalanıyor.

### 2.4 Bucketing

- `PeriodicPriceStats.go:23`: başlangıç noktası `time.Now().Truncate(24*time.Hour)`. Go'da `Truncate` mutlak zamana göre çalıştığı için bucket'lar **UTC 00:00'a hizalı**, `[t, t+15s)` aralıkları.
- Bucket **trade-driven** yayınlanıyor: bir bucket, ancak bir sonraki bucket'a ait ilk trade geldiğinde emit ediliyor. Bu yüzden gecikme 15 s + bir sonraki trade'e kadar geçen süre kadar olabiliyor. Likit BTC için sorun değil; illikit piyasalarda (Polymarket!) bucket'lar süresiz gecikebilir.
- Bucket dokümanında **bucket başlangıç zamanı yok**. Sadece içindeki ilk/son trade zamanları var.
- Geç gelen trade (bucket kapanmış) loglanıp atılıyor (probe ile doğrulandı).
- Shutdown'da yarım bucket **flush edilmiyor**, kayboluyor (probe ile doğrulandı).
- Commit tarihçesi: 1 s → 15 s (`f4e89da`). Daha önce birden fazla çözünürlükte paralel bucketing (`SlicePeriodicStats` + `FanIn`) vardı, `0b2e99e`'de kaldırıldı.

### 2.5 Ne persist ediliyor, ne edilmiyor

| Persist edilen | Persist edilmeyen |
|---|---|
| 15 s OHLC benzeri bucket'lar (O=first, H=max, L=min, C=last) | Ham trade'ler, hacim, trade ID, taker yönü |
| SMA50/200 serisi (interpolasyonlu buffer'dan hesaplanmış) | Interpolasyonla üretilen noktaların kendisi (yalnızca RAM'de), dolayısıyla SMA serisi yeniden üretilemez |
| BUY/SELL sinyalleri + karar anındaki fiyat | Pozisyon, PnL, simulator sonuçları |

---

## 3. Mevcut Durum

### 3.1 Build / Vet / Test

| Komut | Sonuç | Not |
|---|---|---|
| `go build ./...` | ✅ geçti | Go 1.24.7 (go.mod `go 1.24.3`), bağımlılıklar proxy.golang.org'dan indi |
| `go vet ./...` | ✅ temiz | |
| `go test ./...` | ✅ geçti | `aggregator`, `fetcher`: yalnızca `TestDummy` (1==1). `shared`, `simulator`: test dosyası yok |
| `go test -tags=integration ./...` | ✅ geçti | `integration/` paketleri de **dummy**. DB/exchange'e hiç dokunmuyor, o yüzden ortam eksikliğinden **fail eden test yok**, ama test edilen bir şey de yok |
| `staticcheck` (2025.1.1, default) | ✅ temiz | `-checks all` ile yalnızca stil uyarıları (ST1000/ST1003/ST1020) |
| `govulncheck` | ⚠️ çalıştırılamadı | `vuln.go.dev` network policy ile engelli |
| `go list -m -u` | ℹ️ | `mongo-driver v1.17.3` → **v1 deprecated** (v2 mevcut), `go-redis v9.8.0` → v9.23.0, `client_golang v1.22.0` → v1.25.0 |
| Probe testleri (kopyada, `-race`) | Bkz. §4 | Gerçek Redis ile uçtan uca pipeline + shutdown çalıştı. Race detector `ShutdownOrchestrator`'da data race buldu |

CI yok (`.github/` yok), LICENSE yok, `.dockerignore` yok.

### 3.2 Tarihçe ve evrim (yarım kalanları anlamak için)

1. **05-14:** 3 modüllü iskelet: `fetcher`, `cacher`, `signal_gen` + redis + mongo + Makefile.
2. **05-16/17:** `cacher` geliştirildi (çoklu periyot için fan-in, shutdown orchestrator).
3. **05-18:** health endpoint'ler, Docker algılama, Prometheus/Grafana/cAdvisor eklendi, Makefile kaldırıldı.
4. **05-19:** `cacher` → `aggregator` olarak yeniden adlandırıldı, **çoklu periyot kaldırıldı**, SMA buffer eklendi.
5. **05-20:** SMA hesaplama, DB save/load, lineer interpolasyon, Prometheus probe'ları eklendi. **`signal_gen` silindi**: hiç implement edilmemişti (15 satırlık stub), görevi aggregator'a katıldı. Bucket 1 s → 15 s yapıldı. README ve ekran görüntüleri eklendi.
6. **05-22:** `simulator` eklendi. Mongo yardımcıları ve `TradeSignal` `shared`'e taşındı. Proje burada duruyor.

### 3.3 README iddiaları ve kod

| README iddiası | Gerçek |
|---|---|
| "2 services: aggregator, fetcher" | Üçüncü bir binary var (`simulator`), README/compose'da yok |
| "Buy - Sell **order** rate" | Emir değil, sinyal sayacı |
| "trade info error rate" | Yalnızca received−published farkı. WS hataları/reconnect'ler sayılmıyor |
| `go test -tags=integration ./...` "Unit & Integration tests" | İkisi de dummy |
| Runtime'da docker algılama | ✅ doğru (`/.dockerenv`) |
| Lokal çalıştırmada monitoring çalışmaz | ✅ doğru (Prometheus target'ları container adları) |
| Redis kanalı `binance:trade:btcusdt`, 3 Mongo koleksiyonu | ✅ doğru |
| (bahsedilmiyor) | Lokal health portu 8080–8099 aralığında aranıyor (cAdvisor 8080 ile çakışınca fallback var) |
| `.vscode/launch.json` `-tags=debug` | `debug` build tag'li dosya yok |

### 3.4 Yarım kalan / eksik olanlar

- **Testler:** Tamamen placeholder (unit ve integration).
- **Simulator:** compose/Dockerfile/README yok, fee/PnL yok, kendiliğinden kapanmıyor.
- **signal_gen:** Silindi. Sinyal mantığı aggregator'ın `main()` fonksiyonuna gömülü, test edilebilir bir fonksiyon değil.
- **Çoklu çözünürlük / çoklu sembol:** Kaldırılmış ya da hiç yapılmamış. Her şey hardcoded.
- **Execution:** Gerçek ya da testnet emir gönderimi yok. Risk yönetimi ve pozisyon takibi yok.
- **Dayanıklılık:** Backfill yok, durable kuyruk yok, stall watchdog yok, anlamlı health yok, alert yok.
- **Konfigürasyon:** Sembol, periyot, SMA uzunlukları, portlar kodda sabit. Env/flag desteği yalnızca `HEALTH_PORT` için var.

---

## 4. Bug'lar ve Riskler

Önem: **Y**=Yüksek, **O**=Orta, **D**=Düşük. "Kanıt" sütunundaki probe sonuçları scratch kopyada çalıştırıldı.

| # | Önem | Konu | Yer | Açıklama | Kanıt |
|---|---|---|---|---|---|
| H1 | Y | **Restart'ta sahte sinyal** | `aggregator/main.go:105,130-138` | `lastDiff := 0.0` ile başlıyor. SMA ilk kez hazır olduğunda `diff>0` ise BUY, `diff<0` ise SELL üretiliyor, gerçek bir crossover olmasa bile. DB'den reload yapıldığı için **her restart'ta** (ve `restart: always` ile her crash'te) bir sahte sinyal çıkıyor. Simulator bunları gerçek işlem gibi uyguluyor. | Probe: sürekli yükselen piyasa (crossover hiç yok) → `[BUY@199]` |
| H2 | Y | **Uzun boşlukta uydurulmuş SMA penceresi** | `tSmaBuffer.go:46-66`, `main.go:164-185` | Kesintiden sonra ilk bucket ile son DB kaydı arası lineer interpolasyonla dolduruluyor: 1 gün = 5.760 sentetik nokta. 200'lük pencere tamamen sentetik oluyor ve crossover sinyali **uydurma veriyle** üretiliyor. Interpolasyon kesintiden *sonraki* fiyatı kullanıyor (bir tür look-ahead) ve sentetik noktalar persist edilmediği için SMA serisi yeniden üretilemiyor. | Probe: 1 günlük boşluk sonrası pencerede **1/200 gerçek nokta**, `sma50=199.57`, `sma200=198.27` (100→200 arası düz bir doğru) |
| H3 | Y | **Sessiz WS stall** | `fetcher/main.go:108-143` + connector `websocket.go` | Read deadline yok. Connector'da `WebsocketKeepalive=false` (default) ve açılsa bile `keepAlive` timeout'ta bağlantıyı **kapatmıyor**, sadece goroutine'den çıkıyor. Half-open TCP'de `ReadMessage` sonsuza kadar bloklar, reconnect olmaz, veri durur, `/healthz` "OK" der, alert yok. (Binance'in 24 saatlik normal disconnect'i ise doğru şekilde reconnect ediliyor.) | Kod incelemesi (connector v0.8.0 kaynağı) |
| H4 | Y | **Veri kaybı yolları** | birden çok | (a) Redis pub/sub dayanıksız: aggregator kapalıyken her trade kaybolur ve `trades_published_total` yine artar. (b) Reconnect sırasındaki 5 s+ boşluk REST ile backfill edilmiyor, sonra interpolasyonla "dolduruluyor". (c) Shutdown'da son bucket flush edilmiyor. (d) Mongo yavaş ya da kapalıysa (`InsertOne` server selection timeout 30 s × 3 insert) pipeline durur, go-redis kanal buffer'ı (100) dolar ve 1 dakika sonra mesajlar **düşürülür**. | (c) probe ile doğrulandı. (d) go-redis `pubsub.go:583-585,670` |
| M1 | O | **Bağlantı yokken graceful shutdown çalışmıyor** | `fetcher/main.go:113-118,123-126` | Dial-retry döngüsü ve `doneCh` sonrası `time.Sleep` `stop` kanalını kontrol etmiyor. Binance erişilemezken SIGTERM işlenmiyor, Docker 10 s sonra SIGKILL atıyor. | Probe: Binance engelliyken shutdown **12 s'de tamamlanmadı** |
| M2 | O | **Interpolasyonda int64 overflow** | `tSmaBuffer.go:60` | `time.Duration(i) * deltaDate`, ~4,3 günden uzun boşluklarda int64'ü taşırıyor ve interpolasyon timestamp'leri çöp oluyor. | Probe: 7 günlük boşlukta beklenen ~`2025-01-07 23:59:45`, elde edilen `2025-01-02 16:54:36` |
| M3 | O | **`CreateCollection` → `log.Fatal`** | `shared/functions.go:84-87,116-119,148-151` | Driver dokümanına göre koleksiyon varsa `CommandError` döner. Yeni MongoDB sürümleri aynı opsiyonlarla `create` çağrısını muhtemelen no-op kabul ediyor (yazarın restart'ları çalışmış görünüyor), ama bu **burada doğrulanamadı** (Mongo yok). Opsiyonlar değişirse (ör. metaField eklemek) kesin crash-loop olur. `mongo:latest` pinlenmemiş (major upgrade + FCV riski). | Driver kaynağı `database.go:529-531` |
| M4 | O | **ShutdownOrchestrator race ve leak'ler** | `shared/tShutdownOrchestrator.go:30-62` | `send`/`recv` slice'ları lock'suz. Startup sırasında sinyal gelirse data race olur. `Done`'a sonsuza kadar yazan goroutine var. Simulator iş bitince kapanmıyor (bkz. §1.4). | `go test -race` 3 adet DATA RACE raporladı |
| M5 | O | **Zaman semantiği** | `PeriodicPriceStats.go:54-62`, `main.go:123,152` | Bucket emit'i trade-driven. Bucket kimliği bucket başlangıcı değil, son trade zamanı. Sinyal zamanı wall-clock, SMA zamanı trade time. Interpolasyondaki `math.Round(delta/period)` trade jitter'ına duyarlı. | Kod incelemesi + bucketing probe'u |
| M6 | O | **`subscribeRedis` içindeki etkisiz `break`** | `PeriodicPriceStats.go:99-115` | `if !ok { break }` yalnızca `select`'ten çıkıyor. Kanal kapanırsa CPU'yu döndüren bir busy-loop oluşur ve dıştaki re-subscribe döngüsüne asla ulaşılmaz (ayrıca eski `PubSub` kapatılmıyor). Pratikte go-redis reconnect'i kendisi yapıyor. | Probe: Redis restart sonrası subscriber **kendiliğinden toparlandı** (2 publish denemesi). Yani latent bir bug, şu an tetiklenmiyor |
| M7 | O | **Connector v0.8.0 stop semantiği** | connector `websocket.go:54-95` | `stopCh` ile durdurulduğunda websocket **kapatılmıyor** (`c.Close()` yok), okuyucu goroutine handler'ı çağırmaya devam ediyor. `silent` değişkeninde data race var. stop ile okuma hatası eş zamanlı olursa fetcher'ın `stopCh <- struct{}{}` gönderimi sonsuza kadar bloklayabilir. Watchdog ile "zorla reconnect" eklenirse **çift akış** oluşur. | Connector kaynağı |
| L1 | D | Metrik tasarımı | fetcher/aggregator | Birimler s yerine µs/ms. Summary aggregate edilemez. "error rate" yanıltıcı. Reconnect, WS error ve publish-0-alıcı sayaçları yok. `/healthz` statik. | |
| L2 | D | Güvenlik | `docker-compose.yml` | Redis ve Mongo auth'suz ve host'a açık (`0.0.0.0:6379/27017`). Grafana admin/admin. cAdvisor `/` ve docker dizinlerini mount ediyor. Lokal geliştirme için kabul edilebilir, sunucuda değil. | |
| L3 | D | Docker hijyeni | Dockerfile'lar | `COPY . .` → `go build` (modül cache layer'ı yok), `.dockerignore` yok (`.git` ve png'ler context'e giriyor), root user, `alpine:latest` ve diğer image'lar pinlenmemiş, `depends_on` health condition'sız (Mongo hazır değilse 30 s sonra Fatal ve restart). | |
| L4 | D | Diğer | çeşitli | `mongo.Connect` sonrası `Ping` yok. Fetcher `Publish` WS okuma goroutine'inde senkron çalışıyor (Redis kapalıyken her trade ~5 s+ bloklar, Binance yavaş tüketici bağlantısını kesebilir). `IsDefault` her trade'de default struct üretiyor (önemsiz). Interpolasyon log mesajı yeni fiyatı "last added" diye yazıyor. | |

**Doğrulanan doğru davranışlar:** Bucket OHLC hesabı doğru (probe: bucket0 min=90, max=110, first=100, last=90). SMA ring buffer doğru (1..250 için SMA50=225.5, SMA200=150.5). Basit interpolasyon doğru (100→140, 60 s: 110/120/130). Redis üzerinden uçtan uca bucket üretimi ve graceful shutdown (bağlıyken) çalışıyor. Binance'in normal disconnect'inde reconnect çalışıyor (`doneCh` kapanıyor).

---

## 5. Öncelikli Eksikler ve Sonraki Adımlar

Efor: **S** ≤ yarım gün, **M** 1–3 gün, **L** 1 hafta ve üzeri.

> Not: Asıl hedef Polymarket ise tradebot'ta yalnızca **P1** maddelerini (ve README'yi) yapıp repoyu "temiz bir referans/portföy" haline getirmek mantıklı. P2 ve sonrası Polymarket projesinde yeniden tasarlanmalı.

| Öncelik | Madde | Dosya(lar) | Somut adım | Efor |
|---|---|---|---|---|
| **P1** | Gerçek unit testler | `cmd/aggregator/*_test.go`, `cmd/shared/*_test.go` | Scratch'teki `probe_test.go` ile başlanabilir. `calculatePriceStats`, `SmaBuffer`, `AggregatedTradeInfo` için tablo-temelli testler. Crossover mantığını `main()`'den saf bir fonksiyona (`type CrossoverDetector struct{ prev *float64 }`) çıkarıp test etmek. `go test -race` | M |
| **P1** | Restart'ta sahte sinyali düzelt | `aggregator/main.go:105-146` | `lastDiff`'i "bilinmiyor" (`math.NaN()` ya da `*float64`) olarak başlat. İlk hazır hesaplamada yalnızca state kaydet, sinyal üretme. Ya da son `price_stats_sma` kaydından `lastDiff`'i reload et. Testle doğrula | S |
| **P1** | Boşluk politikası ve overflow | `tSmaBuffer.go:46-66`, `main.go:164-185` | Lineer interpolasyon yerine **forward-fill** (trade olmayan bar = son fiyat) kullan. Boşluk > N periyot (ör. 20) ise buffer'ı sıfırla ve warm-up'a dön (sinyal üretme). Timestamp'i `oldDate.Add(time.Duration(i)*period)` ile hesapla (overflow biter). Sentetik noktaları işaretle | S |
| **P1** | Fetcher dayanıklılığı | `fetcher/main.go` | (a) Retry/sleep'leri `select { case <-stop: return; case <-time.After(backoff): }` ile değiştir (M1). (b) Stall watchdog: N saniye (ör. 30 s) trade gelmezse yeniden bağlan (H3). Bunun için connector yerine doğrudan `gorilla/websocket` (ya da `coder/websocket`) + `SetReadDeadline` + `conn.Close()` kullan (M7). (c) Jitter'lı exponential backoff. (d) `ws_reconnects_total`, `ws_errors_total` metrikleri | M |
| **P1** | README güncelle | `README.md` | Simulator'ı, health port davranışını, testlerin durumunu, "order" yerine "signal" ifadesini ve bilinen sınırlamaları yaz | S |
| P2 | Tam trade verisi ve boşluk tespiti | `shared/tTradeDataPrice.go`, `fetcher/main.go:83`, `PeriodicPriceStats.go` | Payload'a `TradeID`, `Quantity`, `IsBuyerMaker` ekle. Bucket'a volume, VWAP, trade count, buy/sell volume ekle. TradeID sıçramasıyla boşluk tespit et ve REST `GET /api/v3/aggTrades` (ya da `historicalTrades`) ile backfill yap | M |
| P2 | Bucketing'i timer-driven yap | `PeriodicPriceStats.go`, `tAggregatedTradeInfo.go` | `time.Ticker` ile bucket sınırında emit (trade gelmese de), `bucket_start` alanı, shutdown'da flush, bir "grace period" ile geç trade toleransı | M |
| P2 | Mongo şema ve idempotency | `shared/functions.go`, `docker-compose.yml` | `ListCollectionNames` ile varlık kontrolü ya da `NamespaceExists` (code 48) hatasını yut. `metaField: {symbol, source}`, `granularity`, opsiyonel `expireAfterSeconds`. `mongo:8.0` gibi pinle. Startup'ta `Ping`. mongo-driver v2'ye geçişi planla | S–M |
| P2 | Durable transport | fetcher/aggregator | Redis pub/sub → **Redis Streams** (`XADD` + `XREADGROUP` + `XACK`, `MAXLEN ~`) ya da NATS JetStream. Aggregator kesintisinde veri kaybolmasın | M |
| P2 | Simulator'ı gerçek backtester yap | `cmd/simulator/main.go` | Kapanmama bug'ını düzelt (Done'u bekleme). Fee (Binance spot 0,1%) ve slippage ekle. Son mark-to-market. PnL, max drawdown, trade sayısı, win rate. `price_stats`'tan **sinyalleri yeniden hesaplayıp** SMA parametre taraması (flag'lerle `--short/--long/--from/--to/--fee`) | M |
| P3 | Konfigürasyon | `shared/constants.go`, `main.go`'lar | Sembol(ler), periyot, SMA uzunlukları, adresler için env/flag. Çoklu sembol için kanal adı `binance:trade:<sym>` | M |
| P3 | Gözlemlenebilirlik | metrics, `configs/` | `/healthz` → son trade yaşı > X ise 503. Prometheus alert rules (veri yok > 60 s, reconnect fırtınası). Metrik birimleri saniye, Summary yerine Histogram | S–M |
| P3 | Lifecycle sadeleştirme | `shared/tShutdownOrchestrator.go` | `signal.NotifyContext` + `context` propagation + `errgroup`. Data race ve leak'ler ortadan kalkar | M |
| P3 | Ops/CI hijyeni | Dockerfile'lar, compose, `.github/` | `go mod download` layer'ı, `.dockerignore`, non-root user, image pinleme, `depends_on: condition: service_healthy`, Redis/Mongo auth + host'a açmama, Grafana şifresi env'den. GitHub Actions: build, vet, test -race, staticcheck. LICENSE | S |
| P4 | Gerçek emir (önce testnet) | yeni `cmd/executor` | Binance Spot Testnet (`https://testnet.binance.vision`), `NewClient(key, secret, testnetURL)`, idempotent `newClientOrderId`, pozisyon ve risk limitleri, kill switch. **15 s SMA crossover'ın fee sonrası kârlı olma ihtimali düşük**. Polymarket'e öncelik verilecekse bunu ertele | L |

---

## 6. Polymarket Projesi İçin Yeniden Kullanım

### 6.1 Polymarket tarafında gerekenler (özet; network engelli olduğu için güncel dokümandan doğrulayın)

- **Market keşfi (REST):** Gamma API `https://gamma-api.polymarket.com` (`/events`, `/markets`: soru, outcomes, `clobTokenIds`, `endDate`, `active/closed`, volume, liquidity).
- **Fiyat ve orderbook (REST):** CLOB `https://clob.polymarket.com` (`/book`, `/books`, `/price`, `/midpoint`, `/spread`, `/prices-history?market=<token_id>&interval=…&fidelity=…` ile tarihsel seri). Data API `https://data-api.polymarket.com` (public `/trades`, positions, activity).
- **Canlı (WebSocket):** `wss://ws-subscriptions-clob.polymarket.com/ws/market`, `{"type":"market","assets_ids":[…]}` ile subscribe. Event'ler: `book` (snapshot), `price_change` (delta), `tick_size_change`, `last_trade_price`. Client'ın periyodik `PING` göndermesi gerekiyor. Kullanıcıya özel kanal (`/ws/user`) auth istiyor.
- **Trading:** Polygon üzerinde EIP-712 imzalı CLOB emirleri. L1 (cüzdan private key) ile L2 API credential'ları türetiliyor (HMAC header). USDC collateral. Resmi tam client'lar Python (`py-clob-client`) ve TypeScript. Go tarafında imzalama için `go-order-utils` var (doğrulayın). **Coğrafi kısıtlar ve ToS kontrol edilmeli.**
- **Hisse verisi:** tradebot'ta yok. Ayrı bir kaynak gerekiyor (ör. Alpaca, Polygon.io/Massive, Tiingo, Databento). Seans saatleri (ET, DST) ile 7/24 prediction market'lerin hizalanması gerekiyor.

### 6.2 Bileşen bazında uygunluk

| tradebot bileşeni | Yeniden kullanım | Ne değişmeli |
|---|---|---|
| `docker-compose` monitoring stack (Prometheus + Grafana provisioning + cAdvisor) | **Yüksek**, neredeyse olduğu gibi | Image pinleme, alert rules, yeni dashboard'lar |
| Prometheus metrik pattern'leri (received/published/age/delay) | **Yüksek** (pattern) | Label'lar (`asset_id`, `market`), Histogram, saniye birimi |
| Dockerfile'lar, health endpoint fikri | **Orta–Yüksek** | Anlamlı health (veri yaşı), non-root, cache layer |
| Fetcher reconnect döngüsü | **Orta** (iskelet) | `binance-connector-go` atılmalı. Generic WS client (read deadline, PING, backoff+jitter, `ctx` ile shutdown, dinamik subscribe/unsubscribe) |
| Kanal-pipeline yapısı (subscribe → unmarshal → aggregate) | **Orta** | Asset başına state (map), timer-driven flush |
| `AggregatedTradeInfo` (OHLC bucket) | **Orta** | Asset key, volume, bid/ask/mid/spread, bucket_start, forward-fill |
| `SmaBuffer` | Düşük–Orta | Genel amaçlı rolling window olarak kullanılabilir, ama interpolasyon politikası değişmeli |
| Redis pub/sub | **Düşük** | Araştırma verisi kaybolmamalı: Streams/JetStream ya da doğrudan append-only storage |
| Mongo time-series (metaField'sız) | **Düşük–Orta** | Korelasyon çalışması için SQL/pandas dostu bir depo daha uygun (TimescaleDB, ClickHouse, ya da Parquet). Mongo kalacaksa `metaField` zorunlu |
| SMA crossover stratejisi ve simulator | **Düşük** | Prediction market fiyatı [0,1] aralığında bir olasılık, resolution'da sıçrıyor. Strateji ve backtest mantığı farklı. Sadece "sinyal → persist → replay" akış fikri taşınabilir |
| `ShutdownOrchestrator` | Düşük | `signal.NotifyContext` + `errgroup` ile değiştirin |

### 6.3 Karar ve önerilen yol

**Karar:** İskelet *kavramsal olarak* uyuyor (canlı WS → bus → aggregator → TSDB → Prometheus/Grafana). Ama *kod olarak* tek sembollü, Binance SDK'sına bağlı ve likit, sürekli trade akışı varsayan bir prototip. Polymarket'te ise yüzlerce–binlerce illikit asset, orderbook delta'ları ve açılıp resolve olan bir market yaşam döngüsü var. **Doğrudan genişletmek yerine yeni bir modül (ya da repo) açıp altyapıyı ve pattern'leri taşımak** daha az maliyetli olur.

Önerilen fazlar:
1. **Faz 0 (S–M) — Backfill önce:** Gamma'dan market listesi → `/prices-history` ile tarihsel seriler → Parquet/DB. Aynı zaman ekseninde (UTC) hisse/ETF verisi. Korelasyon araştırması için önce tarihsel veri gerekiyor, canlı akış sonra gelir.
2. **Faz 1 (M) — Araştırma:** Fiyatların logit dönüşümü, getiri/olasılık değişimi, lead-lag cross-correlation, event-window analizleri, seans dışı saatlerin ele alınması.
3. **Faz 2 (M) — Canlı capture servisi:** `pm-fetcher` (market WS, PING, reconnect, dinamik subscribe) → **ham event'leri** append-only olarak sakla ("raw capture, derive later") → aggregator (timer-driven, forward-fill). tradebot'un compose monitoring stack'i ve metrik pattern'leri buraya taşınır.
4. **Faz 3 (M–L) — Paper trading:** Sinyal servisi + spread/fee modelleyen simulator.
5. **Faz 4 (L) — Canlı trading:** CLOB emirleri (EIP-712), risk limitleri, kill switch, küçük sermaye.

Not: `/home/user/quant-forge` (Python, Parquet cache, vectorized/event-driven backtester) bu incelemenin kapsamında değil, ama Faz 1/3'ün araştırma ve backtest kısmı için Go'dan daha uygun bir yer olabilir. Ayrıca değerlendirin.

---

## 7. `/home/user/crypto_process` ile ilişkisi (kısa)

Ayrı bir Go modülü (`github.com/kaanureyen/crypto_process`, Ağustos–Eylül 2025, son commit 2025-09-17). README'de "Crypto data processor for tradebot" yazıyor ama **tradebot'tan kod import etmiyor**. Binance'in tarihsel **trade CSV** dump'larını (`btcusdt_2401_2506.csv`: tradeId, price, qty, quoteQty, time, isBuyerMaker, isBestMatch; 2025 sonrası µs timestamp'ler de işleniyor) offline okuyup **hacim bazlı** bucket'lara topluyor, feature çıkarıyor ve DP/Q-table tarzı strateji denemeleri yapıyor (commit: "first failed trade strategy"). Kısacası tradebot canlı yakalama prototipi (Mayıs 2025), crypto_process ise onun ardından gelen offline araştırma prototipi. Aynı kanal-pipeline stilini kullanıyor ve tradebot'un attığı alanları (qty, isBuyerMaker) kullanıyor. Ortak paket ya da veri formatı yok. Polymarket araştırmasında "offline backfill + feature" yaklaşımı bu repodan, "canlı capture + monitoring" yaklaşımı tradebot'tan alınabilir. (crypto_process proprietary lisans başlığı taşıyor. tradebot'ta LICENSE yok.)
