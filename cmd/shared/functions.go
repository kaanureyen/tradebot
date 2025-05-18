package shared

import (
	"log"
	"net/http"
	"os"
	"strconv"
)

func IsRunningInDocker() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}
	return false
}

// initializes logger, starts health endpoint, inits&returns pointer to a shutdownOrchestrator
func InitCommon(moduleName string) *ShutdownOrchestrator {
	logger("[" + moduleName + "] ") // set logger and print start msg
	healthEndpoint(moduleName)      // start health endpoint and print msg

	// start shutdown orchestrator
	var shutdownOrchestrator ShutdownOrchestrator
	shutdownOrchestrator.Start()
	return &shutdownOrchestrator
}

func logger(s string) {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix(s)
	log.Println("[Info] Started")
}

func healthEndpoint(moduleName string) {
	go func() {
		http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte(moduleName + " is OK"))
		})

		port := os.Getenv("HEALTH_PORT")
		if port != "" { // found HEALTH_PORT. will only try it.
			log.Printf("[Info] Health endpoint (http://localhost:%v/healthz) listening on :%v\n", port, port)
			if err := http.ListenAndServe(":"+port, nil); err != nil {
				log.Fatalf("[Fatal][Error] Error starting health endpoint: %v", err)
			}

		} else { // will try ports in range [HealthEndpointFirstPort, HealthEndpointLastPort] one by one
			for port := HealthEndpointFirstPort; port < HealthEndpointLastPort; port++ {
				log.Printf("[Info] Health endpoint (http://localhost:%v/healthz) listening on :%v\n", port, port)
				if err := http.ListenAndServe(":"+strconv.Itoa(port), nil); err != nil {
					log.Printf("[Warning] Error starting health endpoint: %v", err)
					log.Printf("[Info] Trying next port.")
					continue
				}
				return
			}
			log.Fatalf("[Fatal][Error] Could not find an empty port for health endpoint for %v in range [%v,%v]\n", moduleName, HealthEndpointFirstPort, HealthEndpointLastPort)
		}
	}()
}
