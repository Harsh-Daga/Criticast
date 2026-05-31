package main_test

import (
	"testing"
	"time"

	main "github.com/criticast/criticast/cmd/criticast"
)

func TestParseDurationNS(t *testing.T) {
	tests := []struct {
		in   string
		want uint64
	}{
		{"1us", 1000},
		{"50us", 50000},
		{"1µs", 1000},
		{"2ms", 2_000_000},
	}
	for _, tc := range tests {
		got, err := main.ParseDurationNS(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %d want %d", tc.in, got, tc.want)
		}
	}
}

func TestParseCLIDuration(t *testing.T) {
	tests := []struct {
		in   string
		want time.Duration
	}{
		{"5", 5 * time.Second},
		{"5s", 5 * time.Second},
		{"30s", 30 * time.Second},
		{"2m", 2 * time.Minute},
	}
	for _, tc := range tests {
		got, err := main.ParseCLIDuration(tc.in)
		if err != nil {
			t.Fatalf("%q: %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("%q: got %v want %v", tc.in, got, tc.want)
		}
	}
}
