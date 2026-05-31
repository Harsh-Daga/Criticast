// Adversarial attribution workload — CHARTER Part H.2 topology.
//
//	go run .                    # :8080
//	OTEL_TRACE_FILE=/tmp/otel.json go run .
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/criticast/criticast/internal/groundtruth"
)

const (
	workerCount = 4
	poolSize    = 4
)

type ctxKey int

const requestTokenKey ctxKey = 1

func main() {
	shutdown, err := initTracing()
	if err != nil {
		log.Fatalf("tracing: %v", err)
	}
	defer func() {
		if err := shutdown(context.Background()); err != nil {
			log.Printf("tracing shutdown: %v", err)
		}
	}()

	port := envOr("PORT", "8080")
	srv := newService()
	http.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	http.HandleFunc("/work", srv.handleWork)
	log.Printf("p0b adversarial server listening on :%s (GOMAXPROCS=%d)", port, runtime.GOMAXPROCS(0))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

type service struct {
	workCh chan workItem
	pool   chan *conn
	cache  sync.Mutex
	cacheM map[string]string
}

var workSeq atomic.Uint64

type workItem struct {
	token string
	resp  chan string
	id    uint64
}

type conn struct {
	id int
}

func newService() *service {
	s := &service{
		workCh: make(chan workItem, 64),
		pool:   make(chan *conn, poolSize),
		cacheM: make(map[string]string),
	}
	for i := 0; i < poolSize; i++ {
		s.pool <- &conn{id: i}
	}
	for w := 0; w < workerCount; w++ {
		go s.worker(w)
	}
	return s
}

func (s *service) handleWork(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("id")
	if token == "" {
		token = fmt.Sprintf("req-%d", time.Now().UnixNano())
	}
	ctx := context.WithValue(r.Context(), requestTokenKey, token)
	ctx, root := startSpan(ctx, "handler", token)
	defer endSpan(root, nil)
	logGT(ctx, groundtruth.SiteHandlerEntry, "handler")

	var wg sync.WaitGroup
	results := make([]string, 3)

	for i := 0; i < 2; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			child := context.WithValue(ctx, requestTokenKey, token)
			child, sp := startSpan(child, "spawn", token)
			defer endSpan(sp, nil)
			logGT(child, groundtruth.SiteSpawn, "spawn")
			results[idx] = s.spawnWork(child, token)
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, sp := startSpan(ctx, "worker-pool", token)
		defer endSpan(sp, nil)
		resp := make(chan string, 1)
		elemID := workSeq.Add(1)
		logG(token, groundtruth.SiteWorkerPoolSend, "worker-pool", strconv.FormatUint(elemID, 10))
		s.workCh <- workItem{token: token, resp: resp, id: elemID}
		results[2] = <-resp
	}()

	wg.Wait()

	c := <-s.pool
	logGT(ctx, groundtruth.SiteConnPoolAcquire, "handler")
	s.cache.Lock()
	logGT(ctx, groundtruth.SiteMutexLock, "handler")
	cacheVal := fmt.Sprintf("v-%s", token)
	s.cacheM[token] = cacheVal
	s.cache.Unlock()
	logGT(ctx, groundtruth.SiteMutexUnlock, "handler")
	s.pool <- c
	logGT(ctx, groundtruth.SiteConnPoolRelease, "handler")

	out := map[string]any{
		"token":   token,
		"results": results,
		"cache":   cacheVal,
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
	logGT(ctx, groundtruth.SiteHandlerExit, "handler")
}

func (s *service) spawnWork(ctx context.Context, token string) string {
	logGT(ctx, groundtruth.SiteSpawnWork, "spawn")
	time.Sleep(time.Millisecond)
	return "spawn:" + token
}

func (s *service) worker(id int) {
	for item := range s.workCh {
		elem := strconv.FormatUint(item.id, 10)
		logG(item.token, groundtruth.SiteWorkerRecv, "worker", elem)
		time.Sleep(2 * time.Millisecond)
		item.resp <- fmt.Sprintf("worker%d:%s", id, item.token)
		logG(item.token, groundtruth.SiteWorkerDone, "worker", elem)
	}
}

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
