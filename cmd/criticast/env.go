package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func runEnv() error {
	root, err := repoRoot()
	if err != nil {
		return err
	}

	fmt.Printf("criticast env (host %s/%s)\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  go: %s\n", runtime.Version())
	if btf := "/sys/kernel/btf/vmlinux"; fileOK(btf) {
		fmt.Printf("  btf: %s (ok)\n", btf)
	} else {
		fmt.Println("  btf: missing (/sys/kernel/btf/vmlinux)")
	}
	for _, bin := range []string{"clang", "bpftool", "llvm-objdump"} {
		printTool(bin)
	}
	fmt.Println("  capabilities: CAP_BPF + CAP_PERFMON (not CAP_SYS_ADMIN / privileged)")
	fmt.Println("  suggested: sudo criticast record …  or setcap on bin/criticast")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, filepath.Join(root, "scripts", "check-linux-env.sh"))
	cmd.Dir = root
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("check-linux-env: %w", err)
	}
	return nil
}

func fileOK(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func printTool(name string) {
	if p, err := exec.LookPath(name); err == nil {
		fmt.Printf("  %s: %s\n", name, p)
		return
	}
	fmt.Printf("  %s: not found\n", name)
}
