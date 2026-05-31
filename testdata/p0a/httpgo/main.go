// P0-A primary workload: Go HTTP with channel fan-out to three simulated backends.
//
//	go run .   # listens :8080, GET /
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"runtime"
	"time"
)

const backendCount = 3

func main() {
	port := envOr("PORT", "8080")
	mux := http.NewServeMux()
	mux.HandleFunc("/", handle)
	log.Printf("p0a-httpgo listening :%s GOMAXPROCS=%d", port, runtime.GOMAXPROCS(0))
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func handle(w http.ResponseWriter, r *http.Request) {
	type result struct {
		ID  int    `json:"id"`
		Out string `json:"out"`
	}
	results := make([]result, backendCount)

	for i := 0; i < backendCount; i++ {
		id := i
		respCh := make(chan string, 1)
		go func() {
			respCh <- simulateBackend(id)
		}()
		results[id] = result{ID: id, Out: <-respCh}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"backends": results})
}

func simulateBackend(id int) string {
	time.Sleep(time.Duration(1+id%3) * time.Millisecond)
	return "ok"
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
