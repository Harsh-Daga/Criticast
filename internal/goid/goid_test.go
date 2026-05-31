package goid

import (
	"runtime"
	"testing"
)

func TestParseStackHeader(t *testing.T) {
	id, err := ParseStackHeader([]byte("goroutine 42 [running]:\nmain.main()"))
	if err != nil || id != 42 {
		t.Fatalf("id=%d err=%v", id, err)
	}
}

func TestCurrentMatchesStack(t *testing.T) {
	var buf [128]byte
	n := runtime.Stack(buf[:], false)
	want, err := ParseStackHeader(buf[:n])
	if err != nil {
		t.Fatal(err)
	}
	got, err := Current()
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("Current()=%d stack=%d", got, want)
	}
}
