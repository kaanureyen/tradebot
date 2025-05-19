package main

import (
	"log"
	"net/http"
	"time"

	"github.com/kaanureyen/tradebot/cmd/shared"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// prometheus metrics
var aggregateInfoAge = prometheus.NewSummary(
	prometheus.SummaryOpts{
		Name:       "aggregate_info_age_milliseconds",
		Help:       "Difference of local time on aggregate creation and trade time in milliseconds",
		Objectives: map[float64]float64{0.5: 0.05, 0.95: 0.01, 0.99: 0.001},
	},
)

func main() {
	shutdownOrchestrator := shared.InitCommon("aggregator") // set logger name, start http health endpoint, initialize & start shutdownOrchestrator
	defer func() {
		<-shutdownOrchestrator.Done // blocks until every shutdownOrchestrator.Get()'s recv is sent an empty struct, after a interrupt/terminate signal.
		log.Println("[Info] Exiting...")
	}()

	// register the prometheus metrics
	prometheus.MustRegister(aggregateInfoAge)
	// start prometheus metrics
	go func() {
		http.Handle("/metrics", promhttp.Handler())
		log.Fatal("[Fatal][Error] Prometheus metrics endpoint could not be opened. Error: ", http.ListenAndServe(":2113", nil))
	}()

	period := time.Second
	aggCh := PeriodicPriceStats(shared.RedisChannel, period, shutdownOrchestrator)
	sB := SmaBuffer{}
	sB.Init(shared.SmaLongTerm)
	for v := range aggCh {
		log.Println("lastprice: ", v.lastPrice)
		sB.AddWithLinInterpFill(v.lastPrice, v.lastTime, period)

		if sB.IsSmaReady(shared.SmaLongTerm) {
			smaShortTerm, _ := sB.CalculateSma(shared.SmaShortTerm)
			log.Printf("[Info] SMA%v:%v\n", shared.SmaShortTerm, smaShortTerm)

			smaLongTerm, _ := sB.CalculateSma(shared.SmaLongTerm)
			log.Printf("[Info] SMA%v:%v\n", shared.SmaLongTerm, smaLongTerm)
		}
	}
}
