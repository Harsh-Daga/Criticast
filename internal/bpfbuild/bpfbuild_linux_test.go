//go:build linux

package bpfbuild_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

func TestBPFObjectBuildsAndHasSymbols(t *testing.T) {
	if _, err := os.Stat("/sys/kernel/btf/vmlinux"); err != nil {
		t.Skip("BTF not available")
	}
	root := repoRoot(t)
	if _, err := exec.LookPath("clang"); err != nil {
		t.Skip("clang not installed")
	}

	cmd := exec.Command("make", "test-bpf")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make test-bpf: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "test-bpf-object: OK") {
		t.Fatalf("unexpected output:\n%s", out)
	}
}
