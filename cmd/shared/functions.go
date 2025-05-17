package shared

import "log"

func Logger(s string) {
	// show line number in logs, show microseconds, add prefix
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.Lmicroseconds)
	log.SetPrefix(s)
	log.Println("Started")
}
