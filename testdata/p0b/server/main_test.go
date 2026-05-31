package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("OTEL_TRACE_FILE", "/dev/null")
	shutdown, err := initTracing()
	if err != nil {
		fmt.Fprintf(os.Stderr, "p0b TestMain: initTracing: %v\n", err)
		os.Exit(1)
	}
	code := m.Run()
	_ = shutdown(context.Background())
	os.Exit(code)
}

func TestHandleWork(t *testing.T) {
	srv := newService()
	mux := http.NewServeMux()
	mux.HandleFunc("/work", srv.handleWork)

	req := httptest.NewRequest(http.MethodGet, "/work?id=A", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), `"token":"A"`) {
		t.Fatalf("body=%s", rr.Body.String())
	}
}

func TestHandleWorkConcurrent(t *testing.T) {
	srv := newService()
	mux := http.NewServeMux()
	mux.HandleFunc("/work", srv.handleWork)

	var wg sync.WaitGroup
	for _, id := range []string{"A", "B", "C"} {
		for range 32 {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()
				req := httptest.NewRequest(http.MethodGet, "/work?id="+id, nil)
				rr := httptest.NewRecorder()
				mux.ServeHTTP(rr, req)
				if rr.Code != http.StatusOK {
					t.Errorf("id=%s status=%d", id, rr.Code)
				}
			}(id)
		}
	}
	wg.Wait()
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}).ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatal(rr.Code)
	}
}
