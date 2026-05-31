// Package loader attaches CO-RE BPF programs (L1).
package loader

//go:generate go run github.com/cilium/ebpf/cmd/bpf2go -target bpf -cc clang -cflags "-I../../bpf" collector ../../bpf/collector.c -- -I../../bpf
