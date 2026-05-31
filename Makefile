# criticast build — BPF (Linux + BTF) + Go userspace
.PHONY: all bpf vmlinux go test verify test-bpf clean spike check-env workloads

GO ?= go
CLANG ?= clang
LLVM_STRIP ?= llvm-strip
# bpf_tracing.h expects __TARGET_ARCH_x86 | arm64 (not amd64).
ARCH ?= $(shell uname -m 2>/dev/null | sed 's/x86_64/x86/;s/amd64/x86/;s/aarch64/arm64/')
BPF_CLANG_FLAGS := -g -O2 -target bpf -D__BPF__ -D__TARGET_ARCH_$(ARCH) -I bpf
BPF_OBJ := bpf/collector.bpf.o
VMLINUX := bpf/vmlinux.h

all: go

vmlinux: $(VMLINUX)

$(VMLINUX):
	@test -f /sys/kernel/btf/vmlinux || (echo "BTF required: /sys/kernel/btf/vmlinux"; exit 1)
	bpftool btf dump file /sys/kernel/btf/vmlinux format c > $@

# Single translation unit (collector.c includes go_probe.c) — no ld.lld BPF link step.
bpf: $(BPF_OBJ)

$(BPF_OBJ): bpf/collector.c bpf/go_probe.c bpf/event.h bpf/go_cfg.h $(VMLINUX)
	@test -f /sys/kernel/btf/vmlinux || (echo "BTF required"; exit 1)
	$(CLANG) $(BPF_CLANG_FLAGS) -c bpf/collector.c -o $@
	$(LLVM_STRIP) -g $@

go:
	mkdir -p bin
	$(GO) build -o bin/criticast ./cmd/criticast

workloads:
	mkdir -p bin
	$(GO) build -o bin/httpgo ./testdata/p0a/httpgo
	$(GO) build -o bin/p0b-server ./testdata/p0b/server

test:
	$(GO) test ./...
	$(GO) test ./testdata/p0a/httpgo
	$(GO) test ./testdata/p0b/server

# Linux + BTF: compile BPF and check object exposes expected programs.
test-bpf: bpf
	@./scripts/test-bpf-object.sh

verify:
	@./scripts/verify.sh

check-env:
	@./scripts/check-linux-env.sh

spike:
	@./scripts/spike.sh

clean:
	rm -f $(BPF_OBJ) $(VMLINUX) bin/criticast bin/httpgo bin/p0b-server
	$(GO) clean -testcache
