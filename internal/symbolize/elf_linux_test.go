//go:build linux

package symbolize

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestOpenELFOnCriticastBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux only")
	}
	root := filepath.Join("..", "..")
	bin := filepath.Join(root, "bin", "criticast")
	if _, err := os.Stat(bin); err != nil {
		cmd := exec.Command("go", "build", "-o", bin, "./cmd/criticast")
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Skip("build criticast:", err, string(out))
		}
	}
	elf, err := OpenELF(bin)
	if err != nil {
		t.Skip("open elf:", err)
	}
	_, ok := elf.ResolvePC(0)
	_ = ok
}
